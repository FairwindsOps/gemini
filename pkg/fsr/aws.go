// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsr

import (
	"context"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ec2API is the subset of the AWS EC2 client we need. Lets tests inject a stub.
type ec2API interface {
	EnableFastSnapshotRestores(ctx context.Context, params *ec2.EnableFastSnapshotRestoresInput, optFns ...func(*ec2.Options)) (*ec2.EnableFastSnapshotRestoresOutput, error)
	DisableFastSnapshotRestores(ctx context.Context, params *ec2.DisableFastSnapshotRestoresInput, optFns ...func(*ec2.Options)) (*ec2.DisableFastSnapshotRestoresOutput, error)
	DescribeFastSnapshotRestores(ctx context.Context, params *ec2.DescribeFastSnapshotRestoresInput, optFns ...func(*ec2.Options)) (*ec2.DescribeFastSnapshotRestoresOutput, error)
}

// awsClient implements Client against the real EC2 API.
type awsClient struct {
	ec2 ec2API
}

// NewAWSClient builds a Client backed by aws-sdk-go-v2. Region and credentials
// come from the standard AWS environment (AWS_REGION, AWS_PROFILE, IRSA, etc).
func NewAWSClient(ctx context.Context) (Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &awsClient{ec2: ec2.NewFromConfig(cfg)}, nil
}

func (c *awsClient) Enable(ctx context.Context, snapshotID string, azs []string) error {
	out, err := c.ec2.EnableFastSnapshotRestores(ctx, &ec2.EnableFastSnapshotRestoresInput{
		SourceSnapshotIds: []string{snapshotID},
		AvailabilityZones: azs,
	})
	if err != nil {
		return fmt.Errorf("EnableFastSnapshotRestores(%s): %w", snapshotID, err)
	}
	// AWS reports per-AZ failures inside Unsuccessful[].FastSnapshotRestoreStateErrors
	// rather than as a top-level error. Bubble the first one up so the reconciler retries.
	if len(out.Unsuccessful) > 0 {
		for _, u := range out.Unsuccessful {
			for _, se := range u.FastSnapshotRestoreStateErrors {
				var az, code, msg string
				if se.AvailabilityZone != nil {
					az = *se.AvailabilityZone
				}
				if se.Error != nil {
					if se.Error.Code != nil {
						code = *se.Error.Code
					}
					if se.Error.Message != nil {
						msg = *se.Error.Message
					}
				}
				return fmt.Errorf("EnableFastSnapshotRestores(%s) unsuccessful in az=%s code=%s: %s",
					snapshotID, az, code, msg)
			}
		}
		return fmt.Errorf("EnableFastSnapshotRestores(%s) reported unsuccessful with no detail", snapshotID)
	}
	return nil
}

func (c *awsClient) Disable(ctx context.Context, snapshotID string, azs []string) error {
	out, err := c.ec2.DisableFastSnapshotRestores(ctx, &ec2.DisableFastSnapshotRestoresInput{
		SourceSnapshotIds: []string{snapshotID},
		AvailabilityZones: azs,
	})
	if err != nil {
		return fmt.Errorf("DisableFastSnapshotRestores(%s): %w", snapshotID, err)
	}
	// Same per-AZ error shape as Enable, but we swallow "not in the enabled state"
	// errors: they mean the target AZ is already disabled (either never enabled,
	// someone disabled it manually, or a previous call already succeeded).
	for _, u := range out.Unsuccessful {
		for _, se := range u.FastSnapshotRestoreStateErrors {
			if se.Error == nil {
				continue
			}
			code, msg := "", ""
			if se.Error.Code != nil {
				code = *se.Error.Code
			}
			if se.Error.Message != nil {
				msg = *se.Error.Message
			}
			if isAlreadyDisabled(code, msg) {
				continue
			}
			var az string
			if se.AvailabilityZone != nil {
				az = *se.AvailabilityZone
			}
			return fmt.Errorf("DisableFastSnapshotRestores(%s) unsuccessful in az=%s code=%s: %s",
				snapshotID, az, code, msg)
		}
	}
	return nil
}

// isAlreadyDisabled reports whether an AWS per-AZ FSR state error indicates
// the snapshot was already not in the "enabled" state. AWS does not document
// a stable typed error for this, so we match the message substring the API
// has historically used ("not in the enabled state"). A conservative match —
// if AWS changes the wording we'll briefly surface these as errors until we
// update the check, which is preferable to silently swallowing real failures.
func isAlreadyDisabled(_, message string) bool {
	m := strings.ToLower(message)
	return strings.Contains(m, "not in the enabled state") || strings.Contains(m, "not enabled")
}

func (c *awsClient) Describe(ctx context.Context, snapshotID string) ([]AZState, error) {
	out, err := c.ec2.DescribeFastSnapshotRestores(ctx, &ec2.DescribeFastSnapshotRestoresInput{
		Filters: []ec2types.Filter{
			{Name: strPtr("snapshot-id"), Values: []string{snapshotID}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeFastSnapshotRestores(%s): %w", snapshotID, err)
	}
	states := make([]AZState, 0, len(out.FastSnapshotRestores))
	for _, r := range out.FastSnapshotRestores {
		s := AZState{State: string(r.State)}
		if r.AvailabilityZone != nil {
			s.AvailabilityZone = *r.AvailabilityZone
		}
		states = append(states, s)
	}
	return states, nil
}

func strPtr(s string) *string { return &s }
