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

var drLister *DRLister

func TestGetServiceObject(t *testing.T) {

	g := gomega.NewGomegaWithT(t)
	svcLister := SharedSvcLister()
	drLister = SharedDRLister()
	drObj := Make("default", "dr_1", 1)
	VSObj := MakeVirtualService("default", "vs_1", 1)
	drLister.DestinationRule("default").Update(drObj)
	vsVers := vsLister.VirtualService("default").GetAllVSNamesVers()
	// Obtaining all the VS objects in the default namespace
	vsList := []string{}
	for key := range vsVers {
		vsList = append(vsList, key)
	}
	dr := drLister.DestinationRule("default").GetAllDRNameVers()
	// Obtaining all the DR objects in the default namespace
	drList := []string{}
	for key := range dr {
		drList = append(drList, key)
	}

	svcLister.Service("default").UpdateSvcToDR("reviews", drList)
	vsLister.VirtualService("default").UpdateSvcVSRefs(VSObj)
	drLister.DestinationRule("default").UpdateDRToSVCMapping("dr_1", "reviews")
	// Obtaining the services associated with the Virtual Service vs_1
	svc_obj := vsLister.VirtualService("default").GetVSToSVC("vs_1")

	// Obtaining all the Services associated with the DR List
	svc_obj1 := []string{}
	for _, val := range drList {
		_, svcName := drLister.DestinationRule("default").GetSvcForDR(val)
		svc_obj1 = append(svc_obj1, svcName)
	}

	// Both the VS and DR must contain the Service "reviews"
	g.Expect(svc_obj).To(gomega.ContainElement("reviews"))
	g.Expect(svc_obj1).To(gomega.ContainElement("reviews"))
}

func TestGetAllSvcs(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	// Obtaining all the services associated with the VS vs_1
	svcs1 := vsLister.VirtualService("default").GetVSToSVC("vs_1")
	svcs2 := []string{}
	dr := drLister.DestinationRule("default").GetAllDRNameVers()
	drList := []string{}
	for key := range dr {
		drList = append(drList, key)
	}

	// Obtaining all the services associated with the DR's in the default namespace
	for _, val := range drList {
		_, svcName := drLister.DestinationRule("default").GetSvcForDR(val)
		svcs2 = append(svcs2, svcName)
	}
	// Since there are 2 services associated with the VS and only 1 associated with the DR, the service associated with the DR must be present in the services associated with the VS vs_1
	g.Expect(svcs1).To(gomega.ContainElement(svcs2[0]))
}
