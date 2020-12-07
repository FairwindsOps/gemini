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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/fairwindsops/gemini/pkg/kube"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
)

func updateSnapshotGroup(sg *snapshotgroup.SnapshotGroup) error {
	klog.Infof("%s/%s: updating PVC spec", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	client := kube.GetClient()
	sg.Spec.Claim.Spec.VolumeName = ""
	_, err := client.SnapshotGroupClient.SnapshotGroups(sg.ObjectMeta.Namespace).Update(context.Background(), sg, metav1.UpdateOptions{})
	return err
}

// ReconcileBackupsForSnapshotGroup handles any changes to SnapshotGroups
func ReconcileBackupsForSnapshotGroup(sg *snapshotgroup.SnapshotGroup) error {
	klog.Infof("%s/%s: reconciling", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	pvc, err := maybeCreatePVC(sg)
	if err != nil {
		return err
	}
	sg.Spec.Claim.Spec = pvc.Spec
	err = updateSnapshotGroup(sg)
	if err != nil {
		return err
	}

	snapshots, err := ListSnapshots(sg)
	if err != nil {
		return err
	}
	klog.Infof("%s/%s: found %d existing snapshots", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, len(snapshots))

	toCreate, toDelete, err := getSnapshotChanges(sg.Spec.Schedule, snapshots)
	if err != nil {
		return err
	}
	klog.Infof("%s/%s: going to create %d, delete %d snapshots", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, len(toCreate), len(toDelete))

	err = deleteSnapshots(toDelete)
	if err != nil {
		return err
	}
	klog.Infof("%s/%s: deleted %d snapshots", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, len(toDelete))

	_, err = createSnapshotForIntervals(sg, toCreate)
	if err != nil {
		return err
	}
	klog.Infof("%s/%s: created %d snapshots", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, len(toCreate))

	return nil
}

// RestoreSnapshotGroup restores the PV to a particular snapshot
func RestoreSnapshotGroup(sg *snapshotgroup.SnapshotGroup, waitForRestoreSeconds int) error {
	restorePoint := sg.ObjectMeta.Annotations[RestoreAnnotation]
	if restorePoint == "" {
		err := fmt.Errorf("%s/%s: has invalid restore annotation %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
		return err
	}
	klog.Infof("%s/%s: restoring to %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
	snap, err := createSnapshotForRestore(sg)
	if err != nil {
		klog.Errorf("%s/%s: could not create failsafe snapshot before restore - %v", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, err)
		return err
	}
	_, err = waitUntilSnapshotReady(snap.ObjectMeta.Namespace, snap.ObjectMeta.Name, waitForRestoreSeconds)
	if err != nil {
		klog.Warningf("%s/%s: failed to create failsafe snapshot before restore - %v", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, err)
		klog.Warningf("%s/%s: proceeding with restore anyway", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	}
	err = restorePVC(sg)
	if err != nil {
		return err
	}
	return nil
}

// OnSnapshotGroupDelete is called when a SnapshotGroup is removed
func OnSnapshotGroupDelete(sg *snapshotgroup.SnapshotGroup) error {
	// TODO(rbren): option to delete snapshots on group deletion
	name := sg.ObjectMeta.Name
	namespace := sg.ObjectMeta.Namespace
	klog.Infof("%s/%s was deleted. Taking no action. You may want to run kubectl delete volumesnapshots --all --namespace %s", namespace, name, namespace)
	return nil
}
