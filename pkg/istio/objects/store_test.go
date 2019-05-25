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
	"testing"
	"time"
)

func TestUpdateObjInNamespaceGlobalLock(t *testing.T) {
	newStore := NewObjectStore()
	var sampleValues = []struct {
		obj_value *IstioObject
	}{
		{Make("default", "vs_1", 1)},
		{Make("red", "vs_2", 2)},
	}
	for _, pt := range sampleValues {
		newStore.UpdateNSStore(pt.obj_value)
	}

	nsHandle := newStore.GetNSStore("default")
	_, vs_obj := nsHandle.Get("vs_1")
	if vs_obj == nil || vs_obj.(*IstioObject).ConfigMeta.Name != "vs_1" {
		t.Errorf("TestUpdateObjInNamespace failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
	// We should get a nil object for below.
	_, vs_obj = nsHandle.Get("vs_2")
	if vs_obj != nil {
		t.Errorf("TestUpdateObjInNamespace failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
	nsHandle = newStore.GetNSStore("red")
	_, vs_obj = nsHandle.Get("vs_2")
	if vs_obj == nil || vs_obj.(*IstioObject).ConfigMeta.Name != "vs_2" {
		t.Errorf("TestUpdateObjInNamespace failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
	// We should get a nil object for below.
	_, vs_obj = nsHandle.Get("vs_1")
	if vs_obj != nil {
		t.Errorf("TestUpdateObjInNamespace failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
}

func TestParallelUpdateStore(t *testing.T) {
	newStore := NewObjectStore()
	var sampleValuesRoutine1 = []struct {
		obj_value *IstioObject
	}{
		{Make("default", "vs_1", 1)},
		{Make("red", "vs_2", 2)},
		{Make("red", "vs_3", 2)},
		{Make("red", "vs_4", 2)},
		{Make("red", "vs_5", 2)},
		{Make("red", "vs_6", 2)},
		{Make("red", "vs_7", 2)},
		{Make("red", "vs_8", 2)},
	}
	var sampleValuesRoutine2 = []struct {
		obj_value *IstioObject
	}{
		{Make("default", "vs_3", 1)},
		{Make("red", "vs_2", 2)},
		{Make("red", "vs_3", 2)},
		{Make("red", "vs_4", 2)},
		{Make("red", "vs_5", 2)},
		{Make("red", "vs_6", 2)},
		{Make("red", "vs_7", 2)},
		{Make("red", "vs_8", 2)},
		{Make("red", "vs_9", 2)},
		{Make("red", "vs_10", 2)},
		{Make("red", "vs_11", 2)},
		{Make("red", "vs_12", 2)},
		{Make("red", "vs_13", 2)},
		{Make("red", "vs_14", 2)},
	}
	go UpdateStore(sampleValuesRoutine1, newStore)
	go UpdateStore(sampleValuesRoutine2, newStore)
	//Assuming the go routines would be done in this time.
	time.Sleep(100 * time.Millisecond)
	nsHandle := newStore.GetNSStore("red")
	objs := nsHandle.GetAllObjectNames()
	if len(objs) != 13 {
		t.Errorf("TestParallelUpdateStore failed to get the expected object in red ns , obtained :%v", len(objs))
	}
	nsHandle = newStore.GetNSStore("default")
	objs = nsHandle.GetAllObjectNames()
	if len(objs) != 2 {
		t.Errorf("TestParallelUpdateStore failed to get the expected object in default ns, obtained :%v", len(objs))
	}
}

func UpdateStore(sampleValues []struct{ obj_value *IstioObject }, newStore *ObjectStore) {
	for _, pt := range sampleValues {
		newStore.UpdateNSStore(pt.obj_value)
	}
}

func TestUpdateDeleteObjInNamespaceLocallLock(t *testing.T) {
	newStore := NewObjectStore()
	var sampleValuesDefault = []struct {
		obj_value *IstioObject
	}{
		{Make("default", "vs_1", 1)},
		{Make("default", "vs_2", 2)},
	}
	nsHandle := newStore.GetNSStore("default")
	for _, pt := range sampleValuesDefault {
		nsHandle.AddOrUpdate(pt.obj_value.Name, pt.obj_value)
	}

	var sampleValuesRed = []struct {
		obj_value *IstioObject
	}{
		{Make("red", "vs_1", 1)},
		{Make("red", "vs_2", 2)},
	}
	nsHandle = newStore.GetNSStore("red")
	for _, pt := range sampleValuesRed {
		nsHandle.AddOrUpdate(pt.obj_value.Name, pt.obj_value)
	}

	nsHandle = newStore.GetNSStore("red")
	_, vs_obj := nsHandle.Get("vs_2")
	if vs_obj == nil || vs_obj.(*IstioObject).ConfigMeta.Name != "vs_2" {
		t.Errorf("TestUpdateDeleteObjInNamespaceLocallLock failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
	// We should get a nil object for below.
	_, vs_obj = nsHandle.Get("vs_3")
	if vs_obj != nil {
		t.Errorf("TestUpdateDeleteObjInNamespaceLocallLock failed to get the expected object, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
	}
	//Let's delete some entries from red.
	ok := nsHandle.Delete("vs_2")
	if ok {
		_, vs_obj := nsHandle.Get("vs_2")
		if vs_obj != nil {
			t.Errorf("TestUpdateDeleteObjInNamespaceLocallLock obtained object for vs_2 :%s", vs_obj.(*IstioObject).ConfigMeta.Name)
		}
	}
	// Let's check if the resourceVersions are updated and we do not get older objects
	var sampleValues = []struct {
		obj_value *IstioObject
	}{
		{Make("default", "vs_2", 3)},
		{Make("red", "vs_1", 4)},
	}
	for _, pt := range sampleValues {
		newStore.UpdateNSStore(pt.obj_value)
	}
	// nsHandle belongs to redNamespace
	_, vs_obj = nsHandle.Get("vs_1")
	if vs_obj == nil || vs_obj.(*IstioObject).ConfigMeta.ResourceVersion != "4" {
		t.Errorf("TestUpdateDeleteObjInNamespaceLocallLock failed to get the expected ResourceVersion, obtained :%s", vs_obj.(*IstioObject).ConfigMeta.ResourceVersion)
	}

}
