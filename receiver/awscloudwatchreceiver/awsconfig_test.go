// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
)

func TestLoadAWSConfigStaticCredentials(t *testing.T) {
	cfg := &Config{
		Region: "us-west-2",
		Credentials: configoptional.Some(CredentialsConfig{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
			SessionToken:    "TOKEN",
		}),
	}

	awsCfg, err := loadAWSConfig(t.Context(), cfg)
	require.NoError(t, err)
	require.Equal(t, "us-west-2", awsCfg.Region)

	creds, err := awsCfg.Credentials.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "AKID", creds.AccessKeyID)
	require.Equal(t, "SECRET", creds.SecretAccessKey)
	require.Equal(t, "TOKEN", creds.SessionToken)
}

func TestLoadAWSConfigAssumeRole(t *testing.T) {
	cfg := &Config{
		Region: "us-west-2",
		Credentials: configoptional.Some(CredentialsConfig{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
			RoleARN:         "arn:aws:iam::123456789012:role/monitoring",
			ExternalID:      "my-external-id",
		}),
	}

	awsCfg, err := loadAWSConfig(t.Context(), cfg)
	require.NoError(t, err)
	// Role assumption replaces the static provider with an STS-backed credentials cache.
	// Retrieving from it would call STS, so only the wiring is asserted here.
	require.IsType(t, &aws.CredentialsCache{}, awsCfg.Credentials)
}

func TestLoadAWSConfigNoCredentials(t *testing.T) {
	cfg := &Config{Region: "eu-west-1"}

	awsCfg, err := loadAWSConfig(t.Context(), cfg)
	require.NoError(t, err)
	require.Equal(t, "eu-west-1", awsCfg.Region)
}
