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

package snapshots

import (
	"testing"
	"time"

	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1"
	"github.com/stretchr/testify/assert"
)

func TestBasicSchedule(t *testing.T) {
	schedule := snapshotgroup.SnapshotSchedule{
		Every: "minute",
		Keep:  4,
	}
	start := time.Now().Add(time.Minute * -5)

	existing := []*GeminiSnapshot{
		&GeminiSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 4),
		},
		&GeminiSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 3),
		},
		&GeminiSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 2),
		},
		&GeminiSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute),
		},
		&GeminiSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start,
		},
	}
	toCreate, toDelete, err := getSnapshotChanges([]snapshotgroup.SnapshotSchedule{schedule}, existing)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(toDelete))
	assert.Equal(t, existing[4], toDelete[0])
	assert.Equal(t, toCreate, []string{"minute"})
}

func TestParseInterval(t *testing.T) {
	testCases := []struct {
		input  string
		output time.Duration
		err    bool
	}{
		{
			input:  "1 hour",
			output: time.Hour,
		},
		{
			input: "asdfadsf",
			err:   true,
		},
		{
			input:  "minute",
			output: time.Minute,
		},
		{
			input:  "3 years",
			output: time.Hour * 24 * 365 * 3,
		},
	}
	for _, testCase := range testCases {
		interval, err := ParseInterval(testCase.input)
		if testCase.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, testCase.output, interval)
		}
	}
}
