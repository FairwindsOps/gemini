package snapshots

import (
	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func AddOrUpdateSnapshotGroup(sg *v1.SnapshotGroup) error {
	klog.Infof("Reconcile SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	err := maybeCreatePVC(sg)
	if err != nil {
		return err
	}
	return nil
}

func maybeCreatePVC(sg *v1.SnapshotGroup) error {
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("pvc %s not found, creating it", pvc.ObjectMeta.Name)
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sg.ObjectMeta.Name,
				Namespace: sg.ObjectMeta.Namespace,
			},
			Spec: sg.Spec.Claim.Spec,
		}
		_, err := pvcClient.Create(pvc)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("Found pvc %s", pvc.ObjectMeta.Name)
	}
	return nil
}
