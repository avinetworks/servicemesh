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
	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

func VSToGateway(vsName string, namespace string) []string {
	// Given a VS Key - trace to the gateways that are associated with it.
	found, istioObj := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).Get(vsName)
	if !found {
		//This is a DELETE event. First let's find the impacted Gateways.
		_, gateways := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		// For each gateway, delete the gateway to VS mappings.
		for _, gateway := range gateways {
			istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteGwToVsRefs(gateway, vsName)
		}
		// DELETE the vs to gateway relationship for this VS
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteVSToGw(vsName)
		// Update the SVC relationships
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteSvcToVs(vsName)
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).DeleteVSToSVC(vsName)
		utils.AviLog.Info.Printf("Obtained the Gateways to process the VS DELETE %s", gateways)
		return gateways
	} else {
		// It's an ADD or UPDATE event. Update the relationships GW Rel first
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).UpdateGatewayVsRefs(istioObj)
		// Update the SVC relationships
		istio_objs.SharedVirtualServiceLister().VirtualService(namespace).UpdateSvcVSRefs(istioObj)
		// Find the gateways.
		_, gateways := istio_objs.SharedVirtualServiceLister().VirtualService(namespace).GetGatewaysForVS(vsName)
		utils.AviLog.Info.Printf("Obtained the Gateways to process the VS ADD/UPDATE %s", gateways)
		return gateways
	}
}

func GwToGateway(key string, namespace string) []string {
	// Gateway is the root of the graph, hence it will no-op.
	return nil
}

func ServiceToGateway(svcName string, namespace string) []string {
	// Given a Service Key - trace to the gateways that are associated with it.
	// first figure out, what are the VSes, associated with this service. Then, for each VS, find out the Gateways
	// Collate the gateways and send it back.
	var gateways []string
	_, vsNames := istio_objs.SharedSvcLister().Service(namespace).GetSvcToVS(svcName)
	for _, vsName := range vsNames {
		// For each VS find out the associated Gateways
		gateways = VSToGateway(vsName, namespace)
	}
	utils.AviLog.Info.Printf("Obtained the Gateways to process the SVC CUD event %s", gateways)
	return gateways
}

func EndpointToGateway(key string, namespace string) []string {
	// Given a VS Endpoint - trace to the gateways that are associated with it.
	return nil
}
