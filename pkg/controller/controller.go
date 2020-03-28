package controller

import (
	"fmt"
	"time"

	clientset "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/clientset/versioned"
	informers "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/informers/externalversions/snapshotgroup/v1"
	listers "github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis/listers/snapshotgroup/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type Controller struct {
	kubeclientset   kubernetes.Interface
	sampleclientset clientset.Interface

	sgLister listers.SnapshotGroupLister
	sgSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
}

func NewController(kubeclientset kubernetes.Interface, sampleclientset clientset.Interface, sgInformer informers.SnapshotGroupInformer) *Controller {
	controller := &Controller{
		kubeclientset:   kubeclientset,
		sampleclientset: sampleclientset,
		sgLister:        sgInformer.Lister(),
		sgSynced:        sgInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
	}
	sgInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(sg interface{}) {
			fmt.Println("ADD")
		},
		UpdateFunc: func(old, new interface{}) {
			fmt.Println("UPDATE")
		},
	})
	return controller
}

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
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
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	sg, err := c.sgLister.SnapshotGroups(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("SnapshotGroup '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	klog.Infof("Found sg %v\n", sg)
	return nil
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	fmt.Println("Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	fmt.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.sgSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	fmt.Println("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	fmt.Println("Started workers")
	<-stopCh
	fmt.Println("Shutting down workers")

	return nil
}
