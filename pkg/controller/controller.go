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

package controller

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/meta"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/fairwindsops/gemini/pkg/kube"
	"github.com/fairwindsops/gemini/pkg/snapshots"
	snapshotgroup "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1"
	listers "github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis/listers/snapshotgroup/v1beta1"
)

const defaultSnapshotReadyTimeoutSeconds = 60

// Controller represents a SnapshotGroup controller
type Controller struct {
	sgLister listers.SnapshotGroupLister
	sgSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	snapshotReadyTimeoutSeconds int
}

type task int

const (
	backupTask task = iota
	restoreTask
	deleteTask
)

var taskLabels = []string{"backup", "restore", "delete"}

type workItem struct {
	name          string
	namespace     string
	snapshotGroup *snapshotgroup.SnapshotGroup
	task          task
}

func getRateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(time.Second, 1000*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
}

// NewController creates a new SnapshotGroup controller
func NewController() *Controller {
	client := kube.GetClient()
	controller := &Controller{
		sgLister:                    client.Informer.Lister(),
		sgSynced:                    client.Informer.Informer().HasSynced,
		workqueue:                   workqueue.NewNamedRateLimitingQueue(getRateLimiter(), "SnapshotGroups"),
		snapshotReadyTimeoutSeconds: defaultSnapshotReadyTimeoutSeconds,
	}
	client.Informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(sg interface{}) {
			controller.enqueue(sg, backupTask)
		},
		UpdateFunc: func(old, sg interface{}) {
			oldAcc, _ := meta.Accessor(old)
			newAcc, _ := meta.Accessor(sg)
			oldRestore := oldAcc.GetAnnotations()[snapshots.RestoreAnnotation]
			newRestore := newAcc.GetAnnotations()[snapshots.RestoreAnnotation]
			if newRestore != "" && oldRestore != newRestore {
				controller.enqueue(sg, restoreTask)
			} else {
				controller.enqueue(sg, backupTask)
			}
		},
		DeleteFunc: func(sg interface{}) {
			controller.enqueue(sg, deleteTask)
		},
	})
	return controller
}

func (c *Controller) enqueue(sg interface{}, todo task) {
	acc, _ := meta.Accessor(sg)
	name := acc.GetName()
	namespace := acc.GetNamespace()
	w := workItem{
		name:          name,
		namespace:     namespace,
		task:          todo,
		snapshotGroup: sg.(*snapshotgroup.SnapshotGroup),
	}
	c.workqueue.Add(w)
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var item workItem
		var ok bool
		if item, ok = obj.(workItem); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected workItem in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(item); err != nil {
			c.workqueue.AddRateLimited(item)
			return fmt.Errorf("%s/%s: error syncing %#v: %s, requeuing", item.namespace, item.name, item, err.Error())
		}
		c.workqueue.Forget(obj)
		klog.Infof("%s/%s: successfully performed %s", item.namespace, item.name, taskLabels[item.task])
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(w workItem) error {
	var err error
	if w.task == backupTask {
		err = snapshots.ReconcileBackupsForSnapshotGroup(w.snapshotGroup)
	} else if w.task == restoreTask {
		err = snapshots.RestoreSnapshotGroup(w.snapshotGroup, c.snapshotReadyTimeoutSeconds)
	} else if w.task == deleteTask {
		err = snapshots.OnSnapshotGroupDelete(w.snapshotGroup)
	}

	if err != nil {
		klog.Errorf("%s/%s: failed to perform %s - %v", w.namespace, w.name, taskLabels[w.task], err)
		return err
	}

	return nil
}

// Run starts the controller
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting SnapshotGroup controller")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.sgSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}
