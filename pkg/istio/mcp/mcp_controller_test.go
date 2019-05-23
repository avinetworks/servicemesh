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

// Some of the test cases are taken and modified from the Istio Project.
package mcp

import (
	"fmt"
	"testing"

	"github.com/avinetworks/servicemesh/pkg/utils"
	"github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/proto"
	"github.com/onsi/gomega"
	mcpapi "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/mcp/sink"
	"k8s.io/client-go/util/workqueue"
)

var (
	gateway1 = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   443,
					Name:     "https",
					Protocol: "HTTP",
				},
				Hosts: []string{"*.secure.example.com"},
			},
		},
	}

	gateway2 = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   80,
					Name:     "http",
					Protocol: "HTTP",
				},
				Hosts: []string{"*.example.com"},
			},
		},
	}

	gateway3 = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   8080,
					Name:     "http",
					Protocol: "HTTP",
				},
				Hosts: []string{"foo.example.com"},
			},
		},
	}
	virtualservice1 = &networking.VirtualService{
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
	virtualservice2 = &networking.VirtualService{
		Hosts:    []string{"prod1", "test1"},
		Gateways: []string{"gw2"},
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
)

func TestControllerQueueShardingCheck(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := NewController()

	messages := convertToResource(g, Gateway.MessageName, []proto.Message{gateway1, gateway2})
	message, message2 := messages[0], messages[1]

	change := convert(
		[]proto.Message{message, message2},
		[]string{"default/gateway1", "red/gateway2"},
		Gateway.Collection, Gateway.MessageName)

	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	sharedQueue := utils.SharedWorkQueueWrappers().GetQueueByName("MCPLayer")
	// Run the hashing algorithm to figure out which bucket to expect the item
	redbkt := utils.Bkt("red", sharedQueue.NumWorkers)
	defaultbkt := utils.Bkt("default", sharedQueue.NumWorkers)
	keys := FindKeyInQueue(sharedQueue.Workqueue[redbkt], 2)
	g.Expect(keys).To(gomega.ContainElement("gateway/red/gateway2"))

	if redbkt == defaultbkt {
		// If they are both hashed to the same bucket then we will find both the keys inside this.
		g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway1"))
	} else {
		keys = FindKeyInQueue(sharedQueue.Workqueue[defaultbkt], 2)
		g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway1"))
	}

}

func TestControllerRepeatResourceShards(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := NewController()

	messages := convertToResource(g, Gateway.MessageName, []proto.Message{gateway1, gateway2})
	messagesVS := convertToResource(g, VirtualService.MessageName, []proto.Message{virtualservice1, virtualservice2})
	message, message2 := messages[0], messages[1]
	message3, message4 := messagesVS[0], messagesVS[1]

	changeGw := convert(
		[]proto.Message{message, message2},
		[]string{"default/gateway1", "red/gateway3"},
		Gateway.Collection, Gateway.MessageName)
	changeVs := convert(
		[]proto.Message{message3, message4},
		[]string{"red/vs1", "red/vs2"},
		VirtualService.Collection, VirtualService.MessageName)
	err := controller.Apply(changeGw)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	err = controller.Apply(changeVs)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	sharedQueue := utils.SharedWorkQueueWrappers().GetQueueByName("MCPLayer")
	// Run the hashing algorithm to figure out which bucket to expect the item
	redbkt := utils.Bkt("red", sharedQueue.NumWorkers)
	// Here we shouldn't find the Gateway object gateway1 because they are already present
	// with the same resource versions in the store due to the previous test case
	keys := FindKeyInQueue(sharedQueue.Workqueue[redbkt], 3)
	g.Expect(keys).To(gomega.ContainElement("virtual-service/red/vs1"))
	g.Expect(keys).To(gomega.ContainElement("virtual-service/red/vs2"))
	g.Expect(keys).To(gomega.ContainElement("gateway/red/gateway3"))

}

// func TestControllerDeleteResourceShards(t *testing.T) {
// 	g := gomega.NewGomegaWithT(t)
// 	controller := NewController()

// 	messages := convertToResource(g, Gateway.MessageName, []proto.Message{gateway1, gateway2})
// 	messagesVS := convertToResource(g, VirtualService.MessageName, []proto.Message{virtualservice1, virtualservice2})
// 	message2 := messages[1]
// 	message3, message4 := messagesVS[0], messagesVS[1]

// 	changeGw := convert(
// 		[]proto.Message{message2},
// 		[]string{"red/gateway3"},
// 		Gateway.Collection, Gateway.MessageName)
// 	changeVs := convert(
// 		[]proto.Message{message3, message4},
// 		[]string{"red/vs1", "red/vs2"},
// 		VirtualService.Collection, VirtualService.MessageName)
// 	err := controller.Apply(changeGw)
// 	g.Expect(err).ToNot(gomega.HaveOccurred())
// 	err = controller.Apply(changeVs)
// 	g.Expect(err).ToNot(gomega.HaveOccurred())
// 	sharedQueue := utils.SharedWorkQueueWrappers().GetQueueByName("MCPLayer")
// 	// Run the hashing algorithm to figure out which bucket to expect the item
// 	redbkt := utils.Bkt("red", sharedQueue.NumWorkers)
// 	// Here we should find gateway1 as a key since it was DELETED
// 	keys := FindKeyInQueue(sharedQueue.Workqueue[redbkt], 1)
// 	g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway1"))

// }

func FindKeyInQueue(workqueue workqueue.RateLimitingInterface, length int) []string {
	var keys []string
	var obj interface{}
	for i := 0; i < length; i++ {
		obj, _ = workqueue.Get()
		keys = append(keys, obj.(string))
	}
	return keys
}

func convertToResource(g *gomega.GomegaWithT, messageName string, resources []proto.Message) (messages []proto.Message) {
	for _, resource := range resources {
		marshaled, err := proto.Marshal(resource)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		message, err := makeMessage(marshaled, messageName)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		messages = append(messages, message)
	}
	return messages
}

func makeMessage(value []byte, responseMessageName string) (proto.Message, error) {
	resource := &types.Any{
		TypeUrl: fmt.Sprintf("type.googleapis.com/%s", responseMessageName),
		Value:   value,
	}

	var dynamicAny types.DynamicAny
	err := types.UnmarshalAny(resource, &dynamicAny)
	if err == nil {
		return dynamicAny.Message, nil
	}

	return nil, err
}

func convert(resources []proto.Message, names []string, collection, responseMessageName string) *sink.Change {
	out := new(sink.Change)
	out.Collection = collection
	for i, res := range resources {
		out.Objects = append(out.Objects,
			&sink.Object{
				TypeURL: responseMessageName,
				Metadata: &mcpapi.Metadata{
					Name: names[i],
				},
				Body: res,
			},
		)
	}
	return out
}
