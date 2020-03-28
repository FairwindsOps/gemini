package main

import (
	"time"

	"github.com/fairwindsops/photon/pkg/controller"
	snapshotgroupv1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
	clientset "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/clientset/versioned"
	informers "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
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
	if _, err = snapshotgroupv1.CreateCustomResourceDefinition("crd-ns", extClientSet); err != nil {
		panic(err)
	}
	informerFactory := informers.NewSharedInformerFactory(clientSet, time.Second*30)
	informer := informerFactory.Snapshotgroup().V1().SnapshotGroups()
	ctrl := controller.NewController(k8s, clientSet, informer)

	stopCh := signals.SetupSignalHandler()
	informerFactory.Start(stopCh)
	if err = ctrl.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
