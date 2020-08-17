package main

import (
	"flag"

	"k8s.io/klog"

	"github.com/fairwindsops/gemini/pkg/controller"
	"github.com/fairwindsops/gemini/pkg/kube"
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
}

func main() {
	klog.V(5).Infof("Running in verbose mode")
	ctrl := controller.NewController()

	stopCh := make(chan struct{})
	kube.GetClient().InformerFactory.Start(stopCh)
	if err := ctrl.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
