/*
 * [2013] - [2018] Avi Networks Incorporated
 * All Rights Reserved.
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*   http://www.apache.org/licenses/LICENSE-2.0
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*/

package k8s

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/avinetworks/servicemesh/pkg/queue"
	"github.com/avinetworks/servicemesh/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var controllerInstance *AviController
var ctrlonce sync.Once

type AviController struct {
	worker_id       uint32
	worker_id_mutex sync.Mutex
	//recorder        record.EventRecorder
	informers *utils.Informers
	workqueue []workqueue.RateLimitingInterface
}

func SharedAviController(inf *utils.Informers) *AviController {
	ctrlonce.Do(func() {
		controllerInstance = &AviController{
			worker_id: (uint32(1) << utils.NumWorkers) - 1,
			//recorder:  recorder,
			informers: inf,
		}
	})
	return controllerInstance
}

func ObjKey(obj interface{}) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utils.AviLog.Warning.Print(err)
	}

	return key
}

func NewInformers(cs *kubernetes.Clientset) *utils.Informers {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(cs, time.Second*30)
	informers := utils.Informers{
		ServiceInformer: kubeInformerFactory.Core().V1().Services(),
		EpInformer:      kubeInformerFactory.Core().V1().Endpoints(),
	}
	return &informers
}

func (c *AviController) SetupEventHandlers(cs *kubernetes.Clientset) {
	utils.AviLog.Info.Printf("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(utils.AviLog.Info.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: cs.CoreV1().Events("")})
	//recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "avi-k8s-controller"})

	mcpQueue := queue.SharedWorkQueueWrappers().GetQueueByName(queue.ObjectIngestionLayer)
	c.workqueue = mcpQueue.Workqueue
	numWorkers := mcpQueue.NumWorkers

	ep_event_handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)
			namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(ep))
			key := "Endpoints/" + ObjKey(ep)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
			utils.AviLog.Info.Printf("Added ADD Endpoint key from the kubernetes controller %s", key)
		},
		DeleteFunc: func(obj interface{}) {
			ep, ok := obj.(*corev1.Endpoints)
			if !ok {
				// endpoints was deleted but its final state is unrecorded.
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Error.Printf("couldn't get object from tombstone %#v", obj)
					return
				}
				ep, ok = tombstone.Obj.(*corev1.Endpoints)
				if !ok {
					utils.AviLog.Error.Printf("Tombstone contained object that is not an Endpoints: %#v", obj)
					return
				}
			}
			ep = obj.(*corev1.Endpoints)
			namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(ep))
			key := "Endpoints/" + ObjKey(ep)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
			utils.AviLog.Info.Printf("Added DELETE Endpoint key from the kubernetes controller %s", key)
		},
		UpdateFunc: func(old, cur interface{}) {
			oep := old.(*corev1.Endpoints)
			cep := cur.(*corev1.Endpoints)
			if !reflect.DeepEqual(cep.Subsets, oep.Subsets) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(cep))
				key := "Endpoints/" + ObjKey(cep)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
				utils.AviLog.Info.Printf("Added UPDATE Endpoint key from the kubernetes controller %s", key)
			}
		},
	}

	svc_event_handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(svc))
			key := "Service/" + ObjKey(svc)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
			utils.AviLog.Info.Printf("Added ADD Service key from the kubernetes controller %s", key)
		},
		DeleteFunc: func(obj interface{}) {
			svc, ok := obj.(*corev1.Service)
			if !ok {
				// endpoints was deleted but its final state is unrecorded.
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Error.Printf("couldn't get object from tombstone %#v", obj)
					return
				}
				svc, ok = tombstone.Obj.(*corev1.Service)
				if !ok {
					utils.AviLog.Error.Printf("Tombstone contained object that is not an Service: %#v", obj)
					return
				}
			}
			svc = obj.(*corev1.Service)
			namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(svc))
			key := "Service/" + ObjKey(svc)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
			utils.AviLog.Info.Printf("Added DELETE Service key from the kubernetes controller %s", key)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldobj := old.(*corev1.Service)
			svc := cur.(*corev1.Service)
			if oldobj.ResourceVersion != svc.ResourceVersion {
				// Only add the key if the resource versions have changed.
				namespace, _, _ := cache.SplitMetaNamespaceKey(ObjKey(svc))
				key := "Service/" + ObjKey(svc)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
				utils.AviLog.Info.Printf("Added UPDATE service key from the kubernetes controller %s", key)
			}
		},
	}

	c.informers.EpInformer.Informer().AddEventHandler(ep_event_handler)
	c.informers.ServiceInformer.Informer().AddEventHandler(svc_event_handler)

}

func (c *AviController) Start(stopCh <-chan struct{}) {
	go c.informers.ServiceInformer.Informer().Run(stopCh)
	go c.informers.EpInformer.Informer().Run(stopCh)

	if !cache.WaitForCacheSync(stopCh,
		c.informers.EpInformer.Informer().HasSynced,
		c.informers.ServiceInformer.Informer().HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
	} else {
		utils.AviLog.Info.Print("Caches synced")
	}
}

// // Run will set up the event handlers for types we are interested in, as well
// // as syncing informer caches and starting workers. It will block until stopCh
// // is closed, at which point it will shutdown the workqueue and wait for
// // workers to finish processing their current work items.
func (c *AviController) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	// for i := uint32(0); i < c.num_workers; i++ {
	// 	defer c.workqueue[i].ShutDown()
	// }

	// Start the informer factories to begin populating the informer caches
	// utils.AviLog.Info.Print("Starting Avi controller")

	// utils.AviLog.Info.Print("Starting workers")
	// // Launch two workers to process Foo resources
	// sharedQueue := utils.SharedWorkQueueWrappers().GetQueueByName("MCPLayer")
	// for i := uint32(0); i < sharedQueue.NumWorkers; i++ {
	// 	go wait.Until(sharedQueue.runWorker, time.Second, stopCh)
	// }

	utils.AviLog.Info.Print("Started the Kubernetes Controller")
	<-stopCh
	utils.AviLog.Info.Print("Shutting down the Kubernetes Controller")

	return nil
}

