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
	"fmt"
	"strconv"
	"strings"
	"time"

	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
	"k8s.io/klog"
)

var durations = map[string]time.Duration{
	"second": time.Second,
	"minute": time.Minute,
	"hour":   time.Hour,
	"day":    time.Hour * 24,
	"week":   time.Hour * 24 * 7,
	// TODO: get more accurate on month/year
	"month": time.Hour * 24 * 30,
	"year":  time.Hour * 24 * 365,
}

func getSnapshotChanges(schedules []snapshotgroup.SnapshotSchedule, snapshots []GeminiSnapshot) ([]string, []GeminiSnapshot, error) {
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

	toDelete := []GeminiSnapshot{}
	needsCreation := map[string]bool{}
	for _, schedule := range schedules {
		needsCreation[schedule.Every] = true
	}
	for _, snapshot := range snapshots {
		klog.V(5).Infof("Checking snapshot %s/%s", snapshot.Namespace, snapshot.Name)
		keep := false
		for _, interval := range snapshot.Intervals {
			if numSnapshotsByInterval[interval] == 0 {
				parsed, err := ParseInterval(interval)
				if err != nil {
					return nil, nil, err
				}
				// This is the latest snapshot
				nextSnapshotTime := snapshot.Timestamp.Add(parsed)
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
		klog.Infof("need creation %v %v", k, v)
		if v {
			klog.V(5).Infof("Need creation for interval %s: %t", k, v)
			toCreate = append(toCreate, k)
		}
	}
	return toCreate, toDelete, nil
}

// ParseInterval parses an interval string as defined by gemini
func ParseInterval(str string) (time.Duration, error) {
	amt := 1
	every := str
	parts := strings.Split(str, " ")
	if len(parts) == 2 {
		every = parts[1]
		var err error
		amt, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Hour, fmt.Errorf("Could not parse interval %s", str)
		}
	}
	every = strings.TrimSuffix(every, "s")
	duration, ok := durations[every]
	if !ok {
		return time.Hour, fmt.Errorf("Could not find duration for interval %s", str)
	}
	ret := time.Duration(amt) * duration
	return ret, nil
}
