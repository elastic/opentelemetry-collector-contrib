// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// fakeCredentialsProvider stands in for the awscredentialsprovider extension; the
// receiver matches it structurally via the awsCredentialsProvider interface.
type fakeCredentialsProvider struct {
	component.StartFunc
	component.ShutdownFunc
	creds aws.CredentialsProvider
}

func (f *fakeCredentialsProvider) GetCredentialsProvider() aws.CredentialsProvider {
	return f.creds
}

// fakeHost serves extensions to components during Start.
type fakeHost struct {
	extensions map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component { return h.extensions }

// stubSTSEndpoint points the AWS SDK's STS client at a local test server that answers
// GetCallerIdentity. ensureSession resolves the account ID via STS; without a reachable
// endpoint that call fails and the receiver has no account ID to report.
func stubSTSEndpoint(t *testing.T, accountID string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::` + accountID + `:user/test</Arn>
    <UserId>AKIAI44QH8DHBEXAMPLE</UserId>
    <Account>` + accountID + `</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>test-request-id</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ENDPOINT_URL_STS", srv.URL)
}

func TestResolveCredentialsProvider(t *testing.T) {
	provID := component.MustNewID("awscredentialsprovider")
	staticCreds := credentials.NewStaticCredentialsProvider("AKID", "SECRET", "TOKEN")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		provID: &fakeCredentialsProvider{creds: staticCreds},
	}}

	t.Run("none configured", func(t *testing.T) {
		creds, err := resolveCredentialsProvider(host, nil)
		require.NoError(t, err)
		require.Nil(t, creds)
	})

	t.Run("resolves provider", func(t *testing.T) {
		creds, err := resolveCredentialsProvider(host, &provID)
		require.NoError(t, err)
		retrieved, err := creds.Retrieve(t.Context())
		require.NoError(t, err)
		require.Equal(t, "AKID", retrieved.AccessKeyID)
	})

	t.Run("unknown extension", func(t *testing.T) {
		unknown := component.MustNewID("missing")
		_, err := resolveCredentialsProvider(host, &unknown)
		require.ErrorContains(t, err, "unknown credentials_provider extension")
	})

	t.Run("extension without provider interface", func(t *testing.T) {
		wrongID := component.MustNewID("other")
		wrongHost := &fakeHost{extensions: map[component.ID]component.Component{
			wrongID: &fakeOtherExtension{},
		}}
		_, err := resolveCredentialsProvider(wrongHost, &wrongID)
		require.ErrorContains(t, err, "does not provide AWS credentials")
	})
}

type fakeOtherExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func TestMetricsScraperUsesCredentialsProvider(t *testing.T) {
	provID := component.MustNewID("awscredentialsprovider")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		provID: &fakeCredentialsProvider{creds: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")},
	}}

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	cfg.CredentialsProvider = &provID

	scraper := newCloudWatchMetricsScraper(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	})
	require.NoError(t, scraper.start(t.Context(), host))
	require.NotNil(t, scraper.client)
	require.NotNil(t, scraper.stsClient)
}

func TestLogsReceiverUsesCredentialsProvider(t *testing.T) {
	stubSTSEndpoint(t, "123456789012")

	provID := component.MustNewID("awscredentialsprovider")
	host := &fakeHost{extensions: map[component.ID]component.Component{
		provID: &fakeCredentialsProvider{creds: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")},
	}}

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	cfg.CredentialsProvider = &provID
	cfg.Logs.Groups.AutodiscoverConfig = nil

	rcvr := newLogsReceiver(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}, consumertest.NewNop())

	require.NoError(t, rcvr.Start(t.Context(), host))
	require.NoError(t, rcvr.ensureSession())
	require.NotNil(t, rcvr.client)
	require.Equal(t, "123456789012", rcvr.accountID)

	creds, err := rcvr.credsProvider.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "AKID", creds.AccessKeyID)

	require.NoError(t, rcvr.Shutdown(t.Context()))
}
