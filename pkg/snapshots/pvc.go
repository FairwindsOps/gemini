package snapshots

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/fairwindsops/gemini/pkg/kube"
	v1 "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1"
)

func createPVC(sg *v1.SnapshotGroup, spec corev1.PersistentVolumeClaimSpec, annotations map[string]string) error {
	klog.Infof("%s/%s: creating PVC", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
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
	_, err := pvcClient.Create(pvc)
	return err
}

func maybeCreatePVC(sg *v1.SnapshotGroup) error {
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("%s/%s: PVC not found, creating it", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
		err := createPVC(sg, sg.Spec.Claim.Spec, nil)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("%s/%s: PVC found", pvc.ObjectMeta.Namespace, pvc.ObjectMeta.Name)
		if pvc.ObjectMeta.Annotations[managedByAnnotation] != managerName {
			return fmt.Errorf("%s/%s: PVC found, but not managed by Gemini", pvc.ObjectMeta.Namespace, pvc.ObjectMeta.Name)
		}
	}
	return nil
}

func restorePVC(sg *v1.SnapshotGroup) error {
	klog.Infof("%s/%s: restoring PVC", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	restorePoint := sg.ObjectMeta.Annotations[RestoreAnnotation]
	annotations := map[string]string{
		RestoreAnnotation: restorePoint,
	}
	spec := sg.Spec.Claim.Spec
	apiGroup := kube.VolumeSnapshotGroupName
	spec.DataSource = &corev1.TypedLocalObjectReference{
		APIGroup: &apiGroup,
		Kind:     kube.VolumeSnapshotKind,
		Name:     sg.ObjectMeta.Name + "-" + restorePoint,
	}
	return createPVC(sg, spec, annotations)
}

func deletePVC(sg *v1.SnapshotGroup) error {
	klog.Infof("%s/%s: deleting PVC", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	return pvcClient.Delete(sg.ObjectMeta.Name, &metav1.DeleteOptions{})
}
