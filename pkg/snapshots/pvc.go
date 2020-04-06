package snapshots

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

func createPVC(sg *v1.SnapshotGroup, spec corev1.PersistentVolumeClaimSpec, annotations map[string]string) error {
	klog.Infof("Creating PVC for SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[managedByAnnotation] = managerName
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sg.ObjectMeta.Name,
			Namespace:   sg.ObjectMeta.Namespace,
			Annotations: annotations,
		},
		Spec: spec,
	}
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	_, err := pvcClient.Create(context.TODO(), pvc, metav1.CreateOptions{})
	return err
}

func maybeCreatePVC(sg *v1.SnapshotGroup) error {
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(context.TODO(), sg.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("PVC %s not found, creating it", pvc.ObjectMeta.Name)
		err := createPVC(sg, sg.Spec.Claim.Spec, nil)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("Found pvc %s", pvc.ObjectMeta.Name)
	}
	return nil
}

func restorePVC(sg *v1.SnapshotGroup) error {
	klog.Infof("Restoring PVC for SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	restorePoint := sg.ObjectMeta.Annotations[RestoreAnnotation]
	annotations := map[string]string{
		RestoreAnnotation: restorePoint,
	}
	spec := sg.Spec.Claim.Spec
	apiGroup := "snapshot.storage.k8s.io"
	spec.DataSource = &corev1.TypedLocalObjectReference{
		APIGroup: &apiGroup,
		Kind:     "VolumeSnapshot",
		Name:     sg.ObjectMeta.Name + "-" + restorePoint,
	}
	return createPVC(sg, spec, annotations)
}

func deletePVC(sg *v1.SnapshotGroup) error {
	klog.Infof("Deleting PVC for SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	return pvcClient.Delete(context.TODO(), sg.ObjectMeta.Name, metav1.DeleteOptions{})
}
