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

func maybeCreatePVC(sg *snapshotgroup.SnapshotGroup) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := getPVC(sg)
	if err == nil {
		klog.Infof("%s/%s: PVC found", pvc.ObjectMeta.Namespace, pvc.ObjectMeta.Name)
		return pvc, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}
	if sg.Spec.Claim.Name != "" {
		return nil, fmt.Errorf("%s/%s: could not find existing PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, sg.Spec.Claim.Name)
	}
	klog.Infof("%s/%s: PVC not found, creating it", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	return createPVC(sg, sg.Spec.Claim.Spec, nil)
}

func createPVC(sg *snapshotgroup.SnapshotGroup, spec corev1.PersistentVolumeClaimSpec, annotations map[string]string) (*corev1.PersistentVolumeClaim, error) {
	name := getPVCName(sg)
	klog.Infof("%s/%s: creating PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, name)
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[managedByAnnotation] = managerName
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   sg.ObjectMeta.Namespace,
			Annotations: annotations,
		},
		Spec: spec,
	}
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	return pvcClient.Create(pvc)
}

func restorePVC(sg *snapshotgroup.SnapshotGroup) error {
	klog.Infof("%s/%s: restoring PVC", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	err := deletePVC(sg)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

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
	_, err = createPVC(sg, spec, annotations)
	return err
}

func deletePVC(sg *snapshotgroup.SnapshotGroup) error {
	name := getPVCName(sg)
	klog.Infof("%s/%s: deleting PVC %s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name, name)
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	return pvcClient.Delete(name, &metav1.DeleteOptions{})
}
