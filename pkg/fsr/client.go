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

// Package fsr wraps the AWS EC2 Fast Snapshot Restore APIs that Gemini
// uses to enable and observe FSR on EBS snapshots.
package fsr

import "context"

// AZState describes the FSR state of one snapshot in one Availability Zone,
// as reported by AWS DescribeFastSnapshotRestores.
type AZState struct {
	AvailabilityZone string
	// State is the AWS-reported state: "enabling", "optimizing", "enabled",
	// "disabling", "disabled", or empty if AWS has no record for this AZ.
	State string
}

// Client is the subset of the EC2 FSR API that Gemini depends on.
// Implementations: aws.go (real, aws-sdk-go-v2) and fake.go (in-memory, for tests).
type Client interface {
	// Enable requests FSR for snapshotID in the listed availability zones.
	// Returns an error on AWS API failure. Calling Enable on an already-enabled
	// snapshot/AZ pair is a no-op as far as Gemini is concerned.
	Enable(ctx context.Context, snapshotID string, availabilityZones []string) error

	// Disable releases FSR for snapshotID in the listed availability zones.
	// Idempotent: if AWS reports a target AZ is already not in the "enabled"
	// state, that AZ is treated as success rather than an error.
	Disable(ctx context.Context, snapshotID string, availabilityZones []string) error

	// Describe returns the current AZ-level state of the snapshot. The returned
	// slice contains exactly one entry per AZ that AWS knows about for this
	// snapshot; AZs with no record are omitted.
	Describe(ctx context.Context, snapshotID string) ([]AZState, error)
}

// IsWarmInAll reports whether the snapshot is in AWS state "enabled" in every
// AZ in the requested list. Used by the reconciler to decide when to flip the
// fsr-state annotation from "enabling" to "enabled".
func IsWarmInAll(states []AZState, requestedAZs []string) bool {
	if len(requestedAZs) == 0 {
		return false
	}
	byAZ := make(map[string]string, len(states))
	for _, s := range states {
		byAZ[s.AvailabilityZone] = s.State
	}
	for _, az := range requestedAZs {
		if byAZ[az] != "enabled" {
			return false
		}
	}
	return true
}

// IsColdInAll reports whether the snapshot has left the "enabled"/"enabling"
// states in every AZ in the requested list. Used by the reconciler to decide
// when to flip the fsr-state annotation from "disabling" to "disabled".
// An AZ that AWS no longer reports is treated as cold.
func IsColdInAll(states []AZState, requestedAZs []string) bool {
	if len(requestedAZs) == 0 {
		return true
	}
	byAZ := make(map[string]string, len(states))
	for _, s := range states {
		byAZ[s.AvailabilityZone] = s.State
	}
	for _, az := range requestedAZs {
		switch byAZ[az] {
		case "enabled", "enabling", "optimizing":
			return false
		}
	}
	return true
}
