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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/fairwindsops/gemini/pkg/fsr"
	"github.com/fairwindsops/gemini/pkg/kube"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1"
)

// FSRPollInterval is the requeue cadence while a snapshot is in a transitional
// state ("enabling" or "disabling"). AWS FSR warmup is minutes-scale
// (~60min/TiB), so polling faster wastes API calls and requeue work.
const FSRPollInterval = 60 * time.Second

// FSREnableTimeout is the maximum time a snapshot may stay in fsr-state=enabling
// before the reconciler gives up and writes fsr-state=failed.
const FSREnableTimeout = 2 * time.Hour

// FSRDisableTimeout is the maximum time a snapshot may stay in fsr-state=disabling
// before the reconciler gives up and writes fsr-state=failed. Mirrors the enable
// timeout; AWS disable is typically fast but network/throttling can stall it.
const FSRDisableTimeout = 2 * time.Hour

var (
	fsrClient        fsr.Client
	defaultFSRAZs    []string
	fsrGlobalEnabled = true
)

// SetFSRClient installs the AWS FSR client used by ReconcileFSR. main.go calls
// this once at startup; tests inject a fake.
func SetFSRClient(c fsr.Client) { fsrClient = c }

// SetDefaultFSRAZs installs the cluster-wide fallback AZ list used when a
// SnapshotGroup omits spec.fastSnapshotRestore.availabilityZones.
func SetDefaultFSRAZs(azs []string) { defaultFSRAZs = azs }

// SetFSRGlobalEnabled flips the cluster-wide FSR kill-switch. When false,
// ReconcileFSR short-circuits regardless of per-SnapshotGroup configuration.
// This is a pure skip — snapshots already FSR-enabled in AWS stay enabled;
// cleanup happens via per-SG `enabled: false` or manual aws ec2 calls.
func SetFSRGlobalEnabled(v bool) { fsrGlobalEnabled = v }

