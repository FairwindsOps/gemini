package snapshots

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

func TestBasicSchedule(t *testing.T) {
	schedule := v1.SnapshotSchedule{
		Every: "minute",
		Keep:  4,
	}
	start := time.Now().Add(time.Minute * -5)

	existing := []PhotonSnapshot{
		PhotonSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 4),
		},
		PhotonSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 3),
		},
		PhotonSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute * 2),
		},
		PhotonSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start.Add(time.Minute),
		},
		PhotonSnapshot{
			Intervals: []string{"minute"},
			Timestamp: start,
		},
	}
	toCreate, toDelete, err := getSnapshotChanges([]v1.SnapshotSchedule{schedule}, existing)
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
