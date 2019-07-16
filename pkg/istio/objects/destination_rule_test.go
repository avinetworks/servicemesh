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

	"github.com/onsi/gomega"
)

func TestGetDRObject(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	drLister = SharedDRLister()
	drObj := Make("default", "dr_1", 1)
	drObj1 := Make("default", "dr_2", 1)
	drLister.DestinationRule("default").Update(drObj)
	drLister.DestinationRule("default").Update(drObj1)
	dr := drLister.DestinationRule("default").GetAllDRNameVers()
	drList := []string{}
	for key := range dr {
		drList = append(drList, key)
	}
	g.Expect(drList).To(gomega.ContainElement("dr_1"))
	g.Expect(len(drList)).To(gomega.Equal(2))
}

func TestGetDRListMisc(t *testing.T) {
	drLister = SharedDRLister()
	_, dr_obj := drLister.DestinationRule("default").Get("dr_1")
	if dr_obj == nil || dr_obj.ConfigMeta.Name != "dr_1" && dr_obj.ConfigMeta.ResourceVersion != "1" {
		t.Errorf("TestGetDRListMisc failed to get the expected object, obtained :%s", dr_obj.ConfigMeta.Name)
	}
	_, dr_obj = drLister.DestinationRule("default").Get("dr_2")
	if dr_obj == nil || dr_obj.ConfigMeta.Name != "dr_2" && dr_obj.ConfigMeta.ResourceVersion != "1" {
		t.Errorf("TestGetDRListMisc failed to get the expected object, obtained :%s", dr_obj.ConfigMeta.Name)
	}

	dr_objs := drLister.DestinationRule("default").List()
	if len(dr_objs) != 2 {
		t.Errorf("TestGetDRListMisc failed to get the expected object, obtained :%d", len(dr_objs))
	}

	drLister.DestinationRule("default").Delete("dr_2")

	// After deleting one of the DR objects ("dr_2") the object should not be in the list
	_, dr_obj = drLister.DestinationRule("default").Get("dr_2")
	if dr_obj != nil {
		t.Errorf("TestGetDRListMisc failed to gert the expected object, obtained: %s", dr_obj.ConfigMeta.Name)
	}
	dr_objs = drLister.DestinationRule("default").List()
	// dr_2 has been deleted and hence the length of thr total number of dr objects must be 1
	if len(dr_objs) != 1 {
		t.Errorf("TestGetDRListMisc failed to get the expected object, obtained :%d", len(dr_objs))
	}
}

func TestGetAllDRs(t *testing.T) {
	drMap := drLister.GetAllDRs()
	if len(drMap["default"]) != 1 {
		t.Errorf("TestGetAllDRs failed to get the expected object, obtained :%s", (drMap["default"]))
	}
	if drMap["default"]["dr_1"] != "1" {
		t.Errorf("TestGetAllDRs failed to get the expected object, obtained :%s", drMap["default"]["dr_1"])
	}

	// Adding additional objects to test the GetAllDRs() method
	drObj := Make("default", "dr_2", 1)
	drObj1 := Make("red", "dr_3", 1)
	drLister.DestinationRule("default").Update(drObj)
	drLister.DestinationRule("red").Update(drObj1)
	drMap = drLister.GetAllDRs()

	// After adding an additional DR object to the list in the default namespace, the length of all objects under the default namespace must be 2
	if len(drMap["default"]) != 2 {
		t.Errorf("TestGetAllDRs failed to get the expected object, obtained :%s", (drMap["default"]))
	}

	// After adding a DR object belonging to the red namespace, the length of all objects under the red namespace must be 1
	if len(drMap["red"]) != 1 {
		t.Errorf("TestGetAllDRs failed to get the expected object, obtained :%s", (drMap["red"]))
	}
}
