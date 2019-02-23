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

package main

import (
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/avinetworks/avi_k8s_controller/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type AviController struct {
	num_workers     uint32
	worker_id       uint32
	worker_id_mutex sync.Mutex
	recorder        record.EventRecorder
	informers       *Informers
	workqueue       []workqueue.RateLimitingInterface
	k8s_ep          *K8sEp
	k8s_svc         *K8sSvc
	epChannel       chan ChannelData
	svcChannel      chan ChannelData
	stopCh          <-chan struct{}
}

func ObjKey(obj interface{}) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utils.AviLog.Warning.Print(err)
	}

	return key
}

func Bkt(key string, num_workers uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	bkt := h.Sum32() & (num_workers - 1)
	return bkt
}

func NewInformers(cs *kubernetes.Clientset) *Informers {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(cs, time.Second*30)
	informers := Informers{
		ServiceInformer: kubeInformerFactory.Core().V1().Services(),
		EpInformer:      kubeInformerFactory.Core().V1().Endpoints(),
		IngInformer:     kubeInformerFactory.Extensions().V1beta1().Ingresses(),
	}
	return &informers
}

func NewAviController(num_workers uint32, inf *Informers, cs *kubernetes.Clientset,
	k8s_ep *K8sEp, k8s_svc *K8sSvc) *AviController {
	utils.AviLog.Info.Printf("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(utils.AviLog.Info.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: cs.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "avi-k8s-controller"})
	stopCh := SetupSignalHandler()

	// Initialize bufferred channels per object type. Right now assuming the size of 100 as good enough.
	endpoint_chan := make(chan ChannelData, 100)
	service_chan := make(chan ChannelData, 100)
	// initialize go routines
	c := &AviController{
		num_workers: num_workers,
		worker_id:   (uint32(1) << num_workers) - 1,
		recorder:    recorder,
		informers:   inf,
		k8s_ep:      k8s_ep,
		k8s_svc:     k8s_svc,
		epChannel:   endpoint_chan,
		svcChannel:  service_chan,
		stopCh:      stopCh,
	}

	go c.handleUpdates(endpoint_chan)
	go c.handleUpdates(service_chan)
	c.workqueue = make([]workqueue.RateLimitingInterface, num_workers)
	for i := uint32(0); i < num_workers; i++ {
		c.workqueue[i] = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("avi-%d", i))
	}
	ep_event_handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)
			key := "Endpoints/" + utils.CrudHashKey("Endpoints", ep) + "/" + ObjKey(ep)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
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
			key := "Endpoints/" + utils.CrudHashKey("Endpoints", ep) + "/" + ObjKey(ep)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, cur interface{}) {
			oep := old.(*corev1.Endpoints)
			cep := cur.(*corev1.Endpoints)
			if !reflect.DeepEqual(cep.Subsets, oep.Subsets) {
				key := "Endpoints/" + utils.CrudHashKey("Endpoints", cep) + "/" + ObjKey(cep)
				bkt := Bkt(key, num_workers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
	}

	svc_event_handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			key := "Service/" + utils.CrudHashKey("Service", svc) + "/" + ObjKey(svc)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
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
			key := "Service/" + utils.CrudHashKey("Service", svc) + "/" + ObjKey(svc)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, cur interface{}) {
			// TODO Check if anything has changed here ?
			svc := cur.(*corev1.Service)
			key := "Service/" + utils.CrudHashKey("Service", svc) + "/" + ObjKey(svc)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}

	ing_event_handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ing := obj.(*extensions.Ingress)
			key := "Ingress/" + utils.CrudHashKey("Ingress", ing) + "/" + ObjKey(ing)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		DeleteFunc: func(obj interface{}) {
			ing, ok := obj.(*extensions.Ingress)
			if !ok {
				// endpoints was deleted but its final state is unrecorded.
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Error.Printf("couldn't get object from tombstone %#v", obj)
					return
				}
				ing, ok = tombstone.Obj.(*extensions.Ingress)
				if !ok {
					utils.AviLog.Error.Printf("Tombstone contained object that is not an Ingress: %#v", obj)
					return
				}
			}
			ing = obj.(*extensions.Ingress)
			key := "Ingress/" + utils.CrudHashKey("Ingress", ing) + "/" + ObjKey(ing)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, cur interface{}) {
			// TODO Check if anything has changed here ?
			ing := cur.(*extensions.Ingress)
			key := "Ingress/" + utils.CrudHashKey("Ingress", ing) + "/" + ObjKey(ing)
			bkt := Bkt(key, num_workers)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}

	c.informers.EpInformer.Informer().AddEventHandler(ep_event_handler)
	c.informers.ServiceInformer.Informer().AddEventHandler(svc_event_handler)
	c.informers.IngInformer.Informer().AddEventHandler(ing_event_handler)

	return c
}

