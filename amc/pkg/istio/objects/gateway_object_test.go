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

import "testing"

func TestGetGatewayObject(t *testing.T) {
	gwLister := SharedGatewayLister()
	gwObj := MakeGateway("default", "gw_1", 1)
	gwLister.Gateway("default").Update(gwObj)
	_, gw_obj := gwLister.Gateway("default").Get("gw_1")
	if gw_obj != nil && gw_obj.ConfigMeta.Name != "gw_1" {
		t.Errorf("TestGetGatewayObject failed to get the expected object, obtained :%s", gw_obj.ConfigMeta.Name)
	}
}

func TestGetGatewayListMisc(t *testing.T) {
	gwLister := SharedGatewayLister()
	_, gw_obj := gwLister.Gateway("default").Get("gw_1")
	if gw_obj == nil || gw_obj.ConfigMeta.Name != "gw_1" && gw_obj.ConfigMeta.ResourceVersion != "1" {
		t.Errorf("TestGetGatewayListMisc failed to get the expected object, obtained :%s", gw_obj.ConfigMeta.Name)
	}
	_, gw_obj = gwLister.Gateway("red").Get("gw_2")
	if gw_obj == nil || gw_obj.ConfigMeta.Name != "gw_1" && gw_obj.ConfigMeta.ResourceVersion != "3" {
		t.Errorf("TestGetGatewayListMisc failed to get the expected object, obtained :%s", gw_obj)
	}
	gwLister.Gateway("red").Delete("gw_2")
	_, gw_obj = gwLister.Gateway("red").Get("gw_2")
	if gw_obj != nil {
		t.Errorf("TestGetGatewayListMisc failed to get the expected object, obtained :%s", gw_obj.ConfigMeta.Name)
	}
	// Red had 3 objects, we deleted one above, so it should have 2
	gw_objs := gwLister.Gateway("red").List()
	if len(gw_objs) != 2 {
		t.Errorf("TestGetGatewayListMisc failed to get the expected object, obtained :%d", len(gw_objs))
	}
}

func TestGetAllGateways(t *testing.T) {
	gwMap := gwLister.GetAllGateways()
	if len(gwMap["default"]) != 4 && len(gwMap["red"]) != 2 {
		t.Errorf("TestGetAllGateways failed to get the expected gwMap: %s", gwMap["default"])
	}
	if gwMap["default"]["gw_2"] != "3" {
		t.Errorf("TestGetAllGateways failed to get the expected Resource version: %s", gwMap["default"]["vs_2"])
	}
}
