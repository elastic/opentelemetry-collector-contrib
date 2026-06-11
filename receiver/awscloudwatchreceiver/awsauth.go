// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/component"
)

// awsCredentialsProvider is implemented by the awsauth extension. It is declared
// locally (structural typing) to avoid a module dependency on the extension.
type awsCredentialsProvider interface {
	GetCredentialsProvider() aws.CredentialsProvider
}

// resolveAuthExtension returns the credentials provider from the auth extension
// referenced by id, or nil when no auth extension is configured.
func resolveAuthExtension(host component.Host, id *component.ID) (aws.CredentialsProvider, error) {
	if id == nil {
		return nil, nil
	}
	ext, ok := host.GetExtensions()[*id]
	if !ok {
		return nil, fmt.Errorf("unknown auth extension %q", id)
	}
	provider, ok := ext.(awsCredentialsProvider)
	if !ok {
		return nil, fmt.Errorf("extension %q does not provide AWS credentials", id)
	}
	return provider.GetCredentialsProvider(), nil
}
