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
	"os"
	"testing"
)

var vsLister *VirtualServiceLister
var gwLister *GatewayLister

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	// If clean ups are needed later.
	//shutdown()
	os.Exit(code)
}

func setup() {
	// Use this method to populate some VS objects
	vsLister = SharedVirtualServiceLister()
	var sampleValues = []struct {
		obj_value *IstioObject
	}{
		{MakeVirtualService("default", "vs_1", 1)},
		{MakeVirtualService("red", "vs_2", 2)},
		{MakeVirtualService("default", "vs_2", 3)},
		{MakeVirtualService("red", "vs_3", 1)},
		{MakeVirtualService("default", "vs_3", 2)},
		{MakeVirtualService("red", "vs_4", 1)},
		{MakeVirtualService("default", "vs_5", 1)},
		{MakeVirtualService("red", "vs_2", 3)},
	}
	for _, pt := range sampleValues {
		vsLister.VirtualService(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
	// Use this method to populate some VS objects
	gwLister = SharedGatewayLister()
	var sampleGWValues = []struct {
		obj_value *IstioObject
	}{
		{MakeGateway("default", "gw_1", 1)},
		{MakeGateway("red", "gw_2", 2)},
		{MakeGateway("default", "gw_2", 3)},
		{MakeGateway("red", "gw_3", 1)},
		{MakeGateway("default", "gw_3", 2)},
		{MakeGateway("red", "gw_4", 1)},
		{MakeGateway("default", "gw_5", 1)},
		{MakeGateway("red", "gw_2", 3)},
	}
	for _, pt := range sampleGWValues {
		gwLister.Gateway(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
}
func TestGetVirtualServiceObject(t *testing.T) {
	vsLister := SharedVirtualServiceLister()
	vsObj := MakeVirtualService("default", "vs_1", 1)
	vsLister.VirtualService("default").Update(vsObj)
	_, vs_obj := vsLister.VirtualService("default").Get("vs_1")
	if vs_obj != nil && vs_obj.ConfigMeta.Name != "vs_1" {
		t.Errorf("TestGetVirtualServiceObject failed to get the expected object, obtained :%s", vs_obj.ConfigMeta.Name)
	}
}

func TestGetVirtualServiceListMisc(t *testing.T) {
	vsLister := SharedVirtualServiceLister()
	_, vs_obj := vsLister.VirtualService("default").Get("vs_1")
	if vs_obj == nil || vs_obj.ConfigMeta.Name != "vs_1" && vs_obj.ConfigMeta.ResourceVersion != "1" {
		t.Errorf("TestGetVirtualServiceListMisc failed to get the expected object, obtained :%s", vs_obj.ConfigMeta.Name)
	}
	_, vs_obj = vsLister.VirtualService("red").Get("vs_2")
	if vs_obj == nil || vs_obj.ConfigMeta.Name != "vs_2" && vs_obj.ConfigMeta.ResourceVersion != "3" {
		t.Errorf("TestGetVirtualServiceListMisc failed to get the expected object, obtained :%s", vs_obj.ConfigMeta.Name)
	}
	vsLister.VirtualService("red").Delete("vs_2")
	_, vs_obj = vsLister.VirtualService("red").Get("vs_2")
	if vs_obj != nil {
		t.Errorf("TestGetVirtualServiceListMisc failed to get the expected object, obtained :%s", vs_obj.ConfigMeta.Name)
	}
	// Red had 3 objects, we deleted one above, so it should have 2
	vs_objs := vsLister.VirtualService("red").List()
	if len(vs_objs) != 2 {
		t.Errorf("TestGetVirtualServiceListMisc failed to get the expected object, obtained :%d", len(vs_objs))
	}
}

// func TestPopulateGatewayRelationships(t *testing.T) {
// 	// Check if the gateway relationship was created
// 	gw_instance := SharedGatewayLister()
// 	_, vslist := gw_instance.Gateway("default").GetVSMapping("gw1")
// 	// We should get two VSes.
// 	if len(vslist) != 4 {
// 		t.Errorf("TestPopulateGatewayRelationships failed to get the expected VSes, obtained :%d", len(vslist))
// 	}
// 	_, vslist = gw_instance.Gateway("red").GetVSMapping("gw1")
// 	if len(vslist) != 3 {
// 		t.Errorf("TestPopulateGatewayRelationships failed to get the expected , obtained :%d", len(vslist))
// 	}
// }

func TestGetAllVirtualServices(t *testing.T) {
	vsMap := vsLister.GetAllVirtualServices()
	if len(vsMap["default"]) != 4 && len(vsMap["red"]) != 2 {
		t.Errorf("TestGetAllVirtualServices failed to get the expected VSes: %s", vsMap["default"])
	}
	if vsMap["default"]["vs_2"] != "3" {
		t.Errorf("TestGetAllVirtualServices failed to get the expected Resource version: %s", vsMap["default"]["vs_2"])
	}
}
