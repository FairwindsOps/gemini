package snapshots

import (
	"fmt"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

// ReconcileBackupsForSnapshotGroup handles any changes to SnapshotGroups
func ReconcileBackupsForSnapshotGroup(sg *v1.SnapshotGroup) error {
	klog.Infof("Reconciling SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	err := maybeCreatePVC(sg)
	if err != nil {
		return err
	}

	snapshots, err := listSnapshots(sg)
	if err != nil {
		return err
	}

	toCreate, toDelete := getSnapshotChanges(sg.Spec.Schedule, snapshots)

	err = deleteSnapshots(toDelete)
	if err != nil {
		return err
	}
	err = createSnapshotForIntervals(sg, toCreate)
	if err != nil {
		return err
	}

	return nil
}

func RestoreSnapshotGroup(sg *v1.SnapshotGroup) error {
	restorePoint := sg.ObjectMeta.Annotations[RestoreAnnotation]
	if restorePoint == "" {
		err := fmt.Errorf("SnapshotGroup %s/%s has invalid restore annotation %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
		return err
	}
	klog.Infof("Restoring SnapshotGroup %s/%s to %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, restorePoint)
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

func OnSnapshotGroupDelete(sg *v1.SnapshotGroup) error {
	// TODO(rbren): option to delete snapshots on group deletion
	name := sg.ObjectMeta.Name
	namespace := sg.ObjectMeta.Namespace
	klog.Infof("SnapshotGroup %s/%s was deleted. Taking no action. You may want to run kubectl delete volumesnapshots --all --namespace %s", namespace, name, namespace)
	return nil
}
