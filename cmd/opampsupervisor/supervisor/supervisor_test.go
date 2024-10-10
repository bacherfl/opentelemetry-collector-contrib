// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"errors"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/commander/mock"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/healthchecker"
	mock3 "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/healthchecker/mock"
	mock2 "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/mock"
	"go.opentelemetry.io/collector/config/configopaque"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/mock/gomock"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func Test_composeEffectiveConfig(t *testing.T) {
	acceptsRemoteConfig := true
	s := Supervisor{
		logger:                       zap.NewNop(),
		persistentState:              &persistentState{},
		config:                       config.Supervisor{Capabilities: config.Capabilities{AcceptsRemoteConfig: acceptsRemoteConfig}},
		pidProvider:                  staticPIDProvider(1234),
		hasNewConfig:                 make(chan struct{}, 1),
		agentConfigOwnMetricsSection: &atomic.Value{},
		mergedConfig:                 &atomic.Value{},
		agentHealthCheckEndpoint:     "localhost:8000",
	}

	agentDesc := &atomic.Value{}
	agentDesc.Store(&protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "otelcol",
					},
				},
			},
		},
	})

	s.agentDescription = agentDesc

	fileLogConfig := `
receivers:
  filelog:
    include: ['/test/logs/input.log']
    start_at: "beginning"

exporters:
  file:
    path: '/test/logs/output.log'

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [file]`

	require.NoError(t, s.createTemplates())
	require.NoError(t, s.loadAndWriteInitialMergedConfig())

	configChanged, err := s.composeMergedConfig(&protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {
					Body: []byte(fileLogConfig),
				},
			},
		},
	})
	require.NoError(t, err)

	expectedConfig, err := os.ReadFile("../testdata/collector/effective_config.yaml")
	require.NoError(t, err)
	expectedConfig = bytes.ReplaceAll(expectedConfig, []byte("\r\n"), []byte("\n"))

	require.True(t, configChanged)
	require.Equal(t, string(expectedConfig), s.mergedConfig.Load().(string))
}

func Test_onMessage(t *testing.T) {
	t.Run("AgentIdentification - New instance ID is valid", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{})
		initialID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		newID := uuid.MustParse("018fef3f-14a8-73ef-b63e-3b96b146ea38")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: initialID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			opampClient:                  client.NewHTTP(NewLoggerFromZap(zap.NewNop())),
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: newID[:],
			},
		})

		require.Equal(t, newID, s.persistentState.InstanceID)
	})

	t.Run("AgentIdentification - New instance ID is invalid", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{})

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: []byte("invalid-value"),
			},
		})

		require.Equal(t, testUUID, s.persistentState.InstanceID)
	})

	t.Run("CustomMessage - Custom message from server is forwarded to agent", func(t *testing.T) {
		customMessage := &protobufs.CustomMessage{
			Capability: "teapot",
			Type:       "brew",
			Data:       []byte("chamomile"),
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		gotMessage := false
		var agentConn serverTypes.Connection = &mockConn{
			sendFunc: func(_ context.Context, message *protobufs.ServerToAgent) error {
				require.Equal(t, &protobufs.ServerToAgent{
					InstanceUid:   testUUID[:],
					CustomMessage: customMessage,
				}, message)
				gotMessage = true

				return nil
			},
		}

		agentConnAtomic := &atomic.Value{}
		agentConnAtomic.Store(agentConn)

		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.onMessage(context.Background(), &types.MessageData{
			CustomMessage: customMessage,
		})

		require.True(t, gotMessage, "Message was not sent to agent")
	})

	t.Run("CustomCapabilities - Custom capabilities from server are forwarded to agent", func(t *testing.T) {
		customCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"coffeemaker", "teapot"},
		}
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		gotMessage := false
		var agentConn serverTypes.Connection = &mockConn{
			sendFunc: func(_ context.Context, message *protobufs.ServerToAgent) error {
				require.Equal(t, &protobufs.ServerToAgent{
					InstanceUid:        testUUID[:],
					CustomCapabilities: customCapabilities,
				}, message)
				gotMessage = true

				return nil
			},
		}

		agentConnAtomic := &atomic.Value{}
		agentConnAtomic.Store(agentConn)

		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.onMessage(context.Background(), &types.MessageData{
			CustomCapabilities: customCapabilities,
		})

		require.True(t, gotMessage, "Message was not sent to agent")
	})

}