// ReconcileFSR drives the FSR state machine for a single SnapshotGroup.
//
// Returns the duration after which the controller should re-enqueue this SG.
// Zero means no time-based requeue is needed (state is steady or terminal);
// the existing informer event path will pick up future changes.
//
// The reconcile runs in two passes:
//
//  1. Enable pass (only if per-SG enabled=true): walk the newest ReadyToUse
//     snapshot through absent -> enabling -> enabled. See reconcileEnable.
//  2. Disable pass: walk every OTHER snapshot in the group through
//     enabling/enabled -> disabling -> disabled, calling DisableFastSnapshotRestores
//     once a replacement is warm (or the group has been opted out entirely).
//     See reconcileDisable.
//
// Contract (matches .claude/feat/snapshot/hot-snapshot-scaleup-gemini.md §3.2):
//   - absent    -> Enable, annotate "enabling" + fsr-enabled-at, requeue
//   - enabling  -> Describe; warm -> "enabled"; > FSREnableTimeout -> "failed"; else requeue
//   - enabled   -> no-op for the target; triggers disable of older peers
//   - disabling -> Describe; cold -> "disabled"; > FSRDisableTimeout -> "failed"; else requeue
//   - disabled  -> no-op (terminal; AWS state already cleared)
//   - failed    -> no-op (terminal; manual intervention required)
func ReconcileFSR(sg *snapshotgroup.SnapshotGroup) (time.Duration, error) {
	if !fsrGlobalEnabled {
		return 0, nil
	}
	// No fastSnapshotRestore block at all: Gemini never touched FSR for this SG,
	// so there is nothing to enable and nothing to clean up from our side.
	if sg.Spec.FastSnapshotRestore == nil {
		return 0, nil
	}
	perSGEnabled := sg.Spec.FastSnapshotRestore.Enabled
	if fsrClient == nil {
		if perSGEnabled {
			klog.Warningf("%s/%s: fastSnapshotRestore.enabled=true but no FSR client configured; skipping",
				sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
		}
		return 0, nil
	}

	azs := sg.Spec.FastSnapshotRestore.AvailabilityZones
	if len(azs) == 0 {
		azs = defaultFSRAZs
	}
	if len(azs) == 0 {
		if !perSGEnabled {
			// Opted out with no AZs recorded: we can't safely target any AZ for
			// Disable. Operators that need cleanup should either restore the AZs
			// block or disable FSR manually via aws ec2.
			return 0, nil
		}
		return 0, fmt.Errorf("%s/%s: fastSnapshotRestore enabled but no AZs configured (set spec.fastSnapshotRestore.availabilityZones or %s)",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, fsr.DefaultAZsEnvVar)
	}

	snapshots, err := ListSnapshots(sg)
	if err != nil {
		return 0, fmt.Errorf("list snapshots: %w", err)
	}

	var (
		target    *GeminiSnapshot
		requeueE  time.Duration
		enableErr error
	)
	if perSGEnabled {
		target = newestReadyToUse(snapshots)
		if target != nil {
			requeueE, enableErr = reconcileEnable(sg, target, azs)
			if enableErr != nil {
				return 0, enableErr
			}
		}
	}

	// Gate: only initiate new disable transitions when it's safe to do so.
	//   - per-SG enabled=false: user explicitly opted out; tear everything down.
	//   - per-SG enabled=true:  only after the replacement target is warm; otherwise
	//     we'd leave the group with no FSR coverage for minutes-to-hours.
	canInitiateDisable := !perSGEnabled ||
		(target != nil && fsrState(target) == FSRStateEnabled)

	requeueD, err := reconcileDisable(sg, snapshots, target, azs, canInitiateDisable)
	if err != nil {
		return 0, err
	}

	return minNonZero(requeueE, requeueD), nil
}

// reconcileEnable advances the target snapshot through absent -> enabling -> enabled.
func reconcileEnable(sg *snapshotgroup.SnapshotGroup, target *GeminiSnapshot, azs []string) (time.Duration, error) {
	state := fsrState(target)
	switch state {
	case "":
		return startEnable(sg, target, azs)
	case FSRStateEnabling:
		return pollEnable(sg, target, azs)
	case FSRStateEnabled, FSRStateFailed, FSRStateDisabled:
		return 0, nil
	case FSRStateDisabling:
		// Target should be the newest; it arriving in "disabling" means something
		// (a human? a prior teardown pass?) already started to tear it down. We
		// won't race that — leave it alone and let the disable-pass poller finish.
		klog.Warningf("%s/%s: newest ready snapshot %s is in fsr-state=disabling; skipping enable",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, target.Name)
		return 0, nil
	default:
		klog.Warningf("%s/%s: snapshot %s has unknown fsr-state=%q; treating as absent",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, target.Name, state)
		return startEnable(sg, target, azs)
	}
}

// reconcileDisable walks every snapshot in the group except target:
//   - {enabling, enabled}: if canInitiate, call Disable and annotate "disabling"
//   - {disabling}: poll AWS; cold everywhere -> "disabled"; timeout -> "failed"
//
// Returns the tightest requeue duration needed by any polling branch.
func reconcileDisable(sg *snapshotgroup.SnapshotGroup, snapshots []*GeminiSnapshot, target *GeminiSnapshot, azs []string, canInitiate bool) (time.Duration, error) {
	var requeue time.Duration
	for _, snap := range snapshots {
		if target != nil && snap.Name == target.Name && snap.Namespace == target.Namespace {
			continue
		}
		state := fsrState(snap)
		switch state {
		case FSRStateEnabling, FSRStateEnabled:
			if !canInitiate {
				continue
			}
			d, err := startDisable(sg, snap, azs)
			if err != nil {
				return 0, err
			}
			requeue = minNonZero(requeue, d)
		case FSRStateDisabling:
			d, err := pollDisable(sg, snap, azs)
			if err != nil {
				return 0, err
			}
			requeue = minNonZero(requeue, d)
		}
	}
	return requeue, nil
}

// fsrState reads the fsr-state annotation off a snapshot (empty string if unset).
func fsrState(s *GeminiSnapshot) string {
	if s == nil || s.VolumeSnapshot == nil {
		return ""
	}
	return s.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation]
}

// minNonZero returns the smaller of two durations, ignoring zeros. When both
// are zero, returns zero.
func minNonZero(a, b time.Duration) time.Duration {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}

