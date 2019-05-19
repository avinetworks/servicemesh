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

	"github.com/avinetworks/servicemesh/pkg/istio/objects"
)

var vsLister *objects.VirtualServiceLister

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
}

func GetConfigDescriptors() ConfigDescriptor {
	return IstioConfigTypes
}

func TestCalculateUpdates(t *testing.T) {
	schema, _ := GetConfigDescriptors().GetByType("virtual-service")
	oldStore := schema.GetAll()
	newObj := objects.MakeVirtualService("default", "vs_2", 5)
	//vsLister.VirtualService(newObj.ConfigMeta.Namespace).Update(newObj)
	schema.Store("vs_2", "default", newObj.ConfigMeta, newObj.Spec)
	newStore := schema.GetAll()
	changedKeys := GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an UPDATE
	if len(changedKeys) != 1 && changedKeys[0] != "vs_2" {
		t.Errorf("TestCalculateUpdates UPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
	// Let's swap the variables
	oldStore = newStore
	vsLister.VirtualService("default").Delete("vs_2")
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is a DELETE event
	if len(changedKeys) != 1 && changedKeys[0] != "vs_2" {
		t.Errorf("TestCalculateUpdates DELETE failed to get the expected object, obtained :%s", changedKeys)
	}
	oldStore = newStore
	newObj = objects.MakeVirtualService("default", "vs_2", 5)
	schema.Store("vs_2", "default", newObj.ConfigMeta, newObj.Spec)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys) != 1 && changedKeys[0] != "vs_2" {
		t.Errorf("TestCalculateUpdates ADD failed to get the expected object, obtained :%s", changedKeys)
	}
	newObj = objects.MakeVirtualService("default", "vs_2", 5)
	oldStore = newStore
	schema.Store("vs_2", "default", newObj.ConfigMeta, newObj.Spec)
	newStore = schema.GetAll()
	changedKeys = GetConfigDescriptors().CalculateUpdates(oldStore, newStore)
	// This is an ADD event
	if len(changedKeys) != 0 {
		t.Errorf("TestCalculateUpdates NOUPDATE failed to get the expected object, obtained :%s", changedKeys)
	}
}
