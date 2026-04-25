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
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/gemini/pkg/fsr"
	"github.com/fairwindsops/gemini/pkg/kube"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1"
)

// fsrTestSetup wires the package-level fsrClient/defaultFSRAZs to a fresh fake
// per test and returns the fake so the test can poke AWS state directly.
func fsrTestSetup(t *testing.T) *fsr.FakeClient {
	t.Helper()
	kube.SetFakeClient()
	fake := fsr.NewFakeClient()
	SetFSRClient(fake)
	SetDefaultFSRAZs(nil)
	t.Cleanup(func() {
		SetFSRClient(nil)
		SetDefaultFSRAZs(nil)
	})
	return fake
}

// makeSG returns a SnapshotGroup wired for FSR with the given AZs.
// fastSnapshotRestore is left nil if azs is nil.
func makeSG(name, ns string, fsrEnabled bool, azs []string) *snapshotgroup.SnapshotGroup {
	sg := &snapshotgroup.SnapshotGroup{
		TypeMeta: metav1.TypeMeta{APIVersion: snapshotgroup.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: map[string]string{},
		},
		Spec: snapshotgroup.SnapshotGroupSpec{
			Schedule: []snapshotgroup.SnapshotSchedule{{Every: "1 second", Keep: 1}},
		},
	}
	if fsrEnabled {
		sg.Spec.FastSnapshotRestore = &snapshotgroup.FastSnapshotRestoreSpec{
			Enabled:           true,
			AvailabilityZones: azs,
		}
	}
	return sg
}

// createSnapshotForTest creates one Gemini-managed snapshot in the cluster so
// ListSnapshots returns it. The fake kube client's reactor auto-fills
// readyToUse=true and a backing VolumeSnapshotContent with snapshotHandle.
func createSnapshotForTest(t *testing.T, sg *snapshotgroup.SnapshotGroup) *GeminiSnapshot {
	t.Helper()
	snap, err := createSnapshot(sg, map[string]string{})
	assert.NoError(t, err)
	return snap
}

func TestReconcileFSR_Disabled_NoOp(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", false, nil)

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)
	assert.Empty(t, fake.EnableCalls)
}

func TestReconcileFSR_NoAZs_Errors(t *testing.T) {
	fsrTestSetup(t)
	sg := makeSG("foo", "default", true, nil) // no AZs and no env default

	_, err := ReconcileFSR(sg)
	assert.Error(t, err)
}

func TestReconcileFSR_NoSnapshotsYet_NoOp(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)
	assert.Empty(t, fake.EnableCalls)
}

func TestReconcileFSR_AbsentTransitionsToEnabling(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a", "az-b"})
	createSnapshotForTest(t, sg)

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, FSRPollInterval, requeue)
	assert.Len(t, fake.EnableCalls, 1)
	assert.Equal(t, []string{"az-a", "az-b"}, fake.EnableCalls[0].AZs)

	// The annotation should now be "enabling".
	snaps, err := ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Len(t, snaps, 1)
	anns := snaps[0].VolumeSnapshot.ObjectMeta.Annotations
	assert.Equal(t, FSRStateEnabling, anns[FSRStateAnnotation])
	assert.NotEmpty(t, anns[FSREnabledAtAnnotation])
}

