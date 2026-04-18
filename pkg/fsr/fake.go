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
	"sync"
)

// FakeClient is an in-memory Client implementation for unit tests.
// Enable records the requested AZs as state="enabling"; tests flip them to
// "enabled" via SetState to simulate AWS warmup completing.
type FakeClient struct {
	mu sync.Mutex
	// snapshotID -> AZ -> state
	states map[string]map[string]string
	// EnableErr, if non-nil, is returned from the next Enable call.
	EnableErr error
	// DescribeErr, if non-nil, is returned from the next Describe call.
	DescribeErr error
	// EnableCalls records every (snapshotID, azs) pair passed to Enable.
	EnableCalls []EnableCall
}

// EnableCall captures one invocation of Enable for assertion in tests.
type EnableCall struct {
	SnapshotID string
	AZs        []string
}

// NewFakeClient returns a fresh FakeClient with no recorded state.
func NewFakeClient() *FakeClient {
	return &FakeClient{states: map[string]map[string]string{}}
}

func (f *FakeClient) Enable(_ context.Context, snapshotID string, azs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.EnableErr != nil {
		err := f.EnableErr
		f.EnableErr = nil
		return err
	}
	f.EnableCalls = append(f.EnableCalls, EnableCall{SnapshotID: snapshotID, AZs: append([]string(nil), azs...)})
	if _, ok := f.states[snapshotID]; !ok {
		f.states[snapshotID] = map[string]string{}
	}
	for _, az := range azs {
		// Don't downgrade an already-enabled state.
		if cur, ok := f.states[snapshotID][az]; !ok || cur == "" {
			f.states[snapshotID][az] = "enabling"
		}
	}
	return nil
}

func (f *FakeClient) Describe(_ context.Context, snapshotID string) ([]AZState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DescribeErr != nil {
		err := f.DescribeErr
		f.DescribeErr = nil
		return nil, err
	}
	out := []AZState{}
	for az, state := range f.states[snapshotID] {
		out = append(out, AZState{AvailabilityZone: az, State: state})
	}
	return out, nil
}

// SetState lets a test simulate AWS having warmed (or otherwise transitioned)
// FSR for a given snapshot/AZ.
func (f *FakeClient) SetState(snapshotID, az, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.states[snapshotID]; !ok {
		f.states[snapshotID] = map[string]string{}
	}
	f.states[snapshotID][az] = state
}
