package controller

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/snapshots"
	v1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	sgLister []*v1.SnapshotGroup
	// Actions expected to happen on the client.
	actions []core.Action
	// Objects from here preloaded into NewSimpleFake.
	objects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	return f
}

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

func (f *fixture) newController() *Controller {
	kube.SetFakeClient()
	client := kube.GetClient()
	c := NewController()

	/*
		c.foosSynced = alwaysReady
		c.deploymentsSynced = alwaysReady
		c.recorder = &record.FakeRecorder{}
	*/

	for _, sg := range f.sgLister {
		client.Informer.Informer().GetIndexer().Add(sg)
	}

	return c
}

func (f *fixture) run(fooName string) {
	f.runController(fooName, true, false)
}

func (f *fixture) runExpectError(fooName string) {
	f.runController(fooName, true, true)
}

func (f *fixture) runController(fooName string, startInformers bool, expectError bool) {
	ctrl := f.newController()
	client := kube.GetClient()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		client.InformerFactory.Start(stopCh)
	}
	item := workItem{
		name:          "thing",
		namespace:     "foo",
		snapshotGroup: &v1.SnapshotGroup{},
		task:          backupTask,
	}

	err := ctrl.syncHandler(item)
	if !expectError && err != nil {
		f.t.Errorf("error syncing item: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing item, got nil")
	}

	/*
		actions := filterInformerActions(client.ClientSet.Actions())
		for i, action := range actions {
			if len(f.actions) < i+1 {
				f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
				break
			}

			expectedAction := f.actions[i]
			checkAction(expectedAction, action, f.t)
		}

		if len(f.actions) > len(actions) {
			f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
		}
	*/
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "foos") ||
				action.Matches("watch", "foos") ||
				action.Matches("list", "deployments") ||
				action.Matches("watch", "deployments")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectUpdateSnapshotGroupStatusAction(foo *v1.SnapshotGroup) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Resource: "foos"}, foo.Namespace, foo)
	// TODO: Until #38113 is merged, we can't use Subresource
	//action.Subresource = "status"
	f.actions = append(f.actions, action)
}

func getKey(foo *v1.SnapshotGroup, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(foo)
	if err != nil {
		t.Errorf("Unexpected error getting key for foo %v: %v", foo.Name, err)
		return ""
	}
	return key
}

func TestDoNothing(t *testing.T) {
	f := newFixture(t)
	foo := newSnapshotGroup("test")

	f.objects = append(f.objects, foo)

	f.expectUpdateSnapshotGroupStatusAction(foo)
	f.run(getKey(foo, t))
}

func TestBackupHandler(t *testing.T) {
	f := newFixture(t)
	ctrl := f.newController()
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
	f := newFixture(t)
	ctrl := f.newController()
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
	f := newFixture(t)
	ctrl := f.newController()

	event := workItem{
		name:          "foo",
		namespace:     "foo",
		snapshotGroup: newSnapshotGroup("foo"),
		task:          deleteTask,
	}
	err := ctrl.syncHandler(event)
	assert.NoError(t, err)
}