func TestReconcileFSR_EnablingNotWarm_RequeuesUnchanged(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})
	createSnapshotForTest(t, sg)

	// First pass: absent -> enabling.
	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Len(t, fake.EnableCalls, 1)

	// Second pass: AWS still reports "enabling" (the fake's default after Enable),
	// so the reconciler should requeue and not change the annotation.
	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, FSRPollInterval, requeue)
	assert.Len(t, fake.EnableCalls, 1, "Enable should not be called again while in enabling state")

	snaps, _ := ListSnapshots(sg)
	assert.Equal(t, FSRStateEnabling, snaps[0].VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_EnablingWarm_TransitionsToEnabled(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a", "az-b"})
	snap := createSnapshotForTest(t, sg)

	// First pass: absent -> enabling.
	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)

	// Simulate AWS warming up in both AZs.
	snapshotID, err := resolveSnapshotID(snap)
	assert.NoError(t, err)
	fake.SetState(snapshotID, "az-a", "enabled")
	fake.SetState(snapshotID, "az-b", "enabled")

	// Second pass: enabling -> enabled.
	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)

	snaps, _ := ListSnapshots(sg)
	assert.Equal(t, FSRStateEnabled, snaps[0].VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_PartialWarm_StaysEnabling(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a", "az-b"})
	snap := createSnapshotForTest(t, sg)

	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)

	snapshotID, _ := resolveSnapshotID(snap)
	fake.SetState(snapshotID, "az-a", "enabled")
	// az-b deliberately left "enabling".

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, FSRPollInterval, requeue)

	snaps, _ := ListSnapshots(sg)
	assert.Equal(t, FSRStateEnabling, snaps[0].VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_TimeoutTransitionsToFailed(t *testing.T) {
	fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})
	snap := createSnapshotForTest(t, sg)

	// Simulate that we issued Enable >2h ago: write the annotation by hand.
	longAgo := strconv.FormatInt(time.Now().Add(-3*time.Hour).Unix(), 10)
	err := patchSnapshotAnnotations(snap, map[string]string{
		FSRStateAnnotation:     FSRStateEnabling,
		FSREnabledAtAnnotation: longAgo,
	})
	assert.NoError(t, err)

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)

	snaps, _ := ListSnapshots(sg)
	assert.Equal(t, FSRStateFailed, snaps[0].VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_EnableErrorBubbles(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})
	createSnapshotForTest(t, sg)

	fake.EnableErr = errors.New("AWS throttling")

	requeue, err := ReconcileFSR(sg)
	assert.Error(t, err)
	assert.Equal(t, time.Duration(0), requeue)

	// No annotation should have been written.
	snaps, _ := ListSnapshots(sg)
	assert.Empty(t, snaps[0].VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_AlreadyEnabled_NoOp(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})
	snap := createSnapshotForTest(t, sg)

	err := patchSnapshotAnnotations(snap, map[string]string{FSRStateAnnotation: FSRStateEnabled})
	assert.NoError(t, err)

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)
	assert.Empty(t, fake.EnableCalls)
}

func TestReconcileFSR_DefaultAZsFallback(t *testing.T) {
	fake := fsrTestSetup(t)
	SetDefaultFSRAZs([]string{"default-az"})
	// SG omits availabilityZones.
	sg := makeSG("foo", "default", true, nil)
	createSnapshotForTest(t, sg)

	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Len(t, fake.EnableCalls, 1)
	assert.Equal(t, []string{"default-az"}, fake.EnableCalls[0].AZs)
}

