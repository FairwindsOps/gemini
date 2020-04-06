package kube

import (
	"time"

	snapshotsFake "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	//apiextensionsFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/versioned/fake"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	//snapshotgroupv1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
	snapshotGroupsFake "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/clientset/versioned/fake"
	snapshotGroupExternalVersions "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions"
)

var noResync = func() time.Duration { return 0 }

func SetFakeClient() {
	singleton = createFakeClient()
}

func createFakeClient() *Client {
	objects := []runtime.Object{}
	k8s := k8sfake.NewSimpleClientset(objects...)
	//extClientSet := apiextensionsFake.NewSimpleClientSet(objects...)
	snapshotClientSet := snapshotsFake.NewSimpleClientset(objects...)
	snapshotGroupClientSet := snapshotGroupsFake.NewSimpleClientset(objects...)
	/*
		if _, err := snapshotgroupv1.CreateCustomResourceDefinition("crd-ns", extClientSet); err != nil {
			panic(err)
		}
	*/
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
