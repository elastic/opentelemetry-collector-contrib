// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// fakeAuthExtension stands in for the awsauth extension; the receiver matches it
// structurally via the awsCredentialsProvider interface.
type fakeAuthExtension struct {
	component.StartFunc
	component.ShutdownFunc
	creds aws.CredentialsProvider
}

func (f *fakeAuthExtension) GetCredentialsProvider() aws.CredentialsProvider { return f.creds }

// fakeHost serves extensions to components during Start.
type fakeHost struct {
	extensions map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component { return h.extensions }

func TestResolveAuthExtension(t *testing.T) {
	authID := component.MustNewID("awsauth")
	staticCreds := credentials.NewStaticCredentialsProvider("AKID", "SECRET", "TOKEN")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		authID: &fakeAuthExtension{creds: staticCreds},
	}}

	t.Run("no auth configured", func(t *testing.T) {
		creds, err := resolveAuthExtension(host, nil)
		require.NoError(t, err)
		require.Nil(t, creds)
	})

	t.Run("resolves provider", func(t *testing.T) {
		creds, err := resolveAuthExtension(host, &authID)
		require.NoError(t, err)
		retrieved, err := creds.Retrieve(t.Context())
		require.NoError(t, err)
		require.Equal(t, "AKID", retrieved.AccessKeyID)
	})

	t.Run("unknown extension", func(t *testing.T) {
		unknown := component.MustNewID("missing")
		_, err := resolveAuthExtension(host, &unknown)
		require.ErrorContains(t, err, "unknown auth extension")
	})

	t.Run("extension without provider interface", func(t *testing.T) {
		wrongID := component.MustNewID("notauth")
		wrongHost := &fakeHost{extensions: map[component.ID]component.Component{
			wrongID: &fakeNonAuthExtension{},
		}}
		_, err := resolveAuthExtension(wrongHost, &wrongID)
		require.ErrorContains(t, err, "does not provide AWS credentials")
	})
}

type fakeNonAuthExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func TestMetricsScraperUsesAuthExtension(t *testing.T) {
	authID := component.MustNewID("awsauth")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		authID: &fakeAuthExtension{creds: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")},
	}}

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	cfg.Auth = &authID

	scraper := newCloudWatchMetricsScraper(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	})
	require.NoError(t, scraper.start(t.Context(), host))
	require.NotNil(t, scraper.client)
}

func TestLogsReceiverUsesAuthExtension(t *testing.T) {
	authID := component.MustNewID("awsauth")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		authID: &fakeAuthExtension{creds: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")},
	}}

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	cfg.Auth = &authID
	cfg.Logs.Groups.AutodiscoverConfig = nil

	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}, consumertest.NewNop())

	require.NoError(t, rcvr.Start(t.Context(), host))
	require.NoError(t, rcvr.ensureSession())
	require.NotNil(t, rcvr.client)

	creds, err := rcvr.credsProvider.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "AKID", creds.AccessKeyID)

	require.NoError(t, rcvr.Shutdown(t.Context()))
}
