package kube

import (
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	// Import known auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/restmapper"
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
		VolumeSnapshotVersion: VolumeSnapshotGroupName + "/" + snapshotCRD.Spec.Version,
	}
}
