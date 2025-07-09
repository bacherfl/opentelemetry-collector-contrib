// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// supported distributions
	distributionKubernetes = "kubernetes"
	distributionOpenShift  = "openshift"

	// Default config values.
	defaultCollectionInterval         = 10 * time.Second
	defaultDistribution               = distributionKubernetes
	defaultMetadataCollectionInterval = 5 * time.Minute

	enableNewAllocatableMetricsFeatureFlag = "receiver.k8scluster.enableNewAllocatableMetrics"
)

// TODO this flag specifically for the new allocatable metrics will likely need to be replaced with the general flag (see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40708#issuecomment-3027128817) related to implementing the stable semantic conventions
var EnableNewAllocatableMetrics = featuregate.GlobalRegistry().MustRegister(
	enableNewAllocatableMetricsFeatureFlag,
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled the k8s.node.allocatable.cpu, k8s.node.allocatable.ephemeral_storage, k8s.node.allocatable.pods and k8s.node.allocatable.memory metrics will be represented by updown counters instead of gauges"),
	// TODO this version may need to be updated, depending on when this PR will get merged
	featuregate.WithRegisterFromVersion("v0.128.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40708"),
)

var defaultNodeConditionsToReport = []string{"Ready"}

func createDefaultConfig() component.Config {
	return &Config{
		Distribution:               defaultDistribution,
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
		MetadataCollectionInterval: defaultMetadataCollectionInterval,
		MetricsBuilderConfig:       metadata.DefaultMetricsBuilderConfig(),
	}
}

// NewFactory creates a factory for k8s_cluster receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(newMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(newLogsReceiver, metadata.MetricsStability),
	)
}

// This is the map of already created k8scluster receivers for particular configurations.
// We maintain this map because the Factory is asked log and metric receivers separately
// when it gets CreateLogs() and CreateMetrics() but they must not
// create separate objects, they must use one receiver object per configuration.
var receivers = sharedcomponent.NewSharedComponents()
