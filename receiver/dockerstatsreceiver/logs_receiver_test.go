// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

var (
	logsMockFolder = filepath.Join("logstestdata", "mock")
	daemonSocket   = "unix:///Users/aboguszewski/.colima/docker.sock"
)

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.EventsConfig.Enabled = true
	cfg.Endpoint = daemonSocket

	sink := &consumertest.LogsSink{}
	logsReceiver := newLogsReceiver(receivertest.NewNopCreateSettings(), cfg, sink)

	err := logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = logsReceiver.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestScrapeEventsV2(t *testing.T) {
	testCases := []struct {
		desc             string
		expectedLogsFile string
		mockDockerEngine func(t *testing.T) *httptest.Server
		cfgBuilder       *testConfigBuilder
	}{
		{
			desc:             "destroy_container",
			expectedLogsFile: filepath.Join(logsMockFolder, "destroy_container", "expected_logs.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/events": filepath.Join(logsMockFolder, "destroy_container", "events.json"),
					// Set the path for stats: as for now, we can't disable fetching stats.
					"/v1.25/containers/json": filepath.Join(mockFolder, "single_container", "containers.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().withEndpoint(daemonSocket).withEventsEnabled(true),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			mockDockerEngine := tc.mockDockerEngine(t)
			defer mockDockerEngine.Close()

			sink := &consumertest.LogsSink{}
			receiver := newLogsReceiver(
				receivertest.NewNopCreateSettings(), tc.cfgBuilder.withEndpoint(mockDockerEngine.URL).build(), sink)
			err := receiver.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return sink.LogRecordCount() > 0
			}, 2*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]
			// Uncomment to regenerate 'expected_logs.yaml' files
			golden.WriteLogs(t, tc.expectedLogsFile, logs)

			expectedLogs, err := golden.ReadLogs(tc.expectedLogsFile)

			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}
