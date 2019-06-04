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
	"strings"
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
	// stores the Vs To Gateway Relationships
	vsgwstore *ObjectStore
	// stores the VS to Svc Relationships.
	vssvcstore *ObjectStore
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
			allVirtualServices[ns] = v.VirtualService(ns).GetAllVSNamesVers()
		}
	}
	return allVirtualServices

}

func SharedVirtualServiceLister() *VirtualServiceLister {
	vsonce.Do(func() {
		VSstore := NewObjectStore()
		instance = &VirtualServiceLister{}
		instance.store = VSstore
		instance.vsgwstore = NewObjectStore()
		instance.vssvcstore = NewObjectStore()
	})
	return instance
}

func (v *VirtualServiceLister) VirtualService(ns string) *VirtualServiceNSCache {
	namespacedObjects := v.store.GetNSStore(ns)
	gwInstance := SharedGatewayLister()
	svcInstance := SharedSvcLister()
	vsToSvcInstance := v.vssvcstore.GetNSStore(ns)
	vsToGwObjects := v.vsgwstore.GetNSStore(ns)
	return &VirtualServiceNSCache{namespace: ns, objects: namespacedObjects, gwInstance: gwInstance, svcInstance: svcInstance,
		vsToSvcInstance: vsToSvcInstance, vsToGwObjects: vsToGwObjects}
}

type VirtualServiceNameSpaceIntf interface {
	Get(name string) (bool, *IstioObject)
	Update(obj *IstioObject) bool
	List() map[string]*IstioObject
	Delete(name string) bool
}

type VirtualServiceNSCache struct {
	namespace       string
	objects         *ObjectMapStore
	gwInstance      *GatewayLister
	svcInstance     *SvcLister
	vsToGwObjects   *ObjectMapStore
	vsToSvcInstance *ObjectMapStore
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
	return v.objects.Delete(name)
}

func (v *VirtualServiceNSCache) GetAllVSNamesVers() map[string]string {
	// Obtain the object for this VS
	allObjects := v.objects.GetAllObjectNames()
	objVersionsMap := make(map[string]string)
	// Now let's parse the object names and their corresponding resourceversions in a Map
	for _, obj := range allObjects {
		objVersionsMap[obj.(*IstioObject).ConfigMeta.Name] = obj.(*IstioObject).ConfigMeta.ResourceVersion
	}
	return objVersionsMap
}

// All of the VS <--> GW relationships follow

func (v *VirtualServiceNSCache) GetGatewaysForVS(vsName string) (bool, []string) {
	// Need checks if it's found or not?
	found, gateways := v.vsToGwObjects.Get(vsName)
	if !found {
		return false, make([]string, 0)
	}
	return true, gateways.([]string)
}

func (v *VirtualServiceNSCache) DeleteVSToGw(vsName string) {
	// Need checks if it's found or not?
	v.vsToGwObjects.Delete(vsName)
}

func (v *VirtualServiceNSCache) DeleteGwToVsRefs(gwName string, vsName string) {
	_, vsList := v.gwInstance.Gateway(v.namespace).GetVSMapping(gwName)
	if Contains(vsList, vsName) {
		vsList = Remove(vsList, vsName)
	}
	v.gwInstance.Gateway(v.namespace).UpdateGWVSMapping(gwName, vsList)
}

func (v *VirtualServiceNSCache) UpdateGatewayVsRefs(obj *IstioObject) {
	// First get the VS Name and then look for gateway. Add the gateway to the list.
	gateways := v.GetGatewayNamesForVS(obj)
	for _, gateway := range gateways {
		// Check if the gateway has a qualified namespace
		namespacedGw := strings.Contains(gateway, "/")
		ns := obj.ConfigMeta.Namespace
		if namespacedGw {
			nsGw := strings.Split(gateway, "/")
			ns = nsGw[0]
			gateway = nsGw[1]
		}
		_, vsList := v.gwInstance.Gateway(ns).GetVSMapping(gateway)
		// Update the VS with it's own namespace.
		vsName := obj.ConfigMeta.Namespace + "/" + obj.ConfigMeta.Name
		if Contains(vsList, vsName) {
			// The vsName is already added, continue
			continue
		}
		vsList = append(vsList, obj.ConfigMeta.Namespace+"/"+obj.ConfigMeta.Name)
		v.gwInstance.Gateway(ns).UpdateGWVSMapping(gateway, vsList)
	}
	v.vsToGwObjects.AddOrUpdate(obj.ConfigMeta.Name, gateways)
}

// All of the VS <--> SVC relationships follow

func (v *VirtualServiceNSCache) UpdateSvcVSRefs(obj *IstioObject) {
	// First update the SVC to VS relationship
	services := v.GetServiceForVS(obj)
	utils.AviLog.Info.Printf("The Services associated with VS: %s is %s ", obj.ConfigMeta.Name, services)
	for _, service := range services {
		_, vsList := v.svcInstance.Service(obj.ConfigMeta.Namespace).GetSvcToVS(service)
		vsList = append(vsList, obj.ConfigMeta.Name)
		v.svcInstance.Service(obj.ConfigMeta.Namespace).UpdateSvcToVSMapping(service, vsList)
	}
	// Now update the VS to SVC relationship
	v.vsToSvcInstance.AddOrUpdate(obj.ConfigMeta.Name, services)
}

func (v *VirtualServiceNSCache) DeleteSvcToVs(vsName string) {
	services := v.GetVSToSVC(vsName)
	for _, service := range services {
		_, vsList := v.svcInstance.Service(v.namespace).GetSvcToVS(service)
		if Contains(vsList, vsName) {
			vsList = Remove(vsList, vsName)
		}
		v.svcInstance.Service(v.namespace).UpdateSvcToVSMapping(service, vsList)
	}
	v.vsToSvcInstance.AddOrUpdate(vsName, services)
}

func (v *VirtualServiceNSCache) GetVSToSVC(vsName string) []string {
	found, services := v.vsToSvcInstance.Get(vsName)
	if !found {
		return make([]string, 0)
	}
	return services.([]string)
}

func (v *VirtualServiceNSCache) DeleteVSToSVC(vsName string) {
	v.vsToSvcInstance.Delete(vsName)
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

func (v *VirtualServiceNSCache) GetServiceForVS(vs *IstioObject) []string {
	// This is a rather complicated method. The Services are found in destinations of various
	// protocol types. For example given a VS object, the relations ship is:
	// vs has TLS or HTTP or TCP. In each HTTP or TLS or TCP we have specific Routes. Inside each route
	// we will find the Destination information for those routes. Implementing this method for HTTP to begin with.
	vsObj, ok := vs.Spec.(*networking.VirtualService)
	if !ok {
		// This is not the right object to cast to VirtualService return error.
		utils.AviLog.Warning.Printf("Wrong object passed. Expecting a Virtual Service object %v", vsObj)
		return nil
	}
	var svcs []string
	if len(vsObj.Http) == 0 {
		utils.AviLog.Warning.Println("There are no HTTP routes found for this Virtual Service")
		return nil
	}
	for _, httpRoute := range vsObj.Http {
		// For each httpRoute, obtain the DestinationRoutes
		if len(httpRoute.Route) == 0 {
			utils.AviLog.Warning.Println("There are no Destination Routes found for this Virtual Service")
			return nil
		}
		for _, HTTPRouteDestinations := range httpRoute.Route {
			// TODO: Take care of subsets
			svcs = append(svcs, HTTPRouteDestinations.Destination.Host)
		}
	}

	return svcs

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
