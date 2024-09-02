// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/commander"
	"os"
	"os/signal"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func main() {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	logger, _ := zap.NewDevelopment()

	cfg, err := config.Load(*configFlag)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	c, err := commander.NewCommander(
		logger,
		cfg.Storage.Directory,
		cfg.Agent,
		"--config", filepath.Join(cfg.Storage.Directory, supervisor.AgentConfigFileName),
	)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	opampClient := client.NewWebSocket(supervisor.NewLoggerFromZap(logger))
	opampServer := server.New(supervisor.NewLoggerFromZap(logger))

	supervisor, err := supervisor.NewSupervisor(logger, cfg, c, opampClient, opampServer)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	err = supervisor.Start()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	supervisor.Shutdown()
}
