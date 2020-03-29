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
