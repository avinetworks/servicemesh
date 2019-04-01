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
	"fmt"

	"github.com/golang/protobuf/proto"
	istio_networking_v1alpha3 "istio.io/api/networking/v1alpha3"
)

var (
	// VirtualService describes v1alpha3 route rules
	VirtualService = ProtoSchema{
		Type:        "virtual-service",
		Plural:      "virtual-services",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.VirtualService",
		Validate:    ValidateVirtualService,
		Collection:  "istio/networking/v1alpha3/virtualservices",
	}
	Gateway = ProtoSchema{
		Type:        "gateway",
		Plural:      "gateways",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.Gateway",
		Validate:    ValidateGateway,
		Collection:  "istio/networking/v1alpha3/gateways",
	}
	// ServiceEntry describes service entries
	ServiceEntry = ProtoSchema{
		Type:        "service-entry",
		Plural:      "service-entries",
		Group:       "networking",
		Version:     "v1alpha3",
		MessageName: "istio.networking.v1alpha3.ServiceEntry",
		Validate:    ValidateServiceEntry,
		Collection:  "istio/networking/v1alpha3/serviceentries",
	}

	// IstioConfigTypes lists all Istio config types with schemas and validation
	IstioConfigTypes = ConfigDescriptor{
		VirtualService,
		Gateway,
		ServiceEntry,
	}
)

func ValidateVirtualService(name, namespace string, msg proto.Message) (errs error) {
	fmt.Println("No-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

func ValidateGateway(name, namespace string, msg proto.Message) (errs error) {
	fmt.Println("No-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

func ValidateServiceEntry(name, namespace string, msg proto.Message) (errs error) {
	fmt.Println("No-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

// GetByType finds a schema by type if it is available
func (descriptor ConfigDescriptor) GetByType(name string) (ProtoSchema, bool) {
	for _, schema := range descriptor {
		if schema.Type == name {
			return schema, true
		}
	}
	return ProtoSchema{}, false
}

type ConfigDescriptor []ProtoSchema

type ProtoSchema struct {
	ClusterScoped bool
	Type          string
	Plural        string
	Group         string
	Version       string
	MessageName   string
	Validate      func(name, namespace string, config proto.Message) error
	Collection    string
}