func Test_handleAgentOpAMPMessage(t *testing.T) {
	t.Run("CustomMessage - Custom message from agent is forwarded to server", func(t *testing.T) {
		customMessage := &protobufs.CustomMessage{
			Capability: "teapot",
			Type:       "brew",
			Data:       []byte("chamomile"),
		}

		gotMessageChan := make(chan struct{})
		client := &mockOpAMPClient{
			sendCustomMessageFunc: func(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
				require.Equal(t, customMessage, message)

				close(gotMessageChan)
				msgChan := make(chan struct{}, 1)
				msgChan <- struct{}{}
				return msgChan, nil
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		loopDoneChan := make(chan struct{})
		go func() {
			defer close(loopDoneChan)
			s.forwardCustomMessagesToServerLoop()
		}()

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			CustomMessage: customMessage,
		})

		select {
		case <-gotMessageChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for custom message to send")
		}

		close(s.doneChan)

		select {
		case <-loopDoneChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for forward loop to stop")
		}
	})

	t.Run("CustomCapabilities - Custom capabilities from agent are forwarded to server", func(t *testing.T) {
		customCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"coffeemaker", "teapot"},
		}

		client := &mockOpAMPClient{
			setCustomCapabilitiesFunc: func(caps *protobufs.CustomCapabilities) error {
				require.Equal(t, customCapabilities, caps)
				return nil
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			CustomCapabilities: customCapabilities,
		})
	})
}

type staticPIDProvider int

func (s staticPIDProvider) PID() int {
	return int(s)
}

type mockOpAMPClient struct {
	agentDesc                 *protobufs.AgentDescription
	sendCustomMessageFunc     func(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
	setCustomCapabilitiesFunc func(customCapabilities *protobufs.CustomCapabilities) error
}

func (mockOpAMPClient) Start(_ context.Context, _ types.StartSettings) error {
	return nil
}

func (mockOpAMPClient) Stop(_ context.Context) error {
	return nil
}

func (m *mockOpAMPClient) SetAgentDescription(descr *protobufs.AgentDescription) error {
	m.agentDesc = descr
	return nil
}

func (m mockOpAMPClient) AgentDescription() *protobufs.AgentDescription {
	return m.agentDesc
}

func (mockOpAMPClient) SetHealth(_ *protobufs.ComponentHealth) error {
	return nil
}

func (mockOpAMPClient) UpdateEffectiveConfig(_ context.Context) error {
	return nil
}

func (mockOpAMPClient) SetRemoteConfigStatus(_ *protobufs.RemoteConfigStatus) error {
	return nil
}

func (mockOpAMPClient) SetPackageStatuses(_ *protobufs.PackageStatuses) error {
	return nil
}

func (mockOpAMPClient) RequestConnectionSettings(_ *protobufs.ConnectionSettingsRequest) error {
	return nil
}

func (m mockOpAMPClient) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	if m.setCustomCapabilitiesFunc != nil {
		return m.setCustomCapabilitiesFunc(customCapabilities)
	}
	return nil
}

func (m mockOpAMPClient) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	if m.sendCustomMessageFunc != nil {
		return m.sendCustomMessageFunc(message)
	}

	msgChan := make(chan struct{}, 1)
	msgChan <- struct{}{}
	return msgChan, nil
}

type mockConn struct {
	sendFunc func(ctx context.Context, message *protobufs.ServerToAgent) error
}

