package main

import (
	"context"
	"flag"

	"k8s.io/klog/v2"

	"github.com/fairwindsops/gemini/pkg/controller"
	"github.com/fairwindsops/gemini/pkg/fsr"
	"github.com/fairwindsops/gemini/pkg/kube"
	"github.com/fairwindsops/gemini/pkg/snapshots"
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
}

func main() {
	klog.V(5).Infof("Running in verbose mode")

	// Cluster-wide FSR kill-switch. When disabled we skip AWS client init too,
	// so operators can turn the feature off on clusters that have no AWS creds
	// wired to the pod without seeing a spurious warning every restart.
	if !fsr.EnabledFromEnv() {
		snapshots.SetFSRGlobalEnabled(false)
		klog.V(2).Infof("FSR: disabled globally via %s=false; ReconcileFSR will no-op for all SnapshotGroups", fsr.EnabledEnvVar)
	} else {
		// Initialize the AWS Fast Snapshot Restore client. SnapshotGroups that opt
		// in via spec.fastSnapshotRestore.enabled use this; others ignore it.
		// We log-and-continue on init failure so a missing AWS environment does
		// not break clusters that don't use FSR.
		if c, err := fsr.NewAWSClient(context.Background()); err != nil {
			klog.Warningf("FSR: AWS client init failed (%v); SnapshotGroups with fastSnapshotRestore.enabled=true will no-op", err)
		} else {
			snapshots.SetFSRClient(c)
		}
		if azs := fsr.DefaultAZsFromEnv(); len(azs) > 0 {
			snapshots.SetDefaultFSRAZs(azs)
			klog.V(2).Infof("FSR: default AZs from %s = %v", fsr.DefaultAZsEnvVar, azs)
		}
	}

	ctrl := controller.NewController()

	stopCh := make(chan struct{})
	kube.GetClient().InformerFactory.Start(stopCh)
	if err := ctrl.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
