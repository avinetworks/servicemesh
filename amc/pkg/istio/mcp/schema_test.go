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
	"os"
	"testing"

	"github.com/avinetworks/servicemesh/amc/pkg/istio/objects"
	istio_objs "github.com/avinetworks/servicemesh/amc/pkg/istio/objects"
)

var vsLister *objects.VirtualServiceLister
var gwLister *objects.GatewayLister

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	// If clean ups are needed later.
	//shutdown()
	os.Exit(code)
}

func setup() {
	// Use this method to populate some VS objects
	vsLister = objects.SharedVirtualServiceLister()
	var sampleValues = []struct {
		obj_value *objects.IstioObject
	}{
		{objects.MakeVirtualService("default", "vs_1", 1)},
		{objects.MakeVirtualService("red", "vs_2", 2)},
		{objects.MakeVirtualService("default", "vs_2", 3)},
		{objects.MakeVirtualService("red", "vs_3", 1)},
		{objects.MakeVirtualService("default", "vs_3", 2)},
		{objects.MakeVirtualService("red", "vs_4", 1)},
		{objects.MakeVirtualService("default", "vs_5", 1)},
		{objects.MakeVirtualService("red", "vs_2", 3)},
	}
	for _, pt := range sampleValues {
		vsLister.VirtualService(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
	gwLister = objects.SharedGatewayLister()
	var sampleGWValues = []struct {
		obj_value *objects.IstioObject
	}{
		{objects.MakeGateway("default", "gw_1", 1)},
		{objects.MakeGateway("red", "gw_2", 2)},
		{objects.MakeGateway("default", "gw_2", 3)},
		{objects.MakeGateway("red", "gw_3", 1)},
		{objects.MakeGateway("default", "gw_3", 2)},
		{objects.MakeGateway("red", "gw_4", 1)},
		{objects.MakeGateway("default", "gw_5", 1)},
		{objects.MakeGateway("red", "gw_2", 3)},
	}
	for _, pt := range sampleGWValues {
		gwLister.Gateway(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
}

func GetConfigDescriptors() ConfigDescriptor {
	return IstioConfigTypes
}

func TestCalculateUpdatesVS(t *testing.T) {
	schema, _ := GetConfigDescriptors().GetByType("virtual-service")
	oldStore := schema.GetAll()
	newObj := objects.MakeVirtualService("default", "vs_2", 5)
	//vsLister.VirtualService(newObj.ConfigMeta.Namespace).Update(newObj)
	presentValues := make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"vs_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore := schema.GetAll()
	changedKeys := GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an UPDATE
	if len(changedKeys["default"]) != 4 {
		t.Errorf("TestCalculateUpdatesVS UPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
	// Let's swap the variables
	oldStore = newStore
	vsLister.VirtualService("default").Delete("vs_2")
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is a DELETE event
	if len(changedKeys["default"]) != 1 && changedKeys["default"][0] != "vs_2" {
		t.Errorf("TestCalculateUpdatesVS DELETE failed to get the expected object, obtained :%s", changedKeys)
	}
	oldStore = newStore
	newObj = objects.MakeVirtualService("default", "vs_2", 5)
	presentValues = make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"vs_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys["default"]) != 1 && changedKeys["default"][0] != "vs_2" {
		t.Errorf("TestCalculateUpdatesVS ADD failed to get the expected object, obtained :%s", changedKeys)
	}
	newObj = objects.MakeVirtualService("default", "vs_2", 5)
	oldStore = newStore
	presentValues = make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"vs_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys["default"]) != 0 {
		t.Errorf("TestCalculateUpdatesVS NOUPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
}

func TestCalculateUpdatesGW(t *testing.T) {
	schema, _ := GetConfigDescriptors().GetByType("gateway")
	oldStore := schema.GetAll()
	newObj := objects.MakeGateway("default", "gw_2", 5)
	presentValues := make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"gw_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore := schema.GetAll()
	changedKeys := GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an UPDATE
	if len(changedKeys["default"]) != 4 {
		t.Errorf("TestCalculateUpdatesGW UPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
	// Let's swap the variables
	oldStore = newStore
	gwLister.Gateway("default").Delete("gw_2")
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is a DELETE event
	if len(changedKeys["default"]) != 1 && changedKeys["default"][0] != "gw_2" {
		t.Errorf("TestCalculateUpdatesGW DELETE failed to get the expected object, obtained :%s", changedKeys)
	}
	oldStore = newStore
	newObj = objects.MakeGateway("default", "gw_2", 5)
	presentValues = make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"gw_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys["default"]) != 1 && changedKeys["default"][0] != "gw_2" {
		t.Errorf("TestCalculateUpdatesGW ADD failed to get the expected object, obtained :%s", changedKeys)
	}
	newObj = objects.MakeGateway("default", "gw_2", 5)
	oldStore = newStore
	presentValues = make(map[string]map[string]*istio_objs.IstioObject)
	presentValues["default"] = map[string]*istio_objs.IstioObject{"gw_2": newObj}
	schema.Store(presentValues, oldStore)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys["default"]) != 0 {
		t.Errorf("TestCalculateUpdatesGW NOUPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
}