func (mockConn) Connection() net.Conn {
	return nil
}
func (m mockConn) Send(ctx context.Context, message *protobufs.ServerToAgent) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}
func (mockConn) Disconnect() error {
	return nil
}

func TestSupervisor_StartAndShutdown(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockCommander := mock.NewMockICommander(ctrl)
	mockOpampClient := mock2.NewMockOpAmpClient(ctrl)
	mockOpAmpServer := mock2.NewMockOpAmpServer(ctrl)
	mockHealthChecker := mock3.NewMockHealthChecker(ctrl)

	s, err := setupTestSupervisor(t, mockCommander, mockOpampClient, mockOpAmpServer, mockHealthChecker, func(_ context.Context, _ types.StartSettings) error {
		return nil
	}, nil)

	err = s.Start()
	require.NoError(t, err)

	// eventually the internal representation of the agent state should be updated
	require.Eventually(t, func() bool {
		return s.agentHasStarted
	}, 10*time.Second, 1*time.Second)

	s.Shutdown()
}

func TestSupervisor_StartAndReceiveMessage(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockCommander := mock.NewMockICommander(ctrl)
	mockOpampClient := mock2.NewMockOpAmpClient(ctrl)
	mockOpAmpServer := mock2.NewMockOpAmpServer(ctrl)
	mockHealthChecker := mock3.NewMockHealthChecker(ctrl)

	const testConfigMessage = `receivers:
  debug:`

	s, err := setupTestSupervisor(t, mockCommander, mockOpampClient, mockOpAmpServer, mockHealthChecker, func(_ context.Context, css types.StartSettings) error {
		css.Callbacks.OnConnect(context.Background())
		css.Callbacks.OnMessage(context.Background(), &types.MessageData{
			RemoteConfig: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {
							Body: []byte(testConfigMessage),
						},
					},
				},
				ConfigHash: []byte("hash"),
			},
		})
		return nil
	}, nil)

	// expect the client status to be updated with the latest applied config hash
	mockOpampClient.
		EXPECT().
		SetRemoteConfigStatus(
			gomock.AssignableToTypeOf(&protobufs.RemoteConfigStatus{}),
		).
		DoAndReturn(func(status *protobufs.RemoteConfigStatus) error {
			require.Equal(t, []byte("hash"), status.LastRemoteConfigHash)
			require.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, status.Status)
			return nil
		})

	// expect the config to be updated
	mockOpampClient.
		EXPECT().
		UpdateEffectiveConfig(
			gomock.Any(),
		).
		Return(nil)

	// expect an additional restart of the commander
	mockCommander.EXPECT().Start(gomock.Any()).Return(nil)
	mockCommander.EXPECT().Stop(gomock.Any()).Return(nil)

	err = s.Start()
	require.NoError(t, err)

	// eventually the internal representation of the agent state should be updated
	require.Eventually(t, func() bool {
		return s.agentHasStarted
	}, 10*time.Second, 1*time.Second)

	s.Shutdown()

	// verify the merged config being stored

	fileContent, err := os.ReadFile(filepath.Join(s.config.Storage.Directory, lastRecvRemoteConfigFile))
	require.NoError(t, err)
	require.Contains(t, string(fileContent), testConfigMessage)
}