// newestReadyToUse returns the snapshot with the highest timestamp whose
// VolumeSnapshot.Status.ReadyToUse is true. Snapshots are pre-sorted desc
// by timestamp by ListSnapshots, so we just pick the first ready one.
func newestReadyToUse(snapshots []*GeminiSnapshot) *GeminiSnapshot {
	for _, s := range snapshots {
		if s.VolumeSnapshot == nil || s.VolumeSnapshot.Status == nil {
			continue
		}
		if s.VolumeSnapshot.Status.ReadyToUse != nil && *s.VolumeSnapshot.Status.ReadyToUse {
			return s
		}
	}
	return nil
}

func startEnable(sg *snapshotgroup.SnapshotGroup, snap *GeminiSnapshot, azs []string) (time.Duration, error) {
	snapshotID, err := resolveSnapshotID(snap)
	if err != nil {
		// VSC may not have published snapshotHandle yet even though ReadyToUse=true.
		// Requeue and try again.
		klog.V(3).Infof("%s/%s: cannot resolve AWS snapshot ID for %s yet: %v",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, err)
		return FSRPollInterval, nil
	}
	klog.V(3).Infof("%s/%s: enabling FSR on %s (snapshotID=%s, azs=%v)",
		sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, snapshotID, azs)
	if err := fsrClient.Enable(context.TODO(), snapshotID, azs); err != nil {
		// Transient AWS errors: don't write "failed", let workqueue rate-limiter retry.
		return 0, fmt.Errorf("FSR Enable(%s): %w", snapshotID, err)
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := patchSnapshotAnnotations(snap, map[string]string{
		FSRStateAnnotation:     FSRStateEnabling,
		FSREnabledAtAnnotation: now,
	}); err != nil {
		return 0, fmt.Errorf("annotate %s as enabling: %w", snap.Name, err)
	}
	return FSRPollInterval, nil
}

func pollEnable(sg *snapshotgroup.SnapshotGroup, snap *GeminiSnapshot, azs []string) (time.Duration, error) {
	snapshotID, err := resolveSnapshotID(snap)
	if err != nil {
		return FSRPollInterval, nil
	}
	states, err := fsrClient.Describe(context.TODO(), snapshotID)
	if err != nil {
		return 0, fmt.Errorf("FSR Describe(%s): %w", snapshotID, err)
	}
	if fsr.IsWarmInAll(states, azs) {
		klog.V(3).Infof("%s/%s: FSR warm on %s in all target AZs", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name)
		if err := patchSnapshotAnnotations(snap, map[string]string{
			FSRStateAnnotation: FSRStateEnabled,
		}); err != nil {
			return 0, fmt.Errorf("annotate %s as enabled: %w", snap.Name, err)
		}
		return 0, nil
	}
	// Not warm yet. Check timeout.
	startedAt, ok := parseFSREnabledAt(snap)
	if ok && time.Since(startedAt) > FSREnableTimeout {
		klog.Warningf("%s/%s: FSR on %s exceeded %s warmup timeout; marking failed",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, FSREnableTimeout)
		if err := patchSnapshotAnnotations(snap, map[string]string{
			FSRStateAnnotation: FSRStateFailed,
		}); err != nil {
			return 0, fmt.Errorf("annotate %s as failed: %w", snap.Name, err)
		}
		return 0, nil
	}
	return FSRPollInterval, nil
}

func parseFSREnabledAt(snap *GeminiSnapshot) (time.Time, bool) {
	raw := snap.VolumeSnapshot.ObjectMeta.Annotations[FSREnabledAtAnnotation]
	if raw == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

func startDisable(sg *snapshotgroup.SnapshotGroup, snap *GeminiSnapshot, azs []string) (time.Duration, error) {
	snapshotID, err := resolveSnapshotID(snap)
	if err != nil {
		klog.V(3).Infof("%s/%s: cannot resolve AWS snapshot ID for %s yet: %v",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, err)
		return FSRPollInterval, nil
	}
	klog.V(3).Infof("%s/%s: disabling FSR on %s (snapshotID=%s, azs=%v)",
		sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, snapshotID, azs)
	if err := fsrClient.Disable(context.TODO(), snapshotID, azs); err != nil {
		return 0, fmt.Errorf("FSR Disable(%s): %w", snapshotID, err)
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := patchSnapshotAnnotations(snap, map[string]string{
		FSRStateAnnotation:      FSRStateDisabling,
		FSRDisabledAtAnnotation: now,
	}); err != nil {
		return 0, fmt.Errorf("annotate %s as disabling: %w", snap.Name, err)
	}
	return FSRPollInterval, nil
}

func pollDisable(sg *snapshotgroup.SnapshotGroup, snap *GeminiSnapshot, azs []string) (time.Duration, error) {
	snapshotID, err := resolveSnapshotID(snap)
	if err != nil {
		return FSRPollInterval, nil
	}
	states, err := fsrClient.Describe(context.TODO(), snapshotID)
	if err != nil {
		return 0, fmt.Errorf("FSR Describe(%s): %w", snapshotID, err)
	}
	if fsr.IsColdInAll(states, azs) {
		klog.V(3).Infof("%s/%s: FSR cold on %s in all target AZs", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name)
		if err := patchSnapshotAnnotations(snap, map[string]string{
			FSRStateAnnotation: FSRStateDisabled,
		}); err != nil {
			return 0, fmt.Errorf("annotate %s as disabled: %w", snap.Name, err)
		}
		return 0, nil
	}
	startedAt, ok := parseFSRDisabledAt(snap)
	if ok && time.Since(startedAt) > FSRDisableTimeout {
		klog.Warningf("%s/%s: FSR disable on %s exceeded %s cooldown timeout; marking failed",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, snap.Name, FSRDisableTimeout)
		if err := patchSnapshotAnnotations(snap, map[string]string{
			FSRStateAnnotation: FSRStateFailed,
		}); err != nil {
			return 0, fmt.Errorf("annotate %s as failed: %w", snap.Name, err)
		}
		return 0, nil
	}
	return FSRPollInterval, nil
}

func parseFSRDisabledAt(snap *GeminiSnapshot) (time.Time, bool) {
	raw := snap.VolumeSnapshot.ObjectMeta.Annotations[FSRDisabledAtAnnotation]
	if raw == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

// resolveSnapshotID maps a Gemini-managed VolumeSnapshot to its AWS EBS snapshot
// ID by following Status.BoundVolumeSnapshotContentName -> VSC.Status.SnapshotHandle.
func resolveSnapshotID(snap *GeminiSnapshot) (string, error) {
	contentName, err := getSnapshotContentName(snap)
	if err != nil {
		return "", err
	}
	details, err := getSnapshotContentDetails(contentName)
	if err != nil {
		return "", err
	}
	if details.SnapshotHandle == "" {
		return "", fmt.Errorf("VolumeSnapshotContent %s has empty snapshotHandle", contentName)
	}
	return details.SnapshotHandle, nil
}

// patchSnapshotAnnotations applies a strategic-merge patch that adds (or
// overwrites) the given annotations on a VolumeSnapshot. Other annotations
// are preserved by the merge patch semantics.
//
// On success the patched annotations are also merged into the in-memory
// snap.VolumeSnapshot so that subsequent reads in the same reconcile see
// the new values. Without this, code paths like ReconcileFSR that run an
// enable pass (which may flip fsr-state to "enabled") and then gate the
// disable pass on reading that same annotation would always see the stale
// pre-patch value and skip disable until the next reconcile event.
func patchSnapshotAnnotations(snap *GeminiSnapshot, anns map[string]string) error {
	body := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": anns,
		},
	}
	patch, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := kube.GetClient()
	if _, err = client.SnapshotClient.Namespace(snap.Namespace).Patch(
		context.TODO(), snap.Name, types.MergePatchType, patch, metav1.PatchOptions{},
	); err != nil {
		return err
	}
	if snap.VolumeSnapshot != nil {
		if snap.VolumeSnapshot.ObjectMeta.Annotations == nil {
			snap.VolumeSnapshot.ObjectMeta.Annotations = map[string]string{}
		}
		for k, v := range anns {
			snap.VolumeSnapshot.ObjectMeta.Annotations[k] = v
		}
	}
	return nil
}
