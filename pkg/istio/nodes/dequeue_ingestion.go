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
	"strings"

	"github.com/avinetworks/servicemesh/pkg/istio/objects"
	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

func DequeueIngestion(key string) {
	// The key format expected here is: objectType/Namespace/ObjKey
	utils.AviLog.Info.Printf("%s: Starting graph Sync", key)
	objType, namespace, name := extractTypeNameNamespace(key)
	schema, valid := ConfigDescriptor().GetByType(objType)
	if !valid {
		// Invalid objectType obtained
		utils.AviLog.Warning.Printf("%s: Invalid Graph Schema type obtained.", key)
		return
	}
	sharedQueue := utils.SharedWorkQueue().GetQueueByName(utils.GraphLayer)
	gatewayNames, gateway_found := schema.GetParentGateways(name, namespace)
	// Update the relationships associated with this object
	if !gateway_found && objType == "gateway" {
		for _, gwName := range gatewayNames {
			model_name := namespace + "/" + gwName
			// This is a special case, Gateway delete event. We need to delete the entire VS.
			// Short circuit and publish the VS key for deletion to Layer 3.
			istio_objs.SharedAviGraphLister().Save(model_name, nil)
			bkt := utils.Bkt(model_name, sharedQueue.NumWorkers)
			sharedQueue.Workqueue[bkt].AddRateLimited(model_name)
		}
		return
	}
	if len(gatewayNames) == 0 {
		utils.AviLog.Info.Printf("%s: Couldn't trace to the gateway for key.", key)
		// No gateways associated with this update. No-op
		return
	}
	for _, gateway := range gatewayNames {
		gatewayNs := namespace
		namespacedGw := strings.Contains(gateway, "/")
		if namespacedGw {
			nsGw := strings.Split(gateway, "/")
			gatewayNs = nsGw[0]
			gateway = nsGw[1]
		}
		// Gateways provide us data for AVI Virtual Machine. First check if it exists?
		found, gwObj := istio_objs.SharedGatewayLister().Gateway(gatewayNs).Get(gateway)
		if !found {
			// The Gateway object is not found, we don't have to care about it. Let's pass
			utils.AviLog.Info.Printf("%s: Gateway object for gateway name: gw-%s-%s does not exist", key, gatewayNs, gateway)
			continue
		} else {
			utils.AviLog.Info.Printf("%s: Obtained Gateway: gw-%s-%s to sync to graph", key, gatewayNs, gateway)
			aviModelGraph := NewAviObjectGraph()
			aviModelGraph.BuildAviObjectGraph(namespace, gatewayNs, gateway, gwObj)
			if len(aviModelGraph.GetOrderedNodes()) != 0 {
				publishKeyToRestLayer(aviModelGraph, gatewayNs, gateway, sharedQueue)
				utils.AviLog.Info.Printf("%s: The list of ordered nodes :%s", key, utils.Stringify(aviModelGraph.GetOrderedNodes()))

			}
		}

	}
}

func publishKeyToRestLayer(aviGraph *AviObjectGraph, gatewayNs string, gatewayName string, sharedQueue *utils.WorkerQueue) {
	model_name := gatewayNs + "/" + gatewayName
	// First see if there's another instance of the same model in the store
	found, aviModel := objects.SharedAviGraphLister().Get(model_name)
	if found {
		prevChecksum := aviModel.(*AviObjectGraph).GetCheckSum()
		utils.AviLog.Info.Printf("The model: %s has a previous checksum: %v", model_name, prevChecksum)
		presentChecksum := aviGraph.GetCheckSum()
		utils.AviLog.Info.Printf("The model: %s has a present checksum: %v", model_name, presentChecksum)
		if prevChecksum == presentChecksum {
			utils.AviLog.Info.Printf("The model: %s has identical checksums, hence not processing. Checksum value: %v", model_name, presentChecksum)
			return
		}
	}
	// TODO (sudswas): Lots of checksum optimization goes here
	istio_objs.SharedAviGraphLister().Save(model_name, aviGraph)
	bkt := utils.Bkt(model_name, sharedQueue.NumWorkers)
	sharedQueue.Workqueue[bkt].AddRateLimited(model_name)
}

func BuildAviGraph(gws []string) {
	/* We should be picking up each gateway and then traverse the gateway with a pre-known relationship.
	 * as we visit each node while walking from the gateway, we would call a AVI Translate function, that would
	 * translate each node into a pre-defined set of AVI objects */

	return
}

func ConfigDescriptor() GraphDescriptor {
	return SupportedGraphTypes
}

func (descriptor GraphDescriptor) GetByType(name string) (GraphSchema, bool) {
	for _, schema := range descriptor {
		if schema.Type == name {
			return schema, true
		}
	}
	return GraphSchema{}, false
}

func extractTypeNameNamespace(key string) (string, string, string) {
	segments := strings.Split(key, "/")
	if len(segments) == 3 {
		return segments[0], segments[1], segments[2]
	}
	return "", "", segments[0]
}
