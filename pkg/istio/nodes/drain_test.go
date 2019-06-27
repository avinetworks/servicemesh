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
	"github.com/onsi/gomega"
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
	}
	for _, pt := range sampleGWValues {
		gwLister.Gateway(pt.obj_value.ConfigMeta.Namespace).Update(pt.obj_value)
	}
}

func TestVSServiceCreate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	gateways := VSToGateway("vs_1", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs := vsLister.VirtualService("default").GetVSToSVC("vs_1")
	expectedSvcs := []string{"reviews", "reviews.prod"}
	g.Expect(svcs).To(gomega.Equal(expectedSvcs))
	gateways = SvcToGateway("reviews", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
}

func TestVSServiceDelete(t *testing.T) {
	// First delete it from the store - simulating the Ingestion Layer function.
	vsLister.VirtualService("default").Delete("vs_1")
	g := gomega.NewGomegaWithT(t)
	gateways := VSToGateway("vs_1", "default")
	g.Expect(gateways).To(gomega.ContainElement("ns/gw1"))
	svcs := vsLister.VirtualService("default").GetVSToSVC("vs_1")
	// We don't expect the relationship to exist anymore.
	g.Expect(len(svcs)).To(gomega.Equal(0))
	// Now the service will not be able to trace to the gateway
	gateways = SvcToGateway("reviews", "default")
	g.Expect(len(gateways)).To(gomega.Equal(0))
}
