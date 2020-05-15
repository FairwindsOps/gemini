package kube

import (
	"time"

	snapshotsFake "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	snapshotGroupsFake "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/clientset/versioned/fake"
	snapshotGroupExternalVersions "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions"
)

var noResync = func() time.Duration { return 0 }

// SetFakeClient sets the singleton to a dummy client
func SetFakeClient() *Client {
	singleton = createFakeClient()
	return singleton
}

func createFakeClient() *Client {
	objects := []k8sruntime.Object{}
	k8s := k8sfake.NewSimpleClientset(objects...)
	_ = snapshotsFake.NewSimpleClientset(objects...)

	snapshotGroupClientSet := snapshotGroupsFake.NewSimpleClientset(objects...)
	informerFactory := snapshotGroupExternalVersions.NewSharedInformerFactory(snapshotGroupClientSet, noResync())
	informer := informerFactory.Snapshotgroup().V1().SnapshotGroups()

	dynamic := dynamicFake.NewSimpleDynamicClient(k8sruntime.NewScheme())
	snapshotClient := dynamic.Resource(schema.GroupVersionResource{VolumeSnapshotGroupName, "v1beta1", VolumeSnapshotKind})

	return &Client{
		K8s:             k8s,
		ClientSet:       snapshotGroupClientSet,
		Informer:        informer,
		InformerFactory: informerFactory,
		SnapshotClient:  snapshotClient,
	}
}
