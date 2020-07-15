package snapshots

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/fairwindsops/gemini/pkg/kube"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
)

func getPVCName(sg *snapshotgroup.SnapshotGroup) string {
	name := sg.Spec.Claim.Name
	if name == "" {
		name = sg.ObjectMeta.Name
	}
	return name
}

func getPVC(sg *snapshotgroup.SnapshotGroup) (*corev1.PersistentVolumeClaim, error) {
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(getPVCName(sg), metav1.GetOptions{})
	return pvc, err
}

func maybeCreatePVC(sg *snapshotgroup.SnapshotGroup) error {
	pvc, err := getPVC(sg)
	if err == nil {
		klog.Infof("%s/%s: PVC found", pvc.ObjectMeta.Namespace, pvc.ObjectMeta.Name)
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	if sg.Spec.Claim.Name != "" {
		return fmt.Errorf("%s/%s: could not find existing PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, sg.Spec.Claim.Name)
	}
	klog.Infof("%s/%s: PVC not found, creating it", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	return createPVC(sg, sg.Spec.Claim.Spec, nil)
}

func createPVC(sg *snapshotgroup.SnapshotGroup, spec corev1.PersistentVolumeClaimSpec, annotations map[string]string) error {
	klog.Infof("%s/%s: creating PVC", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[managedByAnnotation] = managerName
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        getPVCName(sg),
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

func restorePVC(sg *snapshotgroup.SnapshotGroup) error {
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

func deletePVC(sg *snapshotgroup.SnapshotGroup) error {
	name := getPVCName(sg)
	klog.Infof("%s/%s: deleting PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, name)
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	return pvcClient.Delete(name, &metav1.DeleteOptions{})
}