// // processNextWorkItem will read a single work item off the workqueue and
// // attempt to process it, by calling the syncHandler.
// func (c *AviController) processNextWorkItem(worker_id uint32) bool {
// 	obj, shutdown := c.workqueue[worker_id].Get()

// 	if shutdown {
// 		return false
// 	}

// 	// We wrap this block in a func so we can defer c.workqueue.Done.
// 	err := func(obj interface{}) error {
// 		// We call Done here so the workqueue knows we have finished
// 		// processing this item. We also must remember to call Forget if we
// 		// do not want this work item being re-queued. For example, we do
// 		// not call Forget if a transient error occurs, instead the item is
// 		// put back on the workqueue and attempted again after a back-off
// 		// period.
// 		defer c.workqueue[worker_id].Done(obj)
// 		var ok bool
// 		var ev string
// 		// We expect string to come off the workqueue.  We do this as the
// 		// delayed nature of the workqueue means the items in the informer
// 		// cache may actually be more up to date that when the item was
// 		// initially put onto the workqueue.
// 		if ev, ok = obj.(string); !ok {
// 			// As the item in the workqueue is actually invalid, we call
// 			// Forget here else we'd go into a loop of attempting to
// 			// process a work item that is invalid.
// 			c.workqueue[worker_id].Forget(obj)
// 			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
// 			return nil
// 		}
// 		// Run the syncHandler, passing it the ev resource to be synced.
// 		if err := c.syncHandler(ev, worker_id); err != nil {
// 			// If it's a sync error, let's not re-queue the object.
// 			_, ok := err.(*utils.SkipSyncError)
// 			if !ok {
// 				// Put the item back on the workqueue to handle any transient errors.
// 				c.workqueue[worker_id].AddRateLimited(obj)

// 				return fmt.Errorf("error syncing '%v': %s, requeuing", ev, err.Error())
// 			} else {
// 				// No need to put the item back
// 				utils.AviLog.Info.Printf("Skip sync of '%s'", ev)
// 				c.workqueue[worker_id].Forget(obj)
// 				return nil
// 			}

// 		}
// 		// Finally, if no error occurs we Forget this item so it does not
// 		// get queued again until another change happens.
// 		c.workqueue[worker_id].Forget(obj)
// 		utils.AviLog.Info.Printf("Successfully synced '%s'", ev)
// 		return nil
// 	}(obj)

// 	if err != nil {
// 		runtime.HandleError(err)
// 		return true
// 	}

// 	return true
// }

// // syncHandler compares the actual state with the desired, and attempts to
// // converge the two. It then updates the Status block of the Foo resource
// // with the current status of the resource.
// func (c *AviController) syncHandler(key string, worker_id uint32) error {
// 	obj_type_ns := strings.SplitN(key, "/", 3)
// 	if len(obj_type_ns) != 3 {
// 		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
// 		return nil
// 	}
// 	// Convert the namespace/name string into a distinct namespace and name
// 	namespace, name, err := cache.SplitMetaNamespaceKey(obj_type_ns[2])
// 	if err != nil {
// 		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
// 		return nil
// 	}

// 	var obj interface{}
// 	var evt utils.EvType
// 	// Get the latest Service resource with this namespace/name
// 	if obj_type_ns[0] == "Service" {
// 		obj, err = c.informers.ServiceInformer.Lister().Services(namespace).Get(name)
// 	} else if obj_type_ns[0] == "Endpoints" {
// 		obj, err = c.informers.EpInformer.Lister().Endpoints(namespace).Get(name)
// 	} else if obj_type_ns[0] == "Ingress" {
// 		obj, err = c.informers.IngInformer.Lister().Ingresses(namespace).Get(name)
// 	} else {
// 		utils.AviLog.Error.Printf("Unable to handle unknown obj type %v", key)
// 		return errors.New("Unable to handle unknown obj type")
// 	}

// 	if err != nil {
// 		// The Obj may no longer exist, in which case we process deletion
// 		if k8s_errors.IsNotFound(err) {
// 			runtime.HandleError(fmt.Errorf("obj '%s' in work queue no longer exists", key))
// 			utils.AviLog.Info.Printf("Obj key NotFound %v obj type %T value %v", key, obj, obj)
// 			evt = utils.DeleteEv
// 		} else {
// 			return err
// 		}
// 	} else {
// 		evt = utils.UpdateEv
// 	}

// 	if obj_type_ns[0] == "Endpoints" {
// 		if evt == utils.UpdateEv {
// 			_, err = c.k8s_ep.K8sObjCrUpd(worker_id, obj.(*corev1.Endpoints),
// 				"", obj_type_ns[1])
// 		} else {
// 			_, err = c.k8s_ep.K8sObjDelete(worker_id, key)
// 		}
// 	} else if obj_type_ns[0] == "Service" {
// 		if evt == utils.UpdateEv {
// 			_, err = c.k8s_svc.K8sObjCrUpd(worker_id, obj.(*corev1.Service))
// 		} else {
// 			_, err = c.k8s_svc.K8sObjDelete(worker_id, key)
// 		}
// 	}

// 	// TODO
// 	// c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
// 	return err
// }
