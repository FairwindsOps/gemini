package snapshots

import (
	"strconv"
	"strings"
	"time"

	"k8s.io/klog"

	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

var durations = map[string]time.Duration{
	"minute": time.Minute,
	"hour":   time.Hour,
	"day":    time.Hour * 24,
	// TODO: get more accurate on month/year
	"month": time.Hour * 24 * 30,
	"year":  time.Hour * 24 * 365,
}

func getSnapshotChanges(schedules []v1.SnapshotSchedule, snapshots []photonSnapshot) ([]string, []photonSnapshot) {
	numToKeepByInterval := map[string]int{}
	numSnapshotsByInterval := map[string]int{}
	for _, schedule := range schedules {
		// Note - we have to keep an "extra" snapshot to cover the whole range
		// e.g. With "every 1 year, keep 2", on 1/1/2020, we would have snapshots for
		// - 1/1/2020
		// - 1/1/2019
		// - 1/1/2018
		// So we're convered with 2 full years of backups.
		numToKeepByInterval[schedule.Every] = schedule.Keep + 1
	}
	now := time.Now().UTC()

	toDelete := []photonSnapshot{}
	needsCreation := map[string]bool{}
	for _, schedule := range schedules {
		needsCreation[schedule.Every] = true
	}
	for _, snapshot := range snapshots {
		klog.Infof("Checking snapshot %s", snapshot.snapshot.ObjectMeta.Name)
		keep := false
		for _, interval := range snapshot.intervals {
			if numSnapshotsByInterval[interval] == 0 {
				// This is the latest snapshot
				nextSnapshotTime := snapshot.timestamp.Add(ParseInterval(interval))
				if nextSnapshotTime.Before(now) {
					klog.Infof("  stale for interval %s", interval)
					numSnapshotsByInterval[interval]++
				} else {
					needsCreation[interval] = false
				}
			}
			numSnapshotsByInterval[interval]++
			if numSnapshotsByInterval[interval] <= numToKeepByInterval[interval] {
				keep = true
			}
		}
		if !keep {
			toDelete = append(toDelete, snapshot)
		}
	}

	toCreate := []string{}
	for k, v := range needsCreation {
		klog.Infof("Need creation for interval %s: %t", k, v)
		if v {
			toCreate = append(toCreate, k)
		}
	}
	return toCreate, toDelete
}

// ParseInterval parses an interval string as defined by Photon
func ParseInterval(str string) time.Duration {
	amt := 1
	every := str
	parts := strings.Split(str, " ")
	if len(parts) == 2 {
		every = parts[1]
		var err error
		amt, err = strconv.Atoi(parts[0])
		if err != nil {
			klog.Errorf("Could not parse interval %s", str)
			amt = 1
		}
	}
	every = strings.TrimSuffix(every, "s")
	duration, ok := durations[every]
	if !ok {
		klog.Errorf("Could not parse interval %s", str)
		duration = time.Hour
	}
	return time.Duration(amt) * duration
}
