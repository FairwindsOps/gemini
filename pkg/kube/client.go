package kube

import (
	"time"

	snapshotgroupv1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
	clientset "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/clientset/versioned"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions"
	informers "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions/snapshotgroup/v1"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Client struct {
	K8s             kubernetes.Interface
	ClientSet       clientset.Interface
	Informer        informers.SnapshotGroupInformer
	InformerFactory externalversions.SharedInformerFactory
}

var singleton *Client

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
	clientSet, err := clientset.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}

	informerFactory := externalversions.NewSharedInformerFactory(clientSet, time.Second*30)
	informer := informerFactory.Snapshotgroup().V1().SnapshotGroups()

	if _, err = snapshotgroupv1.CreateCustomResourceDefinition("crd-ns", extClientSet); err != nil {
		panic(err)
	}
	return &Client{
		K8s:             k8s,
		ClientSet:       clientSet,
		Informer:        informer,
		InformerFactory: informerFactory,
	}
}
