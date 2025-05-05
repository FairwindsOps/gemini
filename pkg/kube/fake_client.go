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

package kube

import (
	"time"

	snapshotsFake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	snapshotGroupsFake "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/clientset/versioned/fake"
	snapshotGroupExternalVersions "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/informers/externalversions"
)

var noResync = func() time.Duration { return 0 }

// SetFakeClient sets the singleton to a dummy client
func SetFakeClient() *Client {
	singleton = createFakeClient()
	return singleton
}

func createFakeClient() *Client {
	var objects []k8sruntime.Object
	k8s := k8sfake.NewSimpleClientset(objects...)
	_ = snapshotsFake.NewSimpleClientset(objects...)

	snapshotGroupClientSet := snapshotGroupsFake.NewSimpleClientset(objects...)
	informerFactory := snapshotGroupExternalVersions.NewSharedInformerFactory(snapshotGroupClientSet, noResync())
	informer := informerFactory.Snapshotgroup().V1beta1().SnapshotGroups()

	volumeSnapshotVersionResource := schema.GroupVersionResource{
		Group:    VolumeSnapshotGroupName,
		Version:  "v1beta1",
		Resource: VolumeSnapshotKind,
	}
	dynamic := dynamicFake.NewSimpleDynamicClientWithCustomListKinds(k8sruntime.NewScheme(), map[schema.GroupVersionResource]string{
		volumeSnapshotVersionResource: "VolumeSnapshotList",
	})
	snapshotClient := dynamic.Resource(volumeSnapshotVersionResource)

	return &Client{
		K8s:                 k8s,
		Informer:            informer,
		InformerFactory:     informerFactory,
		SnapshotClient:      snapshotClient,
		SnapshotGroupClient: snapshotGroupClientSet.SnapshotgroupV1beta1(),
	}
}
