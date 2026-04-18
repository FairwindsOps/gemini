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

// FSRPollInterval is the requeue cadence while a snapshot is in the "enabling"
// state. AWS FSR warmup is minutes-scale (~60min/TiB), so polling faster wastes
// API calls and requeue work.
const FSRPollInterval = 60 * time.Second

// FSREnableTimeout is the maximum time a snapshot may stay in fsr-state=enabling
// before the reconciler gives up and writes fsr-state=failed.
const FSREnableTimeout = 2 * time.Hour

var (
	fsrClient     fsr.Client
	defaultFSRAZs []string
)

// SetFSRClient installs the AWS FSR client used by ReconcileFSR. main.go calls
// this once at startup; tests inject a fake.
func SetFSRClient(c fsr.Client) { fsrClient = c }

// SetDefaultFSRAZs installs the cluster-wide fallback AZ list used when a
// SnapshotGroup omits spec.fastSnapshotRestore.availabilityZones.
func SetDefaultFSRAZs(azs []string) { defaultFSRAZs = azs }

// ReconcileFSR drives the FSR state machine for a single SnapshotGroup.
//
// Returns the duration after which the controller should re-enqueue this SG.
// Zero means no time-based requeue is needed (state is steady or terminal);
// the existing informer event path will pick up future changes.
//
// Contract (matches docs in .claude/feat/snapshot/hot-snapshot-scaleup-gemini.md §3.2):
//   - absent  -> call Enable, annotate "enabling" + record fsr-enabled-at, requeue
//   - enabling -> Describe; warm -> "enabled"; > FSREnableTimeout -> "failed"; else requeue
//   - enabled  -> no-op
//   - failed   -> no-op (terminal; manual intervention required)
//
// MVP: this only handles the enable path. Disable rotation (annotating older
// snapshots "disabling"/"disabled" + calling DisableFastSnapshotRestores) is
// deferred — see .claude/good-to-haves.md.
func ReconcileFSR(sg *snapshotgroup.SnapshotGroup) (time.Duration, error) {
	if sg.Spec.FastSnapshotRestore == nil || !sg.Spec.FastSnapshotRestore.Enabled {
		return 0, nil
	}
	if fsrClient == nil {
		klog.Warningf("%s/%s: fastSnapshotRestore.enabled=true but no FSR client configured; skipping",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
		return 0, nil
	}

	azs := sg.Spec.FastSnapshotRestore.AvailabilityZones
	if len(azs) == 0 {
		azs = defaultFSRAZs
	}
	if len(azs) == 0 {
		return 0, fmt.Errorf("%s/%s: fastSnapshotRestore enabled but no AZs configured (set spec.fastSnapshotRestore.availabilityZones or %s)",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, fsr.DefaultAZsEnvVar)
	}

	snapshots, err := ListSnapshots(sg)
	if err != nil {
		return 0, fmt.Errorf("list snapshots: %w", err)
	}
	latest := newestReadyToUse(snapshots)
	if latest == nil {
		klog.V(5).Infof("%s/%s: no ReadyToUse snapshot yet; FSR reconcile is a no-op",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
		return 0, nil
	}

	state := latest.VolumeSnapshot.ObjectMeta.Annotations[FSRStateAnnotation]
	switch state {
	case "":
		return startEnable(sg, latest, azs)
	case FSRStateEnabling:
		return pollEnable(sg, latest, azs)
	case FSRStateEnabled, FSRStateFailed:
		return 0, nil
	default:
		klog.Warningf("%s/%s: snapshot %s has unknown fsr-state=%q; treating as absent",
			sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, latest.Name, state)
		return startEnable(sg, latest, azs)
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
	_, err = client.SnapshotClient.Namespace(snap.Namespace).Patch(
		context.TODO(), snap.Name, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	return err
}
