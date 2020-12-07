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

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	// Import known auth providers
	//apimeta "k8s.io/apimachinery/pkg/api/meta"
	//"k8s.io/client-go/discovery"
	//discocache "k8s.io/client-go/discovery/cached"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	snapshotgroupv1 "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
	snapshotGroupClientset "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/clientset/versioned"
	snapshotgroupInterface "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/clientset/versioned/typed/snapshotgroup/v1beta1"
	"github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/informers/externalversions"
	informers "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/informers/externalversions/snapshotgroup/v1beta1"
)

const (
	// VolumeSnapshotGroupName is the group name for the VolumeSnapshot CRD
	VolumeSnapshotGroupName = "snapshot.storage.k8s.io"
	// VolumeSnapshotKind is the kind for VolumeSnapshots
	VolumeSnapshotKind = "VolumeSnapshot"
)

// Client provides access to k8s resources
type Client struct {
	K8s                   kubernetes.Interface
	Informer              informers.SnapshotGroupInformer
	InformerFactory       externalversions.SharedInformerFactory
	SnapshotClient        dynamic.NamespaceableResourceInterface
	SnapshotGroupClient   snapshotgroupInterface.SnapshotgroupV1beta1Interface
	VolumeSnapshotVersion string
	ScaleClient           scale.ScalesGetter
}

var singleton *Client

// GetClient creates a new Client singleton
func GetClient() *Client {
	if singleton == nil {
		singleton = createClient()
	}
	return singleton
}

func createClient() *Client {
	kubeConf, configError := config.GetConfig()
	if configError != nil {
		panic(configError)
	}
	k8s, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}
	extClientSet, err := apiextensionsclient.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}
	sgClientSet, err := snapshotGroupClientset.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}

	informerFactory := externalversions.NewSharedInformerFactory(sgClientSet, time.Second*30)
	informer := informerFactory.Snapshotgroup().V1beta1().SnapshotGroups()

	resources, err := restmapper.GetAPIGroupResources(k8s.Discovery())
	if err != nil {
		panic(err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(resources)

	/*
		cachedDiscovery := discocache.NewMemCacheClient(k8s.Discovery())
		restMapper := discovery.NewDeferredDiscoveryRESTMapper(
			cachedDiscovery,
			apimeta.InterfacesForUnstructured,
		)
	*/
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(
		k8s.Discovery(),
	)
	scaleClient, err := scale.NewForConfig(
		kubeConf, restMapper,
		dynamic.LegacyAPIPathResolverFunc,
		scaleKindResolver,
	)
	if err != nil {
		panic(err)
	}

	snapshotCRD, err := extClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Get("volumesnapshots."+VolumeSnapshotGroupName, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	vsMapping, err := restMapper.RESTMapping(schema.GroupKind{
		Group: VolumeSnapshotGroupName,
		Kind:  VolumeSnapshotKind,
	})
	if err != nil {
		panic(err)
	}
	dynamicInterface, err := dynamic.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}
	snapshotClient := dynamicInterface.Resource(vsMapping.Resource)

	if _, err = snapshotgroupv1.CreateCustomResourceDefinition("crd-ns", extClientSet); err != nil {
		panic(err)
	}
	return &Client{
		K8s:                   k8s,
		Informer:              informer,
		InformerFactory:       informerFactory,
		SnapshotClient:        snapshotClient,
		SnapshotGroupClient:   sgClientSet.SnapshotgroupV1beta1(),
		ScaleClient:           scaleClient,
		VolumeSnapshotVersion: VolumeSnapshotGroupName + "/" + snapshotCRD.Spec.Version,
	}
}