// Sanity check that reconciling picks the *newest* ReadyToUse snapshot.
// We seed two snapshots with different timestamps and verify Enable was
// called against the more recent one.
func TestReconcileFSR_PicksNewestReadyToUse(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	// Older snapshot.
	older, err := createSnapshot(sg, map[string]string{})
	assert.NoError(t, err)
	// Force a later timestamp on a second snapshot by sleeping a second.
	time.Sleep(1100 * time.Millisecond)
	newer, err := createSnapshot(sg, map[string]string{})
	assert.NoError(t, err)
	assert.NotEqual(t, older.Name, newer.Name)

	_, err = ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Len(t, fake.EnableCalls, 1)

	newerID, err := resolveSnapshotID(newer)
	assert.NoError(t, err)
	assert.Equal(t, newerID, fake.EnableCalls[0].SnapshotID)

	// Also confirm only the newer one got the annotation.
	got, err := GetSnapshot("default", newer.Name)
	assert.NoError(t, err)
	assert.Equal(t, FSRStateEnabling, got.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
	gotOlder, err := GetSnapshot("default", older.Name)
	assert.NoError(t, err)
	assert.Empty(t, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

// createSnapshotWithState seeds a snapshot whose annotations already encode a
// specific fsr-state (plus an optional fsr-enabled-at / fsr-disabled-at). Used
// by disable-path tests that want to start from a mid-state configuration.
func createSnapshotWithState(t *testing.T, sg *snapshotgroup.SnapshotGroup, state string) *GeminiSnapshot {
	t.Helper()
	snap := createSnapshotForTest(t, sg)
	err := patchSnapshotAnnotations(snap, map[string]string{
		FSRStateAnnotation: state,
	})
	assert.NoError(t, err)
	return snap
}

func TestReconcileFSR_OlderEnabledDisabledWhenNewerWarm(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	// Older snapshot: already fsr-state=enabled, fake AWS shows it enabled.
	older := createSnapshotWithState(t, sg, FSRStateEnabled)
	olderID, err := resolveSnapshotID(older)
	assert.NoError(t, err)
	fake.SetState(olderID, "az-a", "enabled")

	// Newer snapshot, also already enabled in state + AWS.
	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, err := resolveSnapshotID(newer)
	assert.NoError(t, err)
	fake.SetState(newerID, "az-a", "enabled")

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, FSRPollInterval, requeue)

	assert.Len(t, fake.DisableCalls, 1, "older snapshot should have been disabled")
	assert.Equal(t, olderID, fake.DisableCalls[0].SnapshotID)

	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateDisabling, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
	assert.NotEmpty(t, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRDisabledAtAnnotation])

	gotNewer, _ := GetSnapshot("default", newer.Name)
	assert.Equal(t, FSRStateEnabled, gotNewer.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_OlderEnablingAlsoDisabled(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	// Older snapshot got stuck in "enabling" (never warmed).
	older := createSnapshotWithState(t, sg, FSRStateEnabling)
	olderID, err := resolveSnapshotID(older)
	assert.NoError(t, err)
	fake.SetState(olderID, "az-a", "enabling")

	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, err := resolveSnapshotID(newer)
	assert.NoError(t, err)
	fake.SetState(newerID, "az-a", "enabled")

	_, err = ReconcileFSR(sg)
	assert.NoError(t, err)

	assert.Len(t, fake.DisableCalls, 1)
	assert.Equal(t, olderID, fake.DisableCalls[0].SnapshotID)

	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateDisabling, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_DisableNotStartedUntilNewerIsWarm(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	older := createSnapshotWithState(t, sg, FSRStateEnabled)
	olderID, _ := resolveSnapshotID(older)
	fake.SetState(olderID, "az-a", "enabled")

	time.Sleep(1100 * time.Millisecond)
	// Newer snapshot is still warming up: fsr-state=enabling.
	newer := createSnapshotWithState(t, sg, FSRStateEnabling)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabling")

	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)

	assert.Empty(t, fake.DisableCalls, "disable must not start while replacement is still warming up")
	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateEnabled, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_DisablingStillEnabled_RequeuesUnchanged(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	older := createSnapshotForTest(t, sg)
	olderID, _ := resolveSnapshotID(older)
	// Mid-transition: Gemini wrote "disabling" but AWS still reports "enabled".
	startedAt := strconv.FormatInt(time.Now().Unix(), 10)
	err := patchSnapshotAnnotations(older, map[string]string{
		FSRStateAnnotation:      FSRStateDisabling,
		FSRDisabledAtAnnotation: startedAt,
	})
	assert.NoError(t, err)
	fake.SetState(olderID, "az-a", "enabled")

	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabled")

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, FSRPollInterval, requeue)
	assert.Empty(t, fake.DisableCalls, "Disable should not be called again while in disabling state")

	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateDisabling, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_DisablingCleared_TransitionsToDisabled(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	older := createSnapshotForTest(t, sg)
	olderID, _ := resolveSnapshotID(older)
	startedAt := strconv.FormatInt(time.Now().Unix(), 10)
	err := patchSnapshotAnnotations(older, map[string]string{
		FSRStateAnnotation:      FSRStateDisabling,
		FSRDisabledAtAnnotation: startedAt,
	})
	assert.NoError(t, err)
	// AWS now reports disabled in all target AZs.
	fake.SetState(olderID, "az-a", "disabled")

	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabled")

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)

	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateDisabled, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_DisableTimeoutTransitionsToFailed(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	older := createSnapshotForTest(t, sg)
	olderID, _ := resolveSnapshotID(older)
	// disabled-at is >2h ago, AWS still reports "enabled".
	longAgo := strconv.FormatInt(time.Now().Add(-3*time.Hour).Unix(), 10)
	err := patchSnapshotAnnotations(older, map[string]string{
		FSRStateAnnotation:      FSRStateDisabling,
		FSRDisabledAtAnnotation: longAgo,
	})
	assert.NoError(t, err)
	fake.SetState(olderID, "az-a", "enabled")

	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabled")

	_, err = ReconcileFSR(sg)
	assert.NoError(t, err)

	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateFailed, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

func TestReconcileFSR_DisableErrorBubbles(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	older := createSnapshotWithState(t, sg, FSRStateEnabled)
	olderID, _ := resolveSnapshotID(older)
	fake.SetState(olderID, "az-a", "enabled")

	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotWithState(t, sg, FSRStateEnabled)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabled")

	fake.DisableErr = errors.New("AWS throttling")

	_, err := ReconcileFSR(sg)
	assert.Error(t, err)

	// No annotation change on transient error.
	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateEnabled, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

// Per-SG opt-out: setting enabled:false triggers a disable pass on every
// snapshot that still has fsr-state in {enabling, enabled}.
func TestReconcileFSR_PerSGDisabled_TriggersDisable(t *testing.T) {
	fake := fsrTestSetup(t)
	// Create SG with FSR fully enabled so we can seed state, then flip.
	sg := makeSG("foo", "default", true, []string{"az-a"})
	snap := createSnapshotWithState(t, sg, FSRStateEnabled)
	snapID, _ := resolveSnapshotID(snap)
	fake.SetState(snapID, "az-a", "enabled")

	// Operator opts out — fastSnapshotRestore block is kept (AZs still present)
	// but Enabled flips to false.
	sg.Spec.FastSnapshotRestore.Enabled = false

	_, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Len(t, fake.DisableCalls, 1)
	assert.Equal(t, snapID, fake.DisableCalls[0].SnapshotID)

	got, _ := GetSnapshot("default", snap.Name)
	assert.Equal(t, FSRStateDisabling, got.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

// Regression: a single ReconcileFSR call that drives the newer snapshot
// enabling -> enabled must also initiate Disable on an older already-enabled
// peer in the same pass. Previously the disable-pass gate read a stale
// in-memory annotation and would skip disable until the next reconcile event.
func TestReconcileFSR_EnablingToEnabled_TriggersDisableOfOlderSamePass(t *testing.T) {
	fake := fsrTestSetup(t)
	sg := makeSG("foo", "default", true, []string{"az-a"})

	// Older snapshot already fully enabled (state + AWS).
	older := createSnapshotWithState(t, sg, FSRStateEnabled)
	olderID, _ := resolveSnapshotID(older)
	fake.SetState(olderID, "az-a", "enabled")

	// Newer snapshot mid-warmup: state=enabling with a fresh fsr-enabled-at so
	// the timeout branch doesn't fire. AWS has warmed it up under the hood.
	time.Sleep(1100 * time.Millisecond)
	newer := createSnapshotForTest(t, sg)
	now := strconv.FormatInt(time.Now().Unix(), 10)
	err := patchSnapshotAnnotations(newer, map[string]string{
		FSRStateAnnotation:     FSRStateEnabling,
		FSREnabledAtAnnotation: now,
	})
	assert.NoError(t, err)
	newerID, _ := resolveSnapshotID(newer)
	fake.SetState(newerID, "az-a", "enabled")

	_, err = ReconcileFSR(sg)
	assert.NoError(t, err)

	// Newer should have flipped to enabled, AND older should now be disabling
	// — both in a single reconcile call.
	gotNewer, _ := GetSnapshot("default", newer.Name)
	assert.Equal(t, FSRStateEnabled, gotNewer.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])

	assert.Len(t, fake.DisableCalls, 1, "disable must run in same reconcile once newer transitions to enabled")
	assert.Equal(t, olderID, fake.DisableCalls[0].SnapshotID)
	gotOlder, _ := GetSnapshot("default", older.Name)
	assert.Equal(t, FSRStateDisabling, gotOlder.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation])
}

// Global kill-switch: even with per-SG enabled=true and seeded state, the
// reconciler does nothing when the global flag is off.
func TestReconcileFSR_GlobalDisabled_NoOp(t *testing.T) {
	fake := fsrTestSetup(t)
	SetFSRGlobalEnabled(false)
	t.Cleanup(func() { SetFSRGlobalEnabled(true) })

	sg := makeSG("foo", "default", true, []string{"az-a"})
	createSnapshotForTest(t, sg)

	requeue, err := ReconcileFSR(sg)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), requeue)
	assert.Empty(t, fake.EnableCalls)
	assert.Empty(t, fake.DisableCalls)
}

// Compile-time guard: keep the symbols imported even if a future test edit
// drops one. (Avoids an "imported and not used" churn round-trip.)
var _ = context.Background
