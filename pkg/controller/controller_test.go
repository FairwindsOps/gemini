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

package controller

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/gemini/pkg/kube"
	"github.com/fairwindsops/gemini/pkg/snapshots"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
)

func newSnapshotGroup(name, namespace string) *snapshotgroup.SnapshotGroup {
	return &snapshotgroup.SnapshotGroup{
		TypeMeta: metav1.TypeMeta{APIVersion: snapshotgroup.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		Spec: snapshotgroup.SnapshotGroupSpec{
			Schedule: []snapshotgroup.SnapshotSchedule{
				{
					Every: "1 second",
					Keep:  1,
				},
			},
		},
	}
}

func newTestController() (*Controller, *kube.Client) {
	kube.SetFakeClient()
	ctrl := NewController()
	ctrl.snapshotReadyTimeoutSeconds = 1
	return ctrl, kube.GetClient()
}

func TestControllerQueue(t *testing.T) {
	ctrl, _ := newTestController()
	sg := newSnapshotGroup("foo", "default")
	ctrl.enqueue(sg, deleteTask)
	processed := ctrl.processNextWorkItem()
	assert.Equal(t, true, processed)
}

func TestBackupHandler(t *testing.T) {
	ctrl, client := newTestController()

	sg := newSnapshotGroup("foo", "foo")
	snaps, err := snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(snaps))

	_, err = client.SnapshotGroupClient.SnapshotGroups("foo").Create(context.Background(), sg, metav1.CreateOptions{})
	assert.NoError(t, err)

	event := workItem{
		name:          "foo",
		namespace:     "foo",
		snapshotGroup: sg,
		task:          backupTask,
	}
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[0].Intervals)

	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(context.TODO(), sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "gemini", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])

	time.Sleep(time.Second)
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[0].Intervals)
	assert.Equal(t, []string{"1 second"}, snaps[1].Intervals)

	firstTS := snaps[1].Timestamp
	secondTS := snaps[0].Timestamp

	time.Sleep(time.Second)
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[0].Intervals)
	assert.Equal(t, []string{"1 second"}, snaps[1].Intervals)
	assert.Equal(t, secondTS, snaps[1].Timestamp)
	assert.NotEqual(t, firstTS, snaps[0].Timestamp)
}

func TestRestoreHandler(t *testing.T) {
	ctrl, client := newTestController()
	sgName := "foo"
	sgNamespace := "default"
	sg := newSnapshotGroup(sgName, sgNamespace)
	snaps, err := snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(snaps))

	_, err = client.SnapshotGroupClient.SnapshotGroups(sgNamespace).Create(context.Background(), sg, metav1.CreateOptions{})
	assert.NoError(t, err)

	event := workItem{
		name:          sgName,
		namespace:     sgNamespace,
		snapshotGroup: sg,
		task:          backupTask,
	}
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[0].Intervals)

	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(context.TODO(), sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "gemini", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, "", pvc.ObjectMeta.Annotations["gemini.fairwinds.com/restore"])

	time.Sleep(time.Second)
	timestamp := strconv.Itoa(int(snaps[0].Timestamp.Unix()))
	sg.ObjectMeta.Annotations["gemini.fairwinds.com/restore"] = timestamp
	event.task = restoreTask
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	pvc, err = pvcClient.Get(context.TODO(), sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "gemini", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, timestamp, pvc.ObjectMeta.Annotations["gemini.fairwinds.com/restore"])

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[1].Intervals)
	assert.Equal(t, []string{}, snaps[0].Intervals)
}

func TestDeleteHandler(t *testing.T) {
	ctrl, _ := newTestController()

	event := workItem{
		name:          "foo",
		namespace:     "foo",
		snapshotGroup: newSnapshotGroup("foo", "default"),
		task:          deleteTask,
	}
	err := ctrl.syncHandler(event)
	assert.NoError(t, err)
}

func TestPreexistingPVC(t *testing.T) {
	ctrl, client := newTestController()

	namespace := "default"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-existing",
			Namespace: namespace,
			Annotations: map[string]string{
				"app.kubernetes.io/managed-by": "me",
			},
		},
	}
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(namespace)
	_, err := pvcClient.Create(context.TODO(), pvc, metav1.CreateOptions{})
	assert.NoError(t, err)

	pvcs, err := pvcClient.List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pvcs.Items))
	existingPVC := pvcs.Items[0]
	assert.Equal(t, "pre-existing", existingPVC.ObjectMeta.Name)
	assert.Equal(t, "me", existingPVC.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, "", existingPVC.ObjectMeta.Annotations["gemini.fairwinds.com/restore"])

	sg := newSnapshotGroup("foo", namespace)
	sg.Spec.Claim.Name = "pre-existing"
	snaps, err := snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(snaps))

	_, err = client.SnapshotGroupClient.SnapshotGroups(namespace).Create(context.Background(), sg, metav1.CreateOptions{})
	assert.NoError(t, err)

	event := workItem{
		name:          "foo",
		namespace:     namespace,
		snapshotGroup: sg,
		task:          backupTask,
	}
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[0].Intervals)

	pvcs, err = pvcClient.List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pvcs.Items))
	existingPVC = pvcs.Items[0]
	assert.Equal(t, "pre-existing", existingPVC.ObjectMeta.Name)
	assert.Equal(t, "me", existingPVC.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, "", existingPVC.ObjectMeta.Annotations["gemini.fairwinds.com/restore"])

	time.Sleep(time.Second)
	timestamp := strconv.Itoa(int(snaps[0].Timestamp.Unix()))
	sg.ObjectMeta.Annotations["gemini.fairwinds.com/restore"] = timestamp
	event.task = restoreTask
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	pvcs, err = pvcClient.List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pvcs.Items))
	newPVC := pvcs.Items[0]
	assert.Equal(t, "pre-existing", newPVC.ObjectMeta.Name)
	assert.Equal(t, "gemini", newPVC.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, timestamp, newPVC.ObjectMeta.Annotations["gemini.fairwinds.com/restore"])
}
