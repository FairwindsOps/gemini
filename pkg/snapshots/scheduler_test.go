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

	existing := []photonSnapshot{
		photonSnapshot{
			intervals: []string{"minute"},
			timestamp: start.Add(time.Minute * 4),
		},
		photonSnapshot{
			intervals: []string{"minute"},
			timestamp: start.Add(time.Minute * 3),
		},
		photonSnapshot{
			intervals: []string{"minute"},
			timestamp: start.Add(time.Minute * 2),
		},
		photonSnapshot{
			intervals: []string{"minute"},
			timestamp: start.Add(time.Minute),
		},
		photonSnapshot{
			intervals: []string{"minute"},
			timestamp: start,
		},
	}
	toCreate, toDelete := getSnapshotChanges([]v1.SnapshotSchedule{schedule}, existing)
	assert.Equal(t, 1, len(toDelete))
	assert.Equal(t, existing[4], toDelete[0])
	assert.Equal(t, toCreate, []string{"minute"})
}

func TestParseInterval(t *testing.T) {
	testCases := []struct {
		input  string
		output time.Duration
	}{
		{
			input:  "1 hour",
			output: time.Hour,
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
		assert.Equal(t, testCase.output, ParseInterval(testCase.input))
	}
}
