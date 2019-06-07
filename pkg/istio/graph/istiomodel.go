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

package graph

import (
	"strings"

	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

var (
	VirtualService = GraphSchema{
		Type:              "virtual-service",
		GetParentGateways: VSToGateway,
	}
	Gateway = GraphSchema{
		Type:              "gateway",
		GetParentGateways: DetectGatewayChanges,
	}
	Service = GraphSchema{
		Type:              "Service",
		GetParentGateways: SvcToGateway,
	}
	Endpoint = GraphSchema{
		Type:              "Endpoints",
		GetParentGateways: EPToGateway,
	}
	SupportedGraphTypes = GraphDescriptor{
		VirtualService,
		Gateway,
		Service,
		Endpoint,
	}
)

type GraphSchema struct {
	Type              string
	GetParentGateways func(string, string) []string
	UpdateRels        func(string, string) bool
}

type GraphDescriptor []GraphSchema

func VSToGateway(vsName string, namespace string) []string {
	// Given a VS Key - trace to the gateways that are associated with it.
	found, istioObj := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).Get(vsName)
	if !found {
		utils.AviLog.Info.Printf("vs-%s-%s Object not found. It's a DELETED object.", namespace, vsName)
		//This is a DELETE event. First let's find the impacted Gateways.
		_, gateways := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		// For each gateway, delete the gateway to VS mappings.
		for _, gateway := range gateways {
			ns := GetGatewayNamespace(namespace, gateway)
			istio_objs.SharedVirtualServiceLister().VirtualService(ns).DeleteGwToVsRefs(gateway, vsName)
		}
		// DELETE the vs to gateway relationship for this VS
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteVSToGw(vsName)
		// Update the SVC relationships
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteSvcToVs(vsName)
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteVSToSVC(vsName)
		return gateways
	} else {
		utils.AviLog.Info.Printf("vs-%s-%s Object found for VS. It's a ADDED/UPDATED object.", namespace, vsName)
		// Let's first detect the case of a VS disassociation with a Gateway
		_, gateways := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		// Obtain the associted gateways for this VS object to see if they match with what's there in store
		gatewaysFromVSObj := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewayNamesForVS(istioObj)
		var diffGateways []string
		// Diff the two lists to see what are the gateways that could have possibly been removed from the VS.
		for _, gateway := range gateways {
			// Whatever is present in store, and not present in the current VS object, detect them
			gatewayFound := false
			for _, gatewayInStore := range gatewaysFromVSObj {
				if gatewayInStore == gateway {
					gatewayFound = true
				}
			}
			if gatewayFound {
				diffGateways = append(diffGateways, gateway)
				gatewayFound = false
			}
		}
		if len(diffGateways) > 0 {
			// These are deleted gateways for this VS.
			utils.AviLog.Info.Printf("vs-%s-%s, has the following gateways missing %s", namespace, vsName, diffGateways)
			for _, gateway := range diffGateways {
				ns := GetGatewayNamespace(namespace, gateway)
				istio_objs.SharedVirtualServiceLister().VirtualService(ns).DeleteGwToVsRefs(gateway, vsName)
			}
		}
		// It's an ADD or UPDATE event. Update the relationships GW Rel first
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).UpdateGatewayVsRefs(istioObj)
		// Update the SVC relationships
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).UpdateSvcVSRefs(istioObj)
		// Find the gateways.
		_, gateways = istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		// Add the diffGateways to this list. Diff gateways should not have anything overlapping in the gateways.
		gateways = append(gateways, diffGateways...)
		utils.AviLog.Info.Printf("vs-%s-%s, total gateways retrieved %s", namespace, vsName, gateways)
		return gateways
	}
}

func DetectGatewayChanges(gwName string, namespace string) []string {
	var gateways []string
	found, _ := istio_objs.SharedGatewayLister().Gateway(namespace).Get(gwName)
	if !found {
		// It's a gateway delete event. Translates to a AVI VS delete event.
		return nil
	} else {
		// Just add the key
		gateways = append(gateways, gwName)
	}
	return gateways
}

func SvcToGateway(svcName string, namespace string) []string {
	// Given a Service Key - trace to the gateways that are associated with it.
	// first figure out, what are the VSes, associated with this service. Then, for each VS, find out the Gateways
	// Collate the gateways and send it back.
	var gateways []string
	_, vsNames := istio_objs.SharedSvcLister().Service(namespace).GetSvcToVS(svcName)
	utils.AviLog.Info.Printf("svc-%s-%s, The Service has the following associated VSes:  %s", namespace, svcName, vsNames)
	for _, vsName := range vsNames {
		// For each VS find out the associated Gateways
		_, vsGw := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		gateways = append(gateways, vsGw...)
	}
	utils.AviLog.Info.Printf("svc-%s-%s, total gateways retrieved:  %s", namespace, svcName, gateways)
	return gateways
}

func EPToGateway(epName string, namespace string) []string {
	// Given a VS Endpoint - trace to the gateways that are associated with it.
	// The endpoint name is the same as the service name.
	// The below call is safe to make since the ServiceToGateway does not update relationships at the moment.
	gateways := SvcToGateway(epName, namespace)
	utils.AviLog.Info.Printf("ep-%s-%s, total gateways retrieved:  %s", namespace, epName, gateways)
	return gateways
}

func GetGatewayNamespace(namespace string, gateway string) string {
	namespacedGw := strings.Contains(gateway, "/")
	ns := namespace
	if namespacedGw {
		nsGw := strings.Split(gateway, "/")
		ns = nsGw[0]
	}
	return ns
}
