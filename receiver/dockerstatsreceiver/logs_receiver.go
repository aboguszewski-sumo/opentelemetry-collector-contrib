// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

type logsReceiver struct {
	config     *Config
	settings   receiver.CreateSettings
	client     *docker.Client
	consumer   consumer.Logs
	eventsChan chan struct{}
}

func newLogsReceiver(set receiver.CreateSettings, config *Config, consumer consumer.Logs) *logsReceiver {
	return &logsReceiver{
		config:   config,
		settings: set,
		consumer: consumer,
	}
}

func (r *logsReceiver) Start(ctx context.Context, _ component.Host) error {
	dConfig, err := docker.NewConfig(r.config.Endpoint, r.config.Timeout, r.config.ExcludedImages, r.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	r.client, err = docker.NewDockerClient(dConfig, r.settings.Logger)
	if err != nil {
		return err
	}

	if err = r.client.LoadContainerList(ctx); err != nil {
		return err
	}

	if r.config.EventsConfig.Enabled {
		r.eventsChan = make(chan struct{})
		go r.client.EventToLogsLoop(ctx, r.consumer, r.eventsChan)
	}

	return nil
}

// Shutdown implements receiver.Logs.
func (r *logsReceiver) Shutdown(ctx context.Context) error {
	if r.eventsChan != nil {
		r.eventsChan <- struct{}{}
	}
	return nil
}
