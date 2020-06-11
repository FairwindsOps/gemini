package snapshots

import (
	"fmt"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

// ReconcileBackupsForSnapshotGroup handles any changes to SnapshotGroups
func ReconcileBackupsForSnapshotGroup(sg *v1.SnapshotGroup) error {
	klog.Infof("%s/%s: reconciling", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	err := maybeCreatePVC(sg)
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

	err = createSnapshotForIntervals(sg, toCreate)
	if err != nil {
		return err
	}
	klog.Infof("%s/%s: created %d snapshots", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, len(toCreate))

	return nil
}

// RestoreSnapshotGroup restores the PV to a particular snapshot
func RestoreSnapshotGroup(sg *v1.SnapshotGroup) error {
	restorePoint := sg.ObjectMeta.Annotations[RestoreAnnotation]
	if restorePoint == "" {
		err := fmt.Errorf("%s/%s: has invalid restore annotation %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
		return err
	}
	klog.Infof("%s/%s: restoring to %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
	err := createSnapshotForRestore(sg)
	if err != nil {
		return err
	}
	err = deletePVC(sg)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	err = restorePVC(sg)
	if err != nil {
		return err
	}
	return nil
}

// OnSnapshotGroupDelete is called when a SnapshotGroup is removed
func OnSnapshotGroupDelete(sg *v1.SnapshotGroup) error {
	// TODO(rbren): option to delete snapshots on group deletion
	name := sg.ObjectMeta.Name
	namespace := sg.ObjectMeta.Namespace
	klog.Infof("%s/%s was deleted. Taking no action. You may want to run kubectl delete volumesnapshots --all --namespace %s", namespace, name, namespace)
	return nil
}
