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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	snapshotGroupsFake "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1/apis/clientset/versioned/fake"
	snapshotGroupExternalVersions "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1/apis/informers/externalversions"
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
	informer := informerFactory.Snapshotgroup().V1().SnapshotGroups()

	volumeSnapshotVersionResource := schema.GroupVersionResource{
		Group:    VolumeSnapshotGroupName,
		Version:  "v1",
		Resource: VolumeSnapshotKind,
	}
	volumeSnapshotContentResource := schema.GroupVersionResource{
		Group:    VolumeSnapshotGroupName,
		Version:  "v1",
		Resource: "volumesnapshotcontents",
	}
	dynClient := dynamicFake.NewSimpleDynamicClientWithCustomListKinds(k8sruntime.NewScheme(), map[schema.GroupVersionResource]string{
		volumeSnapshotVersionResource: "VolumeSnapshotList",
		volumeSnapshotContentResource: "VolumeSnapshotContentList",
	})

	// Reactor: auto-populate status on VolumeSnapshot creation and create a matching
	// VolumeSnapshotContent so the blue-green swap can read content details in tests.
	dynClient.PrependReactor("create", "VolumeSnapshot", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		obj := createAction.GetObject().(*unstructured.Unstructured)
		contentName := "snapcontent-" + obj.GetName()
		readyToUse := true
		unstructured.SetNestedField(obj.Object, readyToUse, "status", "readyToUse")
		unstructured.SetNestedField(obj.Object, contentName, "status", "boundVolumeSnapshotContentName")

		// Create a corresponding VolumeSnapshotContent object via the tracker
		// to avoid deadlocking on the fake client's mutex.
		vsc := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": VolumeSnapshotGroupName + "/v1",
				"kind":       "VolumeSnapshotContentList",
				"metadata": map[string]interface{}{
					"name": contentName,
				},
				"spec": map[string]interface{}{
					"deletionPolicy":          "Delete",
					"driver":                  "fake.csi.driver",
					"volumeSnapshotClassName": "fake-snapshot-class",
					"source": map[string]interface{}{
						"volumeHandle": "fake-volume-handle-" + obj.GetName(),
					},
					"volumeSnapshotRef": map[string]interface{}{
						"name":      obj.GetName(),
						"namespace": obj.GetNamespace(),
					},
				},
				"status": map[string]interface{}{
					"snapshotHandle": "fake-snap-handle-" + obj.GetName(),
					"readyToUse":     true,
				},
			},
		}
		dynClient.Tracker().Create(volumeSnapshotContentResource, vsc, "")

		return false, obj, nil
	})

	snapshotClient := dynClient.Resource(volumeSnapshotVersionResource)
	snapshotContentClient := dynClient.Resource(volumeSnapshotContentResource)

	return &Client{
		K8s:                   k8s,
		Informer:              informer,
		InformerFactory:       informerFactory,
		SnapshotClient:        snapshotClient,
		SnapshotContentClient: snapshotContentClient,
		SnapshotGroupClient:   snapshotGroupClientSet.SnapshotgroupV1(),
	}
}
