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

	"github.com/avinetworks/servicemesh/pkg/utils"
)

var (
	VirtualService = GraphSchema{
		Type:         "virtual-service",
		TraceGateway: VSToGateway,
	}
	Gateway = GraphSchema{
		Type:         "gateway",
		TraceGateway: GwToGateway,
	}
	Service = GraphSchema{
		Type:         "Service",
		TraceGateway: ServiceToGateway,
	}
	Endpoint = GraphSchema{
		Type:         "Endpoints",
		TraceGateway: EndpointToGateway,
	}
	SupportedGraphTypes = GraphDescriptor{
		VirtualService,
		Gateway,
		Service,
		Endpoint,
	}
)

func SyncToGraphLayer(key string) {
	// The key format expected here is: objectType/Namespace/ObjKey
	utils.AviLog.Info.Printf("Obtained Key %s", key)
	objType, namespace, name := extractTypeNameNamespace(key)
	schema, valid := ConfigDescriptor().GetByType(objType)
	if !valid {
		// Invalid objectType obtained
		utils.AviLog.Warning.Printf("Invalid Graph Schema type obtained. %s", objType)
		return
	}
	gateways := schema.TraceGateway(name, namespace)
	// Update the relationships associated with this object
	if len(gateways) == 0 {
		utils.AviLog.Info.Printf("Couldn't trace to the gateway for key %s", key)
		//No gateways associated with this update. No-op
		return
	} else {
		GenerateIstioGraph(gateways)
	}

}

func GenerateIstioGraph(gws []string) {
	return
}

func ConfigDescriptor() GraphDescriptor {
	return SupportedGraphTypes
}

type GraphSchema struct {
	Type         string
	TraceGateway func(string, string) []string
	UpdateRels   func(string, string) bool
}

type GraphDescriptor []GraphSchema

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
