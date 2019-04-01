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

package objects

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
)

//Common utils like processing worker queue, that is common for all objects.
type CommonUtils struct {
	workqueue workqueue.RateLimitingInterface
}

func NewCommonUtils(objectName string) *CommonUtils {
	utils := &CommonUtils{}
	utils.workqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("avi-%s", objectName))
	return utils
}

func (c *CommonUtils) processObjectQueue() {
	for c.processNextWorkItem() {
	}
}

func (c *CommonUtils) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	var ok bool
	var ev string
	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		if ev, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncToAvi, passing it the ev resource to be synced.
		if err := c.syncToAvi(ev); err != nil {
			c.workqueue.Forget(obj)
			return nil
		}

		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *CommonUtils) syncToAvi(key string) error {
	// Perform CRUD to AVI by first looking up the latest object from IstioObjectMap and it's corresponding action viz. ADD, UPDATE, DELETE
	return nil
}
