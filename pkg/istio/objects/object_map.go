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
	"sync"

	"github.com/golang/protobuf/proto"
)

// This is a basic object that is used to store istio object information.
type IstioObject struct {
	ConfigMeta
	Spec proto.Message
}

func NewIstioObject(configMeta ConfigMeta, spec proto.Message) *IstioObject {
	obj := &IstioObject{}
	obj.ConfigMeta = configMeta
	obj.Spec = spec
	return obj
}

// Map Key is - namespace:object_name. This stores already discovered Istio Object Map.
type IstioObjectMap struct {
	objMap   map[string]*IstioObject
	map_lock sync.RWMutex
}

func NewIstioObjectMap() *IstioObjectMap {
	objMap := IstioObjectMap{}
	objMap.objMap = make(map[string]*IstioObject)
	return &objMap
}

func (c *IstioObjectMap) GetObjByNameNamespace(key string) (bool, *IstioObject) {
	c.map_lock.RLock()
	defer c.map_lock.RUnlock()
	val, ok := c.objMap[key]
	if !ok {
		return ok, nil
	} else {
		return ok, val
	}
}

func (o *IstioObjectMap) AddObj(key string, val *IstioObject) {
	// Add a gateway object in this Queue for processing.
	o.map_lock.RLock()
	defer o.map_lock.RUnlock()
	o.objMap[key] = val
}

/* This has the mechanics to perform actions on the Istio Object. We keep a map to
figure out what are the operations to be performed against an Istio object.*/
type IstioObjectOpsContainer struct {
	opsMap  map[string][]string
	opsLock sync.RWMutex
	utils   *CommonUtils
}

func NewIstioObjectOpsContainer(objName string) *IstioObjectOpsContainer {
	opsCont := &IstioObjectOpsContainer{}
	opsCont.opsMap = make(map[string][]string)
	opsCont.utils = NewCommonUtils(objName)
	//TODO (sudswas): Do we need to worry about shutting this down based on SIGTERM?
	// Go routine spun up per object type of Istio.
	go opsCont.utils.processObjectQueue()
	return opsCont
}

/* This method keeps adding operations against a given gateway key.
The draining go routine, will ensure that operations are processed and eventually
the actions are removed from the ops queue. But till that happens, we should also
be able to optimize the cost of repeated operations to fewer operations for us to
process*/
func (o *IstioObjectOpsContainer) AddOps(key string, oper string) {
	o.opsLock.RLock()
	defer o.opsLock.RUnlock()
	// Let's add the key to the work queue.
	o.utils.workqueue.AddRateLimited(key)
	switch oper {
	case "ADD":
		o.opsMap[key] = append(o.opsMap[key], oper)
	case "DELETE":
		last_oper := o.opsMap[key][len(o.opsMap[key])-1]
		if last_oper != "DELETE" {
			//If the last operation is ADD/UPDATE but it was deleted after wards - we just need to process DELETE.
			o.opsMap[key] = []string{}
			o.opsMap[key] = append(o.opsMap[key], oper)
		}
	case "UPDATE":
		// There are two possibilities with an UPDATE: [ADD] existed and now UPDATE came- Consolidate to one [ADD] - that means discard this op.
		// [UPDATE] existed, and now another UPDATE came - Consolidate to one [UPDATE]. That also means discard this update.
		// Else if there are no pending operations: [UPDATE] that is length of ops is 0.
		// Note UPDATE cannot come after DELETE.
		if len(o.opsMap[key]) == 0 {
			o.opsMap[key] = append(o.opsMap[key], oper)
		}
	}
}