func TestSupervisor_StartAndReceiveOpAmpConnectionSettings(t *testing.T) {

	const certPEM = `
-----BEGIN CERTIFICATE-----
MIIDujCCAqKgAwIBAgIIE31FZVaPXTUwDQYJKoZIhvcNAQEFBQAwSTELMAkGA1UE
BhMCVVMxEzARBgNVBAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMTHEdvb2dsZSBJbnRl
cm5ldCBBdXRob3JpdHkgRzIwHhcNMTQwMTI5MTMyNzQzWhcNMTQwNTI5MDAwMDAw
WjBpMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwN
TW91bnRhaW4gVmlldzETMBEGA1UECgwKR29vZ2xlIEluYzEYMBYGA1UEAwwPbWFp
bC5nb29nbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEfRrObuSW5T7q
5CnSEqefEmtH4CCv6+5EckuriNr1CjfVvqzwfAhopXkLrq45EQm8vkmf7W96XJhC
7ZM0dYi1/qOCAU8wggFLMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAa
BgNVHREEEzARgg9tYWlsLmdvb2dsZS5jb20wCwYDVR0PBAQDAgeAMGgGCCsGAQUF
BwEBBFwwWjArBggrBgEFBQcwAoYfaHR0cDovL3BraS5nb29nbGUuY29tL0dJQUcy
LmNydDArBggrBgEFBQcwAYYfaHR0cDovL2NsaWVudHMxLmdvb2dsZS5jb20vb2Nz
cDAdBgNVHQ4EFgQUiJxtimAuTfwb+aUtBn5UYKreKvMwDAYDVR0TAQH/BAIwADAf
BgNVHSMEGDAWgBRK3QYWG7z2aLV29YG2u2IaulqBLzAXBgNVHSAEEDAOMAwGCisG
AQQB1nkCBQEwMAYDVR0fBCkwJzAloCOgIYYfaHR0cDovL3BraS5nb29nbGUuY29t
L0dJQUcyLmNybDANBgkqhkiG9w0BAQUFAAOCAQEAH6RYHxHdcGpMpFE3oxDoFnP+
gtuBCHan2yE2GRbJ2Cw8Lw0MmuKqHlf9RSeYfd3BXeKkj1qO6TVKwCh+0HdZk283
TZZyzmEOyclm3UGFYe82P/iDFt+CeQ3NpmBg+GoaVCuWAARJN/KfglbLyyYygcQq
0SgeDh8dRKUiaW3HQSoYvTvdTuqzwK4CXsr3b5/dAOY8uMuG/IAR3FgwTbZ1dtoW
RvOTa8hYiU6A475WuZKyEHcwnGYe57u2I2KbMgcKjPniocj4QzgYsVAVKW3IwaOh
yE+vPxsiUkvQHdO2fojCkY8jg70jxM+gu59tPDNbw3Uh/2Ij310FgTHsnGQMyA==
-----END CERTIFICATE-----`

	ctrl := gomock.NewController(t)

	mockCommander := mock.NewMockICommander(ctrl)
	mockOpampClient := mock2.NewMockOpAmpClient(ctrl)
	mockOpAmpServer := mock2.NewMockOpAmpServer(ctrl)
	mockHealthChecker := mock3.NewMockHealthChecker(ctrl)

	waitForShutdown := make(chan struct{})

	sendOpAmpConnectionSettings := make(chan struct{})
	s, err := setupTestSupervisor(t, mockCommander, mockOpampClient, mockOpAmpServer, mockHealthChecker, func(_ context.Context, css types.StartSettings) error {
		css.Callbacks.OnConnect(context.Background())
		go func() {
			<-sendOpAmpConnectionSettings
			_ = css.Callbacks.OnOpampConnectionSettings(
				context.Background(),
				&protobufs.OpAMPConnectionSettings{
					DestinationEndpoint: "https://localhost:1234",
					Headers: &protobufs.Headers{
						Headers: []*protobufs.Header{
							{
								Key:   "foo",
								Value: "bar",
							},
						},
					},
					Certificate: &protobufs.TLSCertificate{
						CaPublicKey: []byte(certPEM),
					},
				},
			)
		}()

		return nil
	}, func(mockOpampClient *mock2.MockOpAmpClient) {
		mockOpampClient.EXPECT().SetAgentDescription(gomock.Any()).Return(nil)
		mockOpampClient.EXPECT().Stop(gomock.Any()).Return(nil)
		mockOpampClient.
			EXPECT().
			Start(
				gomock.Any(),
				gomock.AssignableToTypeOf(types.StartSettings{}),
			).
			DoAndReturn(func(ctx context.Context, css types.StartSettings) error {
				require.Equal(t, http.Header{"Foo": []string{"bar"}}, css.Header)
				require.Equal(t, "https://localhost:1234", css.OpAMPServerURL)
				go func() {
					waitForShutdown <- struct{}{}
				}()
				return nil
			})

		mockOpampClient.EXPECT().SetHealth(gomock.Any()).DoAndReturn(func(health *protobufs.ComponentHealth) error {
			require.False(t, health.Healthy)
			return nil
		})
	})

	err = s.Start()
	require.NoError(t, err)

	// eventually the internal representation of the agent state should be updated
	require.Eventually(t, func() bool {
		return s.agentHasStarted
	}, 10*time.Second, 1*time.Second)

	sendOpAmpConnectionSettings <- struct{}{}

	// eventually the updated opampConnectionSettings should be applied
	require.Eventually(t, func() bool {
		// wait for one property to be updated - the remaining ones can be checked outside the Eventually function
		return s.config.Server.Endpoint == "https://localhost:1234"
	}, 10*time.Second, 1*time.Second)

	require.Equal(t, http.Header{
		"Foo": []string{"bar"},
	}, s.config.Server.Headers)
	require.Equal(t, configopaque.String("[REDACTED]").String(), s.config.Server.TLSSetting.CAPem.String())

	<-waitForShutdown
	s.Shutdown()
}

