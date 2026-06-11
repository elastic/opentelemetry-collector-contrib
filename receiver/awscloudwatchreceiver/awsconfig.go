// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// loadAWSConfig builds the aws.Config shared by the logs and metrics clients.
//
// Credential resolution:
//   - credentials.access_key_id/secret_access_key set: static credentials.
//   - otherwise: the default SDK chain (env vars, shared config/credentials files,
//     EC2/ECS roles, IRSA, ...), optionally narrowed by profile.
//   - credentials.role_arn set: that role is assumed via STS using whichever base
//     credentials were resolved above.
func loadAWSConfig(ctx context.Context, cfg *Config) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.IMDSEndpoint != "" {
		opts = append(opts, config.WithEC2IMDSEndpoint(cfg.IMDSEndpoint))
	}
	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}

	creds := cfg.Credentials.Get()
	if creds != nil && creds.AccessKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			string(creds.SecretAccessKey),
			string(creds.SessionToken),
		)))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, err
	}

	if creds != nil && creds.RoleARN != "" {
		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsCfg), creds.RoleARN,
			func(o *stscreds.AssumeRoleOptions) {
				if creds.ExternalID != "" {
					o.ExternalID = aws.String(creds.ExternalID)
				}
			})
		awsCfg.Credentials = aws.NewCredentialsCache(provider)
	}

	return awsCfg, nil
}
