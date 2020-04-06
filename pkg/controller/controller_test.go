package controller

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/snapshots"
	v1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

func newSnapshotGroup(name string) *v1.SnapshotGroup {
	return &v1.SnapshotGroup{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   metav1.NamespaceDefault,
			Annotations: map[string]string{},
		},
		Spec: v1.SnapshotGroupSpec{
			Schedule: []v1.SnapshotSchedule{
				v1.SnapshotSchedule{
					Every: "1 second",
					Keep:  1,
				},
			},
		},
	}
}

func newController() *Controller {
	kube.SetFakeClient()
	return NewController()
}

func TestBackupHandler(t *testing.T) {
	ctrl := newController()
	sg := newSnapshotGroup("foo")
	snaps, err := snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(snaps))

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

	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "photon", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])

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
	ctrl := newController()
	sg := newSnapshotGroup("foo")
	snaps, err := snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(snaps))

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

	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "photon", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, "", pvc.ObjectMeta.Annotations["photon.fairwinds.com/restore"])

	time.Sleep(time.Second)
	timestamp := strconv.Itoa(int(snaps[0].Timestamp.Unix()))
	sg.ObjectMeta.Annotations["photon.fairwinds.com/restore"] = timestamp
	event.task = restoreTask
	err = ctrl.syncHandler(event)
	assert.NoError(t, err)

	pvc, err = pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "photon", pvc.ObjectMeta.Annotations["app.kubernetes.io/managed-by"])
	assert.Equal(t, timestamp, pvc.ObjectMeta.Annotations["photon.fairwinds.com/restore"])

	snaps, err = snapshots.ListSnapshots(sg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(snaps))
	assert.Equal(t, []string{"1 second"}, snaps[1].Intervals)
	assert.Equal(t, []string{}, snaps[0].Intervals)
}

func TestDeleteHandler(t *testing.T) {
	ctrl := newController()

	event := workItem{
		name:          "foo",
		namespace:     "foo",
		snapshotGroup: newSnapshotGroup("foo"),
		task:          deleteTask,
	}
	err := ctrl.syncHandler(event)
	assert.NoError(t, err)
}
