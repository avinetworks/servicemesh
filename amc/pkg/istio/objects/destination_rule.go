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

//This module should give a wrapper over underlying objects to give easy API access for DR.

package objects

import (
	"sync"

	"github.com/avinetworks/servicemesh/utils"
	networking "istio.io/api/networking/v1alpha3"
)

var drlisterinstance *DRLister
var dronce sync.Once

func SharedDRLister() *DRLister {
	dronce.Do(func() {
		DRStore := NewObjectStore()
		drSvcStore := NewObjectStore()
		drlisterinstance = &DRLister{}
		drlisterinstance.drsvcstore = drSvcStore
		drlisterinstance.drstore = DRStore
	})
	return drlisterinstance
}

type IstioDR interface {
	DestinationRule(ns string) DRNameSpaceIntf
	//List() *[]ObjectStore
}

type DRLister struct {
	drstore    *ObjectStore
	drsvcstore *ObjectStore
}

func (v *DRLister) DestinationRule(ns string) *DRNSCache {
	nsDRObjects := v.drstore.GetNSStore(ns)
	nsDRSvcObjects := v.drsvcstore.GetNSStore(ns)
	svcInstance := SharedSvcLister()
	return &DRNSCache{namespace: ns, drobjects: nsDRObjects, drsvcobjects: nsDRSvcObjects, svcInstance: svcInstance}
}

func (v *DRLister) GetAllDRs() map[string]map[string]string {
	// This method should return a map that looks like this: {ns: [obj1, obj2]}
	// This is particularly useful if we want to know what are the DR names
	// present in a namespace without affecting the actual store objects.
	allNamespaces := v.drstore.GetAllNamespaces()
	allDRs := make(map[string]map[string]string)
	if len(allNamespaces) != 0 {
		// Iterate over each namespace and formulate the map
		for _, ns := range allNamespaces {
			allDRs[ns] = v.DestinationRule(ns).GetAllDRNameVers()
		}
	}
	return allDRs

}

type DRNameSpaceIntf interface {
	Get(name string) (bool, *IstioObject)
	Update(obj IstioObject) bool
	List() map[string]*IstioObject
	Delete(name string) bool
}

type DRNSCache struct {
	namespace    string
	drobjects    *ObjectMapStore
	drsvcobjects *ObjectMapStore
	svcInstance  *SvcLister
}

func (v *DRNSCache) GetAllDRNameVers() map[string]string {
	// Obtain the object for this DR
	allObjects := v.drobjects.GetAllObjectNames()
	objVersionsMap := make(map[string]string)
	// Now let's parse the object names and their corresponding resourceversions in a Map
	for _, obj := range allObjects {
		objVersionsMap[obj.(*IstioObject).ConfigMeta.Name] = obj.(*IstioObject).ConfigMeta.ResourceVersion
	}
	return objVersionsMap
}

func (v *DRNSCache) Get(name string) (bool, *IstioObject) {
	found, obj := v.drobjects.Get(name)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.(*IstioObject)
	}
}

func (v *DRNSCache) GetSVCMapping(drname string) (bool, []string) {
	found, obj := v.drsvcobjects.Get(drname)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.([]string)
	}
}

func (v *DRNSCache) Update(obj *IstioObject) {
	v.drobjects.AddOrUpdate(obj.Name, obj)
}

func (v *DRNSCache) UpdateDRToSVCMapping(drName string, svc string) {
	v.drsvcobjects.AddOrUpdate(drName, svc)
}

func (v *DRNSCache) Delete(name string) bool {
	return v.drobjects.Delete(name)
}

func (v *DRNSCache) List() map[string]*IstioObject {
	// TODO (sudswas): Let's check if we can abstract out the store objects
	// completely. There's still a possibility that if we pass the references
	// we maybe allowing upper layers to modify the object that would directly
	// impact the store objects.
	convertedMap := make(map[string]*IstioObject)
	// Change the empty interface to IstioObject. Avoid Duck Typing.
	for key, value := range v.drobjects.ObjectMap {
		convertedMap[key] = value.(*IstioObject)
	}
	return convertedMap
}

func (v *DRNSCache) GetServiceFromDRObj(dr *IstioObject) string {
	drObj, ok := dr.Spec.(*networking.DestinationRule)
	if !ok {
		// This is not the right object to cast to VirtualService return error.
		utils.AviLog.Warning.Printf("Wrong object passed. Expecting a DestinationRule object %v", drObj)
		return ""
	}
	if len(drObj.Host) == 0 {
		utils.AviLog.Warning.Println("There are no Services found for this Destination Rule")
		return ""
	}

	return drObj.Host

}

func (v *DRNSCache) UpdateSvcDRRefs(obj *IstioObject) {
	var drList []string
	service := v.GetServiceFromDRObj(obj)
	_, drList = v.svcInstance.Service(obj.ConfigMeta.Namespace).GetSvcToDR(service)
	if !utils.HasElem(drList, obj.ConfigMeta.Name) {
		drList = append(drList, obj.ConfigMeta.Name)
	}
	v.svcInstance.Service(obj.ConfigMeta.Namespace).UpdateSvcToDR(service, drList)
	utils.AviLog.Info.Printf("The Service associated with this DR: %s is %s ", obj.ConfigMeta.Name, service)
	v.UpdateDRToSVCMapping(obj.ConfigMeta.Name, service)
}

func (v *DRNSCache) GetSvcForDR(drName string) (bool, string) {
	// Need checks if it's found or not?
	found, svc := v.drsvcobjects.Get(drName)
	if !found {
		return false, ""
	}
	return true, svc.(string)
}

func (v *DRNSCache) DeleteDRToSvc(drName string) {
	// Need checks if it's found or not?
	v.drsvcobjects.Delete(drName)
}

func (v *DRNSCache) DeleteSVCToDRRefs(drName string, namespace string) {
	// Need checks if it's found or not?
	found, svc := v.drsvcobjects.Get(drName)
	if found {
		var drList []string
		var ok bool
		ok, drList = v.svcInstance.Service(namespace).GetSvcToDR(svc.(string))
		if ok {
			drList = Remove(drList, drName)
			v.svcInstance.Service(namespace).UpdateSvcToDR(svc.(string), drList)
		}
	}
	v.DeleteDRToSvc(drName)
}
