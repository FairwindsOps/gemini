package kube

import (
	"time"

	snapshotsFake "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	apiextensionsFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/runtime"
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
	objects := []runtime.Object{}
	k8s := k8sfake.NewSimpleClientset(objects...)
	_ = apiextensionsFake.NewSimpleClientset(objects...)
	snapshotClientSet := snapshotsFake.NewSimpleClientset(objects...)
	snapshotGroupClientSet := snapshotGroupsFake.NewSimpleClientset(objects...)
	informerFactory := snapshotGroupExternalVersions.NewSharedInformerFactory(snapshotGroupClientSet, noResync())
	informer := informerFactory.Snapshotgroup().V1().SnapshotGroups()
	return &Client{
		K8s:             k8s,
		ClientSet:       snapshotGroupClientSet,
		Informer:        informer,
		InformerFactory: informerFactory,
		SnapshotClient:  snapshotClientSet,
	}
}
