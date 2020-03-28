package main

import (
	snapshotgroupv1 "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	kubeConf, configError := config.GetConfig()
	if configError != nil {
		panic(configError)
	}
	apiextensionsClientSet, err := apiextensionsclient.NewForConfig(kubeConf)
	if err != nil {
		panic(err)
	}
	if _, err = snapshotgroupv1.CreateCustomResourceDefinition("crd-ns", apiextensionsClientSet); err != nil {
		panic(err)
	}
}
