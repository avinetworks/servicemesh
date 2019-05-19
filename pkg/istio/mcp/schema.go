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
	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/golang/protobuf/proto"
)

var (
	// VirtualService describes v1alpha3 route rules
	VirtualService = ProtoSchema{
		Type:        "virtual-service",
		Plural:      "virtual-services",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.VirtualService",
		GetAll:      GetAllVSes,
		Store:       ProcessVS,
		Collection:  "istio/networking/v1alpha3/virtualservices",
	}
	Gateway = ProtoSchema{
		Type:        "gateway",
		Plural:      "gateways",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.Gateway",
		Store:       ProcessGateway,
		GetAll:      GetAllVSes,
		Collection:  "istio/networking/v1alpha3/gateways",
	}
	// ServiceEntry describes service entries
	ServiceEntry = ProtoSchema{
		Type:        "service-entry",
		Plural:      "service-entries",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.ServiceEntry",
		// Will change to ProcessSE, once we have the store updates in place.
		Store:      ProcessVS,
		GetAll:     GetAllVSes,
		Collection: "istio/networking/v1alpha3/serviceentries",
	}

	// IstioConfigTypes lists all Istio config types with schemas and validation
	IstioConfigTypes = ConfigDescriptor{
		VirtualService,
		Gateway,
		ServiceEntry,
	}
)

func GetAllVSes() map[string]map[string]string {
	// Obtain all the VirtualServices across namespaces. This should be of the form:
	// {ns : {obj_name1: rvs, obj_name2: rv}}
	StoredVSes := istio_objs.SharedVirtualServiceLister().GetAllVirtualServices()
	return StoredVSes
}

func ProcessVS(name string, namespace string, configMeta istio_objs.ConfigMeta, msg proto.Message) {
	istio_object := istio_objs.NewIstioObject(configMeta, msg)
	// First do a GET on this VS to check if it exists already in the store or not.
	ok, obj := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).Get(name)
	if ok {
		// Object found, let's check now if it's a update or a no-op
		if configMeta.ResourceVersion == obj.ConfigMeta.ResourceVersion {
			// The same resourceversion obtained from store. Won't process anything
			return
		} else {
			// Update the new object in store
			istio_objs.SharedVirtualServiceLister().VirtualService(namespace).Update(istio_object)
			return
		}

	}
	istio_objs.SharedVirtualServiceLister().VirtualService(namespace).Update(istio_object)
}

func ProcessGateway(name string, namespace string, configMeta istio_objs.ConfigMeta, msg proto.Message) {
	istio_object := istio_objs.NewIstioObject(configMeta, msg)
	// First do a GET on this Gateway to check if it exists already in the store or not.
	ok, obj := istio_objs.SharedGatewayLister().Gateway(namespace).Get(name)
	if ok {
		// Object found, let's check now if it's a update or a no-op
		if configMeta.ResourceVersion == obj.ConfigMeta.ResourceVersion {
			// The same resourceversion obtained from store. Won't process anything
			return
		} else {
			// Update the new object in store
			istio_objs.SharedGatewayLister().Gateway(namespace).Update(istio_object)
			return
		}

	}
	istio_objs.SharedGatewayLister().Gateway(namespace).Update(istio_object)
}

func (descriptor ConfigDescriptor) CalculateUpdates(prevStore map[string]map[string]string, currentStore map[string]map[string]string) []string {
	// This method calculates the ADD/DELETES/UPDATES and updates the workqueue.
	var changedKeys []string
	for namespace, currObjMap := range currentStore {
		prevObjMap := prevStore[namespace]
		for objName, oldRV := range prevObjMap {
			// Check if new has the obj present in the old.
			currRV, found := currObjMap[objName]
			if found {
				// Object exists in new - check resource versions
				if currRV != oldRV {
					// This is a update event.
					changedKeys = append(changedKeys, objName)
				}
			} else {
				// New does not have obj present in old. It's a delete
				changedKeys = append(changedKeys, objName)
			}
			// Let's check if it's the same resource versions
		}
		for objName, _ := range currObjMap {
			_, found := prevObjMap[objName]

			if !found {
				// Object present in new but absent in old - it's an ADD
				changedKeys = append(changedKeys, objName)
			}
		}
	}
	return changedKeys
}

// GetByType finds a schema by type if it is available
func (descriptor ConfigDescriptor) GetByType(name string) (ProtoSchema, bool) {
	for _, schema := range descriptor {
		if schema.Type == name {
			return schema, true
		}
	}
	return ProtoSchema{}, false
}

type ConfigDescriptor []ProtoSchema

type ProtoSchema struct {
	ClusterScoped    bool
	Type             string
	Plural           string
	Group            string
	Version          string
	MessageName      string
	CalculateUpdates func(map[string]map[string]string, map[string]map[string]string)
	GetAll           func() map[string]map[string]string
	Store            func(string, string, istio_objs.ConfigMeta, proto.Message)
	Collection       string
}
