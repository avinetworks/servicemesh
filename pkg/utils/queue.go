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

package utils

import (
	"fmt"
	"hash/fnv"
	"sync"

	"k8s.io/client-go/util/workqueue"
)

var queuewrapper sync.Once
var queueInstance *WorkQueueWrapper
var fixedQueues = [...]WorkerQueue{WorkerQueue{NumWorkers: NumWorkers, WorkqueueName: "MCPLayer"}}

type WorkQueueWrapper struct {
	// This struct should manage a set of WorkerQueues for the various layers
	queueCollection map[string]*WorkerQueue
}

func (w *WorkQueueWrapper) GetQueueByName(queueName string) *WorkerQueue {
	workqueue, _ := w.queueCollection[queueName]
	return workqueue
}

func SharedWorkQueueWrappers() *WorkQueueWrapper {
	queuewrapper.Do(func() {
		queueInstance = &WorkQueueWrapper{}
		queueInstance.queueCollection = make(map[string]*WorkerQueue)
		for _, queue := range fixedQueues {
			workqueue := NewWorkQueue(queue.NumWorkers)
			queueInstance.queueCollection[queue.WorkqueueName] = workqueue
		}
	})
	return queueInstance
}

//Common utils like processing worker queue, that is common for all objects.
type WorkerQueue struct {
	NumWorkers    uint32
	Workqueue     []workqueue.RateLimitingInterface
	WorkqueueName string
}

func NewWorkQueue(num_workers uint32) *WorkerQueue {
	queue := &WorkerQueue{}
	queue.Workqueue = make([]workqueue.RateLimitingInterface, num_workers)
	queue.NumWorkers = num_workers
	for i := uint32(0); i < num_workers; i++ {
		queue.Workqueue[i] = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("avi-%d", i))
	}
	return queue
}

// func (c *WorkerQueue) ProcessObjectQueue() {
// 	worker_id := uint32(0xffffffff)
// 	for c.processNextWorkItem(worker_id) {
// 	}
// }

// func (c *WorkerQueue) processNextWorkItem(worker_id uint32) bool {
// 	obj, shutdown := c.Workqueue[worker_id].Get()
// 	if shutdown {
// 		return false
// 	}
// 	var ok bool
// 	var ev string
// 	// We wrap this block in a func so we can defer c.workqueue.Done.
// 	err := func(obj interface{}) error {
// 		// We call Done here so the workqueue knows we have finished
// 		// processing this item. We also must remember to call Forget if we
// 		// do not want this work item being re-queued. For example, we do
// 		// not call Forget if a transient error occurs, instead the item is
// 		// put back on the workqueue and attempted again after a back-off
// 		// period.
// 		defer c.Workqueue[worker_id].Done(obj)
// 		if ev, ok = obj.(string); !ok {
// 			// As the item in the workqueue is actually invalid, we call
// 			// Forget here else we'd go into a loop of attempting to
// 			// process a work item that is invalid.
// 			c.Workqueue[worker_id].Forget(obj)
// 			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
// 			return nil
// 		}
// 		// Run the syncToAvi, passing it the ev resource to be synced.
// 		if err := c.syncToAvi(ev); err != nil {
// 			c.Workqueue[worker_id].Forget(obj)
// 			return nil
// 		}

// 		return nil
// 	}(obj)
// 	if err != nil {
// 		runtime.HandleError(err)
// 		return true
// 	}

// 	return true
// }

// func (c *WorkerQueue) syncToAvi(key string) error {
// 	return nil
// }

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue. Pick a worker_id from worker_id mask
// func (c *WorkerQueue) runWorker() {
// 	worker_id := uint32(0xffffffff)
// 	c.workerIdMutex.Lock()
// 	for i := uint32(0); i < c.NumWorkers; i++ {
// 		if ((uint32(1) << i) & c.WorkerId) != 0 {
// 			worker_id = i
// 			c.WorkerId = c.WorkerId & ^(uint32(1) << i)
// 			break
// 		}
// 	}
// 	c.workerIdMutex.Unlock()
// 	utils.AviLog.Info.Printf("Worker id %d", worker_id)
// 	for c.processNextWorkItem(worker_id) {
// 	}
// 	c.workerIdMutex.Lock()
// 	c.WorkerId = c.WorkerId | (uint32(1) << worker_id)
// 	c.workerIdMutex.Unlock()
// 	utils.AviLog.Info.Printf("Worker id %d restarting", worker_id)
// }

func Bkt(key string, num_workers uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	bkt := h.Sum32() & (num_workers - 1)
	return bkt
}