func TestSupervisor_StartAndReceiveRestartCommand(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockCommander := mock.NewMockICommander(ctrl)
	mockOpampClient := mock2.NewMockOpAmpClient(ctrl)
	mockOpAmpServer := mock2.NewMockOpAmpServer(ctrl)
	mockHealthChecker := mock3.NewMockHealthChecker(ctrl)

	s, err := setupTestSupervisor(t, mockCommander, mockOpampClient, mockOpAmpServer, mockHealthChecker, func(_ context.Context, css types.StartSettings) error {
		css.Callbacks.OnConnect(context.Background())
		css.Callbacks.OnCommand(context.Background(), &protobufs.ServerToAgentCommand{
			Type: protobufs.CommandType_CommandType_Restart,
		})
		return nil
	}, nil)

	// expect an additional restart of the commander
	mockCommander.EXPECT().Restart(gomock.Any()).Return(nil)

	err = s.Start()
	require.NoError(t, err)

	// eventually the internal representation of the agent state should be updated
	require.Eventually(t, func() bool {
		return s.agentHasStarted
	}, 10*time.Second, 1*time.Second)

	s.Shutdown()
}

func setupTestSupervisor(t *testing.T, mockCommander *mock.MockICommander, mockOpampClient *mock2.MockOpAmpClient, mockOpAmpServer *mock2.MockOpAmpServer, mockHealthChecker *mock3.MockHealthChecker, clientBehavior func(_ context.Context, css types.StartSettings) error, clientExpectationsAfterStart func(ampClient *mock2.MockOpAmpClient)) (*Supervisor, error) {
	tmpDir := t.TempDir()

	cfg := config.Supervisor{
		Server:       config.OpAMPServer{},
		Agent:        config.Agent{},
		Capabilities: config.Capabilities{},
		Storage: config.Storage{
			Directory: tmpDir,
		},
	}
	s, err := NewSupervisor(
		zap.NewNop(),
		cfg,
		mockCommander,
		mockOpampClient,
		mockOpAmpServer,
		WithHealthCheckerFunc(func() healthchecker.HealthChecker {
			return mockHealthChecker
		}),
	)

	require.NoError(t, err)

	// Set up the expectations for components included by the supervisor

	// =======================================
	// Opamp Client
	// =======================================

	// Expect the client to start
	// this assertion can be a lot more detailed with the start settings being evaluated
	mockOpampClient.
		EXPECT().
		Start(
			gomock.Any(),
			gomock.AssignableToTypeOf(types.StartSettings{}),
		).
		DoAndReturn(func(ctx context.Context, css types.StartSettings) error {
			return clientBehavior(ctx, css)
		})

	// the Agent description of the client should be set
	mockOpampClient.
		EXPECT().
		SetAgentDescription(
			gomock.Any(),
		).
		DoAndReturn(func(ad *protobufs.AgentDescription) error {
			require.NotNil(t, ad)
			return nil
		})

	// The health of the client will be set multiple times:

	// Set health to false during start up
	mockOpampClient.
		EXPECT().
		SetHealth(gomock.Any()).
		DoAndReturn(func(ch *protobufs.ComponentHealth) error {
			require.False(t, ch.Healthy)
			return nil
		}).Times(2)

	// after all components have been started, the health should eventually be set to true
	mockOpampClient.
		EXPECT().
		SetHealth(gomock.Any()).
		DoAndReturn(func(ch *protobufs.ComponentHealth) error {
			require.True(t, ch.Healthy)
			return nil
		})

	if clientExpectationsAfterStart != nil {
		clientExpectationsAfterStart(mockOpampClient)
	}

	// AFTER SHUTDOWN

	// Set health to false during shutdown
	mockOpampClient.
		EXPECT().
		SetHealth(gomock.Any()).
		DoAndReturn(func(ch *protobufs.ComponentHealth) error {
			require.False(t, ch.Healthy)
			return nil
		})

	// The client should be stopped upon shutdown
	mockOpampClient.
		EXPECT().
		Stop(gomock.Any()).
		Return(nil)

	// =======================================
	// Opamp Server
	// =======================================

	// The Opamp Server should be started two times:
	// 1. When obtaining the bootstrap info
	// 2. When starting the long-lived server
	mockOpAmpServer.
		EXPECT().
		Start(gomock.Any()).
		DoAndReturn(func(settings server.StartSettings) error {
			connectionResponse := settings.Callbacks.OnConnecting(&http.Request{})
			connectionResponse.ConnectionCallbacks.OnMessage(
				context.TODO(),
				&mockConn{},
				&protobufs.AgentToServer{
					AgentDescription: &protobufs.AgentDescription{
						IdentifyingAttributes: []*protobufs.KeyValue{
							{
								Key:   semconv.AttributeServiceInstanceID,
								Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: s.persistentState.InstanceID.String()}},
							},
						},
					},
				},
			)
			return nil
		}).Times(2)

	// Upon Shutdown of the supervisor the Opamp server should be stopped as well
	mockOpAmpServer.
		EXPECT().
		Stop(gomock.Any()).
		Return(nil).
		Times(2)

	// =======================================
	// Commander
	// =======================================

	// The Commander should be started two times:
	// 1. When obtaining the bootstrap info
	// 2. When starting the long-lived agent process
	mockCommander.
		EXPECT().
		Start(
			gomock.Any(),
		).
		Times(2).
		Return(nil)

	// Upon Shutdown of the supervisor the Commander should be stopped as well
	mockCommander.
		EXPECT().
		Stop(
			gomock.Any(),
		).
		Return(nil).
		Times(2)

	ch := make(chan struct{})
	mockCommander.EXPECT().Exited().DoAndReturn(func() <-chan struct{} {
		return ch
	}).AnyTimes()

	mockCommander.EXPECT().IsRunning().Return(true).AnyTimes()

	// return an error on the first health check
	mockHealthChecker.EXPECT().Check(gomock.Any()).Return(errors.New("not ready"))
	// on the second check, simulate an up and running agent process
	// TODO: The health of the client seems to only be set to true if there was at least one
	// error during the health checks before - this likely means that if the agent starts too fast the
	// internal state will not be set properly
	mockHealthChecker.EXPECT().Check(gomock.Any()).Return(nil).AnyTimes()
	return s, err
}
