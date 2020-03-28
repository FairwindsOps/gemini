package main

import (
	"github.com/fairwindsops/photon/pkg/controller"
	"github.com/fairwindsops/photon/pkg/kube"

	"k8s.io/klog"
	"k8s.io/sample-controller/pkg/signals"
)

func main() {
	ctrl := controller.NewController()

	stopCh := signals.SetupSignalHandler()
	kube.GetClient().InformerFactory.Start(stopCh)
	if err := ctrl.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
