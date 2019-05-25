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
	"strconv"

	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model/test"
)

func Make(namespace string, name string, i int) *IstioObject {
	return &IstioObject{
		ConfigMeta: ConfigMeta{
			Type:            "mocked-type",
			Group:           "test.istio.io",
			Version:         "v1",
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: strconv.Itoa(i),
			Labels: map[string]string{
				"key": name,
			},
			Annotations: map[string]string{
				"annotationkey": name,
			},
		},
		Spec: &test.MockConfig{
			Key: name,
			Pairs: []*test.ConfigPair{
				{Key: "key", Value: strconv.Itoa(i)},
			},
		},
	}
}

func MakeVirtualService(namespace string, name string, i int) *IstioObject {
	ExampleVirtualService := &networking.VirtualService{
		Hosts:    []string{"prod", "test"},
		Gateways: []string{"gw1", "mesh"},
		Http: []*networking.HTTPRoute{
			{
				Route: []*networking.HTTPRouteDestination{
					{
						Destination: &networking.Destination{
							Host: "job",
						},
						Weight: 80,
					},
				},
			},
		},
	}
	return &IstioObject{
		ConfigMeta: ConfigMeta{
			Type:            "mocked-type",
			Group:           "test.vs.io",
			Version:         "v1",
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: strconv.Itoa(i),
			Labels: map[string]string{
				"key": name,
			},
			Annotations: map[string]string{
				"annotationkey": name,
			},
		},
		Spec: ExampleVirtualService,
	}
}

func MakeGateway(namespace string, name string, i int) *IstioObject {
	ExampleGateway := &networking.Gateway{
		Servers: []*networking.Server{
			{
				Hosts: []string{"google.com"},
				Port:  &networking.Port{Name: "http", Protocol: "http", Number: 10080},
			},
		},
	}
	return &IstioObject{
		ConfigMeta: ConfigMeta{
			Type:            "mocked-type",
			Group:           "test.gw.io",
			Version:         "v1",
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: strconv.Itoa(i),
			Labels: map[string]string{
				"key": name,
			},
			Annotations: map[string]string{
				"annotationkey": name,
			},
		},
		Spec: ExampleGateway,
	}
}
