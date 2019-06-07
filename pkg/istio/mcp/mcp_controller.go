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

package mcp

import (
	"fmt"
	"strings"
	"sync"
	"time"

	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	queue "github.com/avinetworks/servicemesh/pkg/k8s"
	"github.com/avinetworks/servicemesh/pkg/utils"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pkg/mcp/sink"
)

type Controller struct {
	syncedMu                sync.Mutex
	synced                  map[string]bool
	descriptorsByCollection map[string]ProtoSchema
}

func NewController() *Controller {
	synced := make(map[string]bool)
	descriptorsByMessageName := make(map[string]ProtoSchema, len(IstioConfigTypes))
	for _, descriptor := range IstioConfigTypes {
		// don't register duplicate descriptors for the same collection
		if _, ok := descriptorsByMessageName[descriptor.Collection]; !ok {
			descriptorsByMessageName[descriptor.Collection] = descriptor
			synced[descriptor.Collection] = false
		}
	}
	return &Controller{
		synced:                  synced,
		descriptorsByCollection: descriptorsByMessageName,
	}
}

// HasSynced is used to tell the MCP server that the first set of items that were
// supposed to be sent to this client for the registered types has been received.
func (c *Controller) HasSynced() bool {
	var notReady []string

	c.syncedMu.Lock()
	for messageName, synced := range c.synced {
		if !synced {
			notReady = append(notReady, messageName)
		}
	}
	c.syncedMu.Unlock()

	if len(notReady) > 0 {
		//log.Infof("Configuration not synced: first push for %v not received", notReady)
		return false
	}
	return true
}

// ConfigDescriptor returns all the ConfigDescriptors that this
// controller is responsible for
func (c *Controller) ConfigDescriptor() ConfigDescriptor {
	return IstioConfigTypes
}

// Apply receives changes from MCP server and creates the
// corresponding config
func (c *Controller) Apply(change *sink.Change) error {
	descriptor, ok := c.descriptorsByCollection[change.Collection]
	if !ok {
		return fmt.Errorf("apply type not supported %s", change.Collection)
	}

	schema, valid := c.ConfigDescriptor().GetByType(descriptor.Type)
	// Retrive all the existing store objects
	prevStore := schema.GetAll()
	if !valid {
		return fmt.Errorf("descriptor type not supported %s", change.Collection)
	}
	c.syncedMu.Lock()
	c.synced[change.Collection] = true
	c.syncedMu.Unlock()
	presentValues := make(map[string]map[string]*istio_objs.IstioObject)
	createTime := time.Now()
	for _, obj := range change.Objects {
		namespace, name := extractNameNamespace(obj.Metadata.Name)
		if obj.Metadata.CreateTime != nil {
			var err error
			if createTime, err = types.TimestampFromProto(obj.Metadata.CreateTime); err != nil {
				continue
			}
		}
		configMeta := istio_objs.ConfigMeta{
			Type:              descriptor.Type,
			Group:             descriptor.Group,
			Version:           descriptor.Version,
			Name:              name,
			Namespace:         namespace,
			ResourceVersion:   obj.Metadata.Version,
			CreationTimestamp: createTime,
			Labels:            obj.Metadata.Labels,
			Annotations:       obj.Metadata.Annotations,
		}
		istioObj := istio_objs.NewIstioObject(configMeta, obj.Body)
		addLocalIstioObjs(name, namespace, istioObj, presentValues)
	}
	schema.Store(presentValues, prevStore)
	newStore := schema.GetAll()
	changedKeysMap := c.ConfigDescriptor().CalculateUpdates(prevStore, newStore)
	sharedQueue := queue.SharedWorkQueue().GetQueueByName(utils.ObjectIngestionLayer)
	// Sharding logic here.
	for namespace, objKeys := range changedKeysMap {
		// Hash on namespace
		bkt := utils.Bkt(namespace, sharedQueue.NumWorkers)
		for _, key := range objKeys {
			key = descriptor.Type + "/" + namespace + "/" + key
			sharedQueue.Workqueue[bkt].AddRateLimited(key)
			utils.AviLog.Info.Printf("Added Key from MCP update to the workerqueue %s", key)
		}
	}
	return nil
}

func addLocalIstioObjs(name string, namespace string, istio_object *istio_objs.IstioObject, presentValues map[string]map[string]*istio_objs.IstioObject) {
	obj, ok := presentValues[namespace]
	if ok {
		// Namespace is found. Let's update the value against it.
		obj[name] = istio_object
	} else {
		objMap := map[string]*istio_objs.IstioObject{name: istio_object}
		presentValues[namespace] = objMap
	}
}

func extractNameNamespace(metadataName string) (string, string) {
	segments := strings.Split(metadataName, "/")
	if len(segments) == 2 {
		return segments[0], segments[1]
	}
	return "", segments[0]
}
