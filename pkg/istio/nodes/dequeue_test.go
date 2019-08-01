/*
* [2013] - [2019] Avi Networks Incorporated
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

package nodes

import (
	"os"
	"testing"

	"github.com/avinetworks/servicemesh/pkg/istio/objects"
	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/onsi/gomega"
)

var vsLister *objects.VirtualServiceLister
var gwLister *objects.GatewayLister

var drLister *objects.DRLister

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
		{objects.MakeVirtualService("default", "vs_2", 1)},
		{objects.MakeVirtualService("default", "vs_3", 1)},
		{objects.MakeVirtualService("default", "vs_4", 1)},
	}
	for _, pt := range sampleValues {
		vsLister.VirtualService(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
	gwLister = objects.SharedGatewayLister()
	var sampleGWValues = []struct {
		obj_value *objects.IstioObject
	}{
		{objects.MakeGateway("default", "gw_1", 1)},
		{objects.MakeGateway("default", "gw_2", 1)},
	}
	for _, pt := range sampleGWValues {
		gwLister.Gateway(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
}

func TestVSServiceCreate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	gateways, _ := VSToGateway("vs_1", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs := vsLister.VirtualService("default").GetVSToSVC("vs_1")
	expectedSvcs := []string{"reviews", "reviews.prod"}
	g.Expect(svcs).To(gomega.Equal(expectedSvcs))
	gateways, _ = SvcToGateway("reviews", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))

	gateways, _ = VSToGateway("vs_2", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs = vsLister.VirtualService("default").GetVSToSVC("vs_2")
	expectedSvcs = []string{"reviews", "reviews.prod"}
	g.Expect(svcs).To(gomega.Equal(expectedSvcs))
	gateways, _ = SvcToGateway("reviews", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))

	//Testing whether values for a VS which was not created get returned correctly
	gateways, _ = VSToGateway("vs_5", "default")
	g.Expect(gateways).To(gomega.Equal([]string{}))
	svcs = vsLister.VirtualService("default").GetVSToSVC("vs_5")
	g.Expect(svcs).To(gomega.Equal([]string{}))
	gateways, _ = SvcToGateway("", "default")
	g.Expect(gateways).To(gomega.BeNil())

}

func TestVSToGateway(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	// get the associated gateways to the corresponding VS from the VSToGateway() method
	gateways, _ := VSToGateway("vs_1", "default")
	v := vsLister.VirtualService("default")

	// get the gateways associated with the corresponding VS using the GetGatewaysForVS() method
	flag, gws := v.GetGatewaysForVS("vs_1")
	if flag {
		for _, gateway := range gws {
			// range over all the gateways obtained from the GetGatewaysForVS() method and check whether each of those methods are in the gateways variable obtained from the VSToGateway() method
			g.Expect(gateways).To(gomega.ContainElement(gateway))

		}
	} else {
		t.Error("Error occurred")
	}

	g.Expect(len(gateways)).To(gomega.Equal(1))

	// Testing if an empty string array is returned by the VSToGateway() method when we pass a VS which has not been created
	gateways, _ = VSToGateway("vs_5", "default")
	check, gws2 := v.GetGatewaysForVS("vs_5")
	if !check {
		g.Expect(gateways).To(gomega.Equal(gws2))
	}

}

func TestGetGatewayNamespace(t *testing.T) {
	// Testing the GetGatewayNamespace() method for individual gateways
	gatewaynsTest := GetGatewayNamespace("default", "gw_1")
	g := gomega.NewGomegaWithT(t)
	g.Expect(gatewaynsTest).To(gomega.Equal("default"))

	gatewaynsTest2 := GetGatewayNamespace("default", "istio-system/gw_2")
	g.Expect(gatewaynsTest2).To(gomega.Equal("istio-system"))

	// No namespace should be associated with this gateway as this gateway has not been created
	gatewaysnsTest3 := GetGatewayNamespace("", "gw_3")
	g.Expect(gatewaysnsTest3).To(gomega.Equal(""))

	// Testing the GetGatewayNamespace() method when iterating over all the gateways
	allGateways := gwLister.GetAllGateways()
	for gwKey, gwValue := range allGateways {
		for gw := range gwValue {
			g.Expect(GetGatewayNamespace(gwKey, gw)).To(gomega.Equal("default"))

		}
	}
}

func TestSvcToGateway(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	v := vsLister.VirtualService("default")
	svcs := v.GetVSToSVC("vs_1")

	for _, service := range svcs {
		gateways, _ := SvcToGateway(service, "default")
		flag, vs := istio_objs.SharedSvcLister().Service("default").GetSvcToVS(service)
		if flag {
			for _, vsName := range vs {
				check, gateway := v.GetGatewaysForVS(vsName)
				if check {
					for _, gw := range gateway {
						g.Expect(gateways).To(gomega.ContainElement(gw))
					}
				}
			}
		} else {
			t.Error("Error occurred")
		}
	}

	// Checking whether the SvcToGateway() method works correctly when a service which was not created is passed to the method
	svcs = v.GetVSToSVC("vs_5")
	if svcs == nil {
		g.Expect(SvcToGateway("", "default")).To(gomega.BeNil())
	} else {
		for _, service := range svcs {
			gateways, _ := SvcToGateway(service, "default")
			flag, vs := istio_objs.SharedSvcLister().Service("default").GetSvcToVS(service)
			if flag {
				for _, vsName := range vs {
					check, gateway := v.GetGatewaysForVS(vsName)
					if check {
						for _, gw := range gateway {
							g.Expect(gateways).To(gomega.ContainElement(gw))
						}
					}
				}
			} else {
				t.Error("Error occurred")
			}
		}
	}
}

// Testing endpoint to gateway
func TestEPToGateway(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	v := vsLister.VirtualService("default")
	svcs := v.GetVSToSVC("vs_1")

	// Testing whether or not the EPToGateway() method works when we iterate over all the EP's and access the associated gateways
	for _, ep := range svcs {
		gateways, _ := EPToGateway(ep, "default")
		flag, vs := istio_objs.SharedSvcLister().Service("default").GetSvcToVS(ep)
		if flag {
			for _, vsName := range vs {
				check, gateway := v.GetGatewaysForVS(vsName)
				if check {
					for _, gw := range gateway {
						// Checking if each gateway(gw) element is in the gateways object obtained from the EPToGateway() method
						g.Expect(gateways).To(gomega.ContainElement(gw))
					}
				}
			}
		} else {
			// If there are no associated VS's to the Endpoint then there is an error
			t.Error("Error occurred")
		}
	}

	svcs = v.GetVSToSVC("vs_5")

	if len(svcs) == 0 {
		_, found := EPToGateway("", "default")
		g.Expect(found).To(gomega.Equal(false))
	} else {
		for _, ep := range svcs {
			gateways, _ := EPToGateway(ep, "default")
			flag, vs := istio_objs.SharedSvcLister().Service("default").GetSvcToVS(ep)
			if flag {
				for _, vsName := range vs {
					check, gateway := v.GetGatewaysForVS(vsName)
					if check {
						for _, gw := range gateway {
							// Checking if each gateway(gw) element is in the gateways object obtained from the EPToGateway() method
							g.Expect(gateways).To(gomega.ContainElement(gw))
						}
					}
				}
			} else {
				// If there are no associated VS's to the Endpoint then there is an error
				t.Error("Error occurred")
			}
		}
	}
}

func TestGatewayChanges(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Checking whether or not a current Gateway is added/exists in the list of Gateways
	gateways, _ := DetectGatewayChanges("gw_1", "default")
	g.Expect(gateways).To(gomega.ContainElement("gw_1"))

	//Deleting a current Gateway and checking if the DetectGatewayChanges() method returns nil or not
	v := gwLister.Gateway("default")
	if v.Delete("gw_1") {
		_, found := DetectGatewayChanges("gw_1", "default")
		g.Expect(found).To(gomega.Equal(false))
	}

	//Checking if the DetectGatewayChanges() method returns nil on checking if there is a Gateway which doesn't exist
	_, found := DetectGatewayChanges("gw_3", "default")
	g.Expect(found).To(gomega.Equal(false))
}

func TestVSServiceDelete(t *testing.T) {
	// First delete it from the store - simulating the Ingestion Layer function.
	vsLister.VirtualService("default").Delete("vs_1")
	g := gomega.NewGomegaWithT(t)
	gateways, _ := VSToGateway("vs_1", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs := vsLister.VirtualService("default").GetVSToSVC("vs_1")
	// We don't expect the relationship to exist anymore.
	g.Expect(len(svcs)).To(gomega.Equal(0))
	gateways, _ = SvcToGateway("reviews", "default")
	g.Expect(len(gateways)).To(gomega.Equal(1))

	vsLister.VirtualService("default").Delete("vs_2")
	gateways, _ = VSToGateway("vs_2", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs = vsLister.VirtualService("default").GetVSToSVC("vs_2")
	g.Expect(len(svcs)).To(gomega.Equal(0))
	// Now the service will not be able to trace to the gateway
	gateways, _ = SvcToGateway("reviews", "default")
	g.Expect(len(gateways)).To(gomega.Equal(0))

	// Testing whether or not the correct values are returned when one tries to delete a VS which has already been deleted
	emptyStringArray := []string{}
	vsLister.VirtualService("default").Delete("vs_2")
	gateways, _ = VSToGateway("vs_2", "default")
	g.Expect(gateways).To(gomega.Equal(emptyStringArray))
	svcs = vsLister.VirtualService("default").GetVSToSVC("vs_2")
	g.Expect(len(svcs)).To(gomega.Equal(0))
	gateways, _ = SvcToGateway("reviews", "default")
	g.Expect(len(gateways)).To(gomega.Equal(0))

}
