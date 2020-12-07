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
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/gemini/pkg/kube"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"

	snapshotsv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
)

// GeminiSnapshot represents a VolumeSnapshot created by Gemini
type GeminiSnapshot struct {
	Namespace string
	Name      string
	Intervals []string
	Timestamp time.Time
	Restore   string
}

// ListSnapshots returns all snapshots associated with a particular SnapshotGroup
func ListSnapshots(sg *snapshotgroup.SnapshotGroup) ([]GeminiSnapshot, error) {
	client := kube.GetClient()
	snapshots, err := client.SnapshotClient.Namespace(sg.ObjectMeta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	GeminiSnapshots := []GeminiSnapshot{}
	for _, snapshot := range snapshots.Items {
		snapshotMeta, err := meta.Accessor(&snapshot)
		if err != nil {
			return nil, err
		}
		annotations := snapshotMeta.GetAnnotations()
		if managedBy, ok := annotations[managedByAnnotation]; !ok || managedBy != managerName {
			continue
		}
		if annotations[GroupNameAnnotation] != sg.ObjectMeta.Name {
			continue
		}
		timestampStr := annotations[TimestampAnnotation]
		timestamp, err := strconv.Atoi(timestampStr)
		if err != nil {
			klog.Errorf("%s/%s: failed to parse unix timestamp %s for %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, timestampStr, snapshotMeta.GetName())
			continue
		}
		intervals := []string{}
		intervalsStr := annotations[IntervalsAnnotation]
		if intervalsStr != "" {
			intervals = strings.Split(intervalsStr, intervalsSeparator)
		}
		GeminiSnapshots = append(GeminiSnapshots, GeminiSnapshot{
			Namespace: snapshotMeta.GetNamespace(),
			Name:      snapshotMeta.GetName(),
			Timestamp: time.Unix(int64(timestamp), 0),
			Intervals: intervals,
			Restore:   annotations[RestoreAnnotation],
		})
	}
	sort.Slice(GeminiSnapshots, func(i, j int) bool {
		return GeminiSnapshots[j].Timestamp.Before(GeminiSnapshots[i].Timestamp)
	})
	return GeminiSnapshots, nil
}

func GetSnapshot(namespace, name string) (*snapshotsv1.VolumeSnapshot, error) {
	client := kube.GetClient()
	snapClient := client.SnapshotClient.Namespace(namespace)
	snapUnst, err := snapClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return parseSnapshot(snapUnst)
}

func parseSnapshot(unst *unstructured.Unstructured) (*snapshotsv1.VolumeSnapshot, error) {
	b, err := json.Marshal(unst)
	if err != nil {
		return nil, err
	}
	snap := snapshotsv1.VolumeSnapshot{}
	err = json.Unmarshal(b, &snap)
	return &snap, err
}

// createSnapshot creates a new snappshot for a given SnapshotGroup
func createSnapshot(sg *snapshotgroup.SnapshotGroup, annotations map[string]string) (*snapshotsv1.VolumeSnapshot, error) {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	annotations[TimestampAnnotation] = timestamp
	annotations[managedByAnnotation] = managerName
	annotations[GroupNameAnnotation] = sg.ObjectMeta.Name

	snapshot := snapshotsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   sg.ObjectMeta.Namespace,
			Name:        sg.ObjectMeta.Name + "-" + timestamp,
			Annotations: annotations,
		},
		Spec: sg.Spec.Template.Spec,
	}
	name := getPVCName(sg)
	klog.Infof("%s/%s: creating snapshot for PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, name)
	snapshot.Spec.Source.PersistentVolumeClaimName = &name

	marshaled, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	unst := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	err = json.Unmarshal(marshaled, &unst.Object)
	if err != nil {
		return nil, err
	}
	client := kube.GetClient()
	unst.Object["kind"] = "VolumeSnapshot"
	unst.Object["apiVersion"] = client.VolumeSnapshotVersion

	if strings.HasSuffix(client.VolumeSnapshotVersion, "v1alpha1") {
		// There is a slight change in `source` from alpha to beta
		spec := unst.Object["spec"].(map[string]interface{})
		source := spec["source"].(map[string]interface{})
		delete(source, "persistentVolumeClaimName")
		source["name"] = name
		source["kind"] = "PersistentVolumeClaim"
		spec["source"] = source
		unst.Object["spec"] = spec
	}

	snapClient := client.SnapshotClient.Namespace(snapshot.ObjectMeta.Namespace)
	snap, err := snapClient.Create(&unst, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return parseSnapshot(snap)
}

func createSnapshotForIntervals(sg *snapshotgroup.SnapshotGroup, intervals []string) (*snapshotsv1.VolumeSnapshot, error) {
	if len(intervals) == 0 {
		return nil, nil
	}
	klog.V(5).Infof("%s/%s: creating snapshot for intervals %v", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, intervals)
	annotations := map[string]string{
		IntervalsAnnotation: strings.Join(intervals, intervalsSeparator),
	}
	return createSnapshot(sg, annotations)
}

func createSnapshotForRestore(sg *snapshotgroup.SnapshotGroup) (*snapshotsv1.VolumeSnapshot, error) {
	restore := sg.ObjectMeta.Annotations[RestoreAnnotation]
	existing, err := ListSnapshots(sg)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range existing {
		if snapshot.Restore == restore {
			klog.V(5).Infof("%s/%s: snapshot already exists for timestamp %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restore)
			return nil, nil
		}
	}
	klog.V(5).Infof("%s/%s: creating snapshot for restore %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restore)
	annotations := map[string]string{
		RestoreAnnotation: restore,
	}
	return createSnapshot(sg, annotations)
}

func deleteSnapshots(toDelete []GeminiSnapshot) error {
	klog.V(5).Infof("Deleting %d expired snapshots", len(toDelete))
	client := kube.GetClient()
	for _, snapshot := range toDelete {
		snapClient := client.SnapshotClient.Namespace(snapshot.Namespace)
		err := snapClient.Delete(snapshot.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.V(5).Infof("Deleted snapshot %s/%s", snapshot.Namespace, snapshot.Name)
	}
	return nil
}

func waitUntilSnapshotReady(namespace, name string, readyTimeoutSeconds int) (*snapshotsv1.VolumeSnapshot, error) {
	timeout := time.After(time.Duration(readyTimeoutSeconds) * time.Second)
	tick := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			return nil, errors.New("timed out")
		case <-tick:
			snapshot, err := GetSnapshot(namespace, name)
			if err != nil {
				return nil, err
			} else if snapshot != nil && snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse {
				return snapshot, nil
			}
		}
	}
}