func (c *AviController) Start() {
	go c.informers.ServiceInformer.Informer().Run(c.stopCh)
	go c.informers.EpInformer.Informer().Run(c.stopCh)
	go c.informers.IngInformer.Informer().Run(c.stopCh)

	if !cache.WaitForCacheSync(c.stopCh,
		c.informers.EpInformer.Informer().HasSynced,
		c.informers.ServiceInformer.Informer().HasSynced,
		c.informers.IngInformer.Informer().HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
	} else {
		utils.AviLog.Info.Print("Caches synced")
	}
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *AviController) Run() error {
	defer runtime.HandleCrash()
	for i := uint32(0); i < c.num_workers; i++ {
		defer c.workqueue[i].ShutDown()
	}

	// Start the informer factories to begin populating the informer caches
	utils.AviLog.Info.Print("Starting Avi controller")

	utils.AviLog.Info.Print("Starting workers")
	// Launch two workers to process Foo resources
	for i := uint32(0); i < c.num_workers; i++ {
		go wait.Until(c.runWorker, time.Second, c.stopCh)
	}

	utils.AviLog.Info.Print("Started workers")
	<-c.stopCh
	utils.AviLog.Info.Print("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue. Pick a worker_id from worker_id mask
func (c *AviController) runWorker() {
	worker_id := uint32(0xffffffff)
	c.worker_id_mutex.Lock()
	for i := uint32(0); i < c.num_workers; i++ {
		if ((uint32(1) << i) & c.worker_id) != 0 {
			worker_id = i
			c.worker_id = c.worker_id & ^(uint32(1) << i)
			break
		}
	}
	c.worker_id_mutex.Unlock()
	utils.AviLog.Info.Printf("Worker id %d", worker_id)
	for c.processNextWorkItem(worker_id) {
	}
	c.worker_id_mutex.Lock()
	c.worker_id = c.worker_id | (uint32(1) << worker_id)
	c.worker_id_mutex.Unlock()
	utils.AviLog.Info.Printf("Worker id %d restarting", worker_id)
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *AviController) processNextWorkItem(worker_id uint32) bool {
	obj, shutdown := c.workqueue[worker_id].Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue[worker_id].Done(obj)
		var ok bool
		var ev string
		// We expect string to come off the workqueue.  We do this as the
		// delayed nature of the workqueue means the items in the informer
		// cache may actually be more up to date that when the item was
		// initially put onto the workqueue.
		if ev, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue[worker_id].Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the ev resource to be synced.
		if err := c.syncHandler(ev, worker_id); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue[worker_id].AddRateLimited(obj)
			return fmt.Errorf("error syncing '%v': %s, requeuing", ev, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue[worker_id].Forget(obj)
		utils.AviLog.Info.Printf("Successfully synced '%s'", ev)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *AviController) syncHandler(key string, worker_id uint32) error {
	obj_type_ns := strings.SplitN(key, "/", 3)
	if len(obj_type_ns) != 3 {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	// Write it to a channel
	if obj_type_ns[0] == "Service" {
		utils.AviLog.Info.Printf("Got an update for Service Object '%s' will push this to the channel", obj_type_ns)
		c.svcChannel <- ChannelData{key: key, workerId: worker_id}
	} else if obj_type_ns[0] == "Endpoints" {
		utils.AviLog.Info.Printf("Got an update for Endpoint Object '%s' will push this to the channel", obj_type_ns)
		c.epChannel <- ChannelData{key: key, workerId: worker_id}
	}

	return nil
}

// This method should get executed inside a go routine.
func (c *AviController) handleUpdates(channelData <-chan ChannelData) error {
	for {
		select {
		case <-c.stopCh:
			utils.AviLog.Info.Printf("Received a SIGTERM, will terminate")
			return nil
		default:
			data := <-channelData
			utils.AviLog.Info.Printf("Just received an update: '%s'", data.key)
			obj_type_ns := strings.SplitN(data.key, "/", 3)
			namespace, name, err := cache.SplitMetaNamespaceKey(obj_type_ns[2])
			if err != nil {
				runtime.HandleError(fmt.Errorf("invalid resource key: %s", data.key))
				return nil
			}
			var obj interface{}
			var evt EvType
			// Get the latest Service resource with this namespace/name
			if obj_type_ns[0] == "Service" {
				obj, err = c.informers.ServiceInformer.Lister().Services(namespace).Get(name)
			} else if obj_type_ns[0] == "Endpoints" {
				obj, err = c.informers.EpInformer.Lister().Endpoints(namespace).Get(name)
			} else if obj_type_ns[0] == "Ingress" {
				obj, err = c.informers.IngInformer.Lister().Ingresses(namespace).Get(name)
			} else {
				utils.AviLog.Error.Printf("Unable to handle unknown obj type %v", data.key)
				return errors.New("Unable to handle unknown obj type")
			}

			if err != nil {
				// The Obj may no longer exist, in which case we process deletion
				if k8s_errors.IsNotFound(err) {
					runtime.HandleError(fmt.Errorf("obj '%s' in work queue no longer exists", data.key))
					utils.AviLog.Info.Printf("Obj key NotFound %v obj type %T value %v", data.key, obj, obj)
					evt = DeleteEv
				} else {
					return err
				}
			} else {
				evt = UpdateEv
			}

			if obj_type_ns[0] == "Endpoints" {
				if evt == UpdateEv {
					_, err = c.k8s_ep.K8sObjCrUpd(data.workerId, obj.(*corev1.Endpoints),
						"", obj_type_ns[1])
				} else {
					_, err = c.k8s_ep.K8sObjDelete(data.workerId, data.key)
				}
			} else if obj_type_ns[0] == "Service" {
				if evt == UpdateEv {
					_, err = c.k8s_svc.K8sObjCrUpd(data.workerId, obj.(*corev1.Service))
				} else {
					_, err = c.k8s_svc.K8sObjDelete(data.workerId, data.key)
				}
			}

			// TODO
			// c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
			return err
		}
	}
}
