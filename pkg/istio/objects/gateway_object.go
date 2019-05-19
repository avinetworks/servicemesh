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

//This module should give a wrapper over underlying objects to give easy API access for Gateway.

package objects

import (
	"sync"
)

var gwlisterinstance *GatewayLister
var gwonce sync.Once

func SharedGatewayLister() *GatewayLister {
	gwonce.Do(func() {
		GWStore := NewObjectStore()
		gwvsStore := NewObjectStore()
		gwlisterinstance = &GatewayLister{}
		gwlisterinstance.gwvsstore = gwvsStore
		gwlisterinstance.gwstore = GWStore
	})
	return gwlisterinstance
}

type IstioGateway interface {
	Gateway(ns string) GatewayNameSpaceIntf
	//List() *[]ObjectStore
}

type GatewayLister struct {
	gwstore   *ObjectStore
	gwvsstore *ObjectStore
}

func (v *GatewayLister) Gateway(ns string) *GatewayNSCache {
	nsGwObjects := v.gwstore.GetNSStore(ns)
	nsGwVsObjects := v.gwvsstore.GetNSStore(ns)
	return &GatewayNSCache{namespace: ns, gwobjects: nsGwObjects, gwvsobjects: nsGwVsObjects}
}

type GatewayNameSpaceIntf interface {
	Get(name string) (bool, *IstioObject)
	Update(obj IstioObject) bool
	List() map[string]*IstioObject
	Delete(name string) bool
}

type GatewayNSCache struct {
	namespace   string
	gwobjects   *ObjectMapStore
	gwvsobjects *ObjectMapStore
}

func (v *GatewayNSCache) Get(name string) (bool, *IstioObject) {
	found, obj := v.gwobjects.Get(name)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.(*IstioObject)
	}
}

func (v *GatewayNSCache) GetVSMapping(gwname string) (bool, []string) {
	found, obj := v.gwvsobjects.Get(gwname)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.([]string)
	}
}

func (v *GatewayNSCache) Update(obj *IstioObject) {
	v.gwobjects.AddOrUpdate(obj.Name, obj)
}

func (v *GatewayNSCache) UpdateGWVSMapping(gwName string, vsList []string) {
	v.gwvsobjects.AddOrUpdate(gwName, vsList)
}
