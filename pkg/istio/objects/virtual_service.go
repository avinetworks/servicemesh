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

//This module should give a wrapper over underlying objects to give easy API access for Virtual Services.
package objects

import (
	"sync"

	"github.com/avinetworks/servicemesh/pkg/utils"
	networking "istio.io/api/networking/v1alpha3"
)

var instance *VirtualServiceLister
var vsonce sync.Once

type IstioVirtualService interface {
	VirtualService(ns string) VirtualServiceNameSpaceIntf
	//List() *[]ObjectStore
}

type VirtualServiceLister struct {
	store *ObjectStore
}

func (v *VirtualServiceLister) GetAllVirtualServices() map[string]map[string]string {
	// This method should return a map that looks like this: {ns: [obj1, obj2]}
	// This is particularly useful if we want to know what are the vs names
	// present in a namespace without affecting the actual store objects.
	allNamespaces := v.store.GetAllNamespaces()
	allVirtualServices := make(map[string]map[string]string)
	if len(allNamespaces) != 0 {
		// Iterate over each namespace and formulate the map
		for _, ns := range allNamespaces {
			allVirtualServices[ns] = v.VirtualService(ns).GetAllVSNames()
		}
	}
	return allVirtualServices

}

func SharedVirtualServiceLister() *VirtualServiceLister {
	vsonce.Do(func() {
		VSstore := NewObjectStore()
		instance = &VirtualServiceLister{}
		instance.store = VSstore
	})
	return instance
}

func (v *VirtualServiceLister) VirtualService(ns string) *VirtualServiceNSCache {
	namespacedObjects := v.store.GetNSStore(ns)
	gwInstance := SharedGatewayLister()
	return &VirtualServiceNSCache{namespace: ns, objects: namespacedObjects, gwInstance: gwInstance}
}

type VirtualServiceNameSpaceIntf interface {
	Get(name string) (bool, *IstioObject)
	Update(obj *IstioObject) bool
	List() map[string]*IstioObject
	Delete(name string) bool
}

type VirtualServiceNSCache struct {
	namespace  string
	objects    *ObjectMapStore
	gwInstance *GatewayLister
}

func (v *VirtualServiceNSCache) Get(name string) (bool, *IstioObject) {
	found, obj := v.objects.Get(name)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		// Let's return a VS object now
		_, ok := obj.(*IstioObject).Spec.(*networking.VirtualService)
		if !ok {
			// This is not the right object to cast to VirtualService return error.
			utils.AviLog.Warning.Printf("Wrong object type found in store, will return nil %v", obj)
			return false, nil
		}
		return true, obj.(*IstioObject)
	}
}

func (v *VirtualServiceNSCache) Update(obj *IstioObject) {
	// Check if the resource version in the repo is the same as the one sent.
	found, storedVS := v.Get(obj.ConfigMeta.Name)
	if found && storedVS.ConfigMeta.ResourceVersion == obj.ConfigMeta.ResourceVersion {
		utils.AviLog.Trace.Printf("Nothing to update, resource versions same %s", obj.ConfigMeta.Name)
		return
	}
	v.objects.AddOrUpdate(obj.Name, obj)
	// This should be updated in Layer 2.
	//v.UpdateGatewayRefs(obj)
}

func (v *VirtualServiceNSCache) UpdateGatewayRefs(obj *IstioObject) {
	// First get the VS Name and then look for gateway. Add the gateway to the list.
	// This is not thread safe.
	gateways := v.GetGatewayNamesForVS(obj)
	for _, gateway := range gateways {
		_, vsList := v.gwInstance.Gateway(obj.ConfigMeta.Namespace).GetVSMapping(gateway)
		vsList = append(vsList, obj.ConfigMeta.Name)
		v.gwInstance.Gateway(obj.ConfigMeta.Namespace).UpdateGWVSMapping(gateway, vsList)
	}
}

func (v *VirtualServiceNSCache) List() map[string]*IstioObject {
	// TODO (sudswas): Let's check if we can abstract out the store objects
	// completely. There's still a possibility that if we pass the references
	// we maybe allowing upper layers to modify the object that would directly
	// impact the store objects.
	convertedMap := make(map[string]*IstioObject)
	// Change the empty interface to IstioObject. Avoid Duck Typing.
	for key, value := range v.objects.ObjectMap {
		convertedMap[key] = value.(*IstioObject)
	}
	return convertedMap
}

func (v *VirtualServiceNSCache) Delete(name string) bool {
	// Obtain the object for this VS
	//_, vsObj := v.Get(name)
	// To be updated in Layer 2
	// if found {
	// 	// Let's delete the Gateway relationship first.
	// 	v.DeleteGatewayRefs(vsObj)
	// }

	return v.objects.Delete(name)
}

func (v *VirtualServiceNSCache) GetAllVSNames() map[string]string {
	// Obtain the object for this VS
	allObjects := v.objects.GetAllObjectNames()
	objVersionsMap := make(map[string]string)
	// Now let's parse the object names and their corresponding resourceversions in a Map
	for _, obj := range allObjects {
		objVersionsMap[obj.(*IstioObject).ConfigMeta.Name] = obj.(*IstioObject).ConfigMeta.ResourceVersion
	}
	return objVersionsMap
}

func (v *VirtualServiceNSCache) DeleteGatewayRefs(obj *IstioObject) {
	gateways := v.GetGatewayNamesForVS(obj)
	for _, gateway := range gateways {
		_, vsList := v.gwInstance.Gateway(obj.ConfigMeta.Namespace).GetVSMapping(gateway)
		if Contains(vsList, obj.ConfigMeta.Name) {
			vsList = Remove(vsList, obj.ConfigMeta.Name)
		}
		v.gwInstance.Gateway(obj.ConfigMeta.Namespace).UpdateGWVSMapping(gateway, vsList)
	}
}

func (v *VirtualServiceNSCache) GetGatewayNamesForVS(vs *IstioObject) []string {
	vsObj, ok := vs.Spec.(*networking.VirtualService)
	if !ok {
		// This is not the right object to cast to VirtualService return error.
		utils.AviLog.Warning.Printf("Wrong object passed. Expecting a Virtual Service object %v", vsObj)
		return nil
	}
	var gateways []string
	if len(vsObj.Gateways) == 0 {
		utils.AviLog.Warning.Println("Avi does not support virtual services for internal traffic")
	}
	for _, gateway := range vsObj.Gateways {
		if gateway != IstioMeshGateway {
			gateways = append(gateways, gateway)
		}
	}
	return gateways
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
