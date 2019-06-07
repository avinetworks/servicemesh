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
package mcptests

import (
	"fmt"
	"testing"

	"github.com/avinetworks/servicemesh/pkg/istio/mcp"
	queue "github.com/avinetworks/servicemesh/pkg/k8s"
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
	controller := mcp.NewController()

	messages := convertToResource(g, mcp.Gateway.MessageName, []proto.Message{gateway1, gateway2})
	message, message2 := messages[0], messages[1]

	change := convertToMcpAPIEvent(
		[]proto.Message{message, message2},
		[]string{"default/gateway1", "red/gateway2"},
		mcp.Gateway.Collection, mcp.Gateway.MessageName)

	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	sharedQueue := queue.SharedWorkQueue().GetQueueByName(utils.ObjectIngestionLayer)
	// Run the hashing algorithm to figure out which bucket to expect the item
	redbkt := utils.Bkt("red", sharedQueue.NumWorkers)
	defaultbkt := utils.Bkt("default", sharedQueue.NumWorkers)
	keys := GetKeysFromQueue(sharedQueue.Workqueue[redbkt], 2)
	g.Expect(keys).To(gomega.ContainElement("gateway/red/gateway2"))

	if redbkt == defaultbkt {
		// If they are both hashed to the same bucket then we will find both the keys inside this.
		g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway1"))
	} else {
		keys = GetKeysFromQueue(sharedQueue.Workqueue[defaultbkt], 2)
		g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway1"))
	}

}

func TestControllerRepeatResourceShards(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := mcp.NewController()

	messages := convertToResource(g, mcp.Gateway.MessageName, []proto.Message{gateway1, gateway2})
	messagesVS := convertToResource(g, mcp.VirtualService.MessageName, []proto.Message{virtualservice1, virtualservice2})
	message, message2 := messages[0], messages[1]
	message3, message4 := messagesVS[0], messagesVS[1]

	changeGw := convertToMcpAPIEvent(
		[]proto.Message{message, message2},
		[]string{"default/gateway1", "red/gateway3"},
		mcp.Gateway.Collection, mcp.Gateway.MessageName)
	changeVs := convertToMcpAPIEvent(
		[]proto.Message{message3, message4},
		[]string{"red/vs1", "red/vs2"},
		mcp.VirtualService.Collection, mcp.VirtualService.MessageName)
	err := controller.Apply(changeGw)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	err = controller.Apply(changeVs)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	sharedQueue := queue.SharedWorkQueue().GetQueueByName(utils.ObjectIngestionLayer)
	// Run the hashing algorithm to figure out which bucket to expect the item
	redbkt := utils.Bkt("red", sharedQueue.NumWorkers)
	// Here we shouldn't find the Gateway object gateway1 because they are already present
	// with the same resource versions in the store due to the previous test case
	keys := GetKeysFromQueue(sharedQueue.Workqueue[redbkt], 3)
	g.Expect(keys).To(gomega.ContainElement("virtual-service/red/vs1"))
	g.Expect(keys).To(gomega.ContainElement("virtual-service/red/vs2"))
	g.Expect(keys).To(gomega.ContainElement("gateway/red/gateway3"))

}

func TestControllerDeleteResourceShards(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := mcp.NewController()

	messages := convertToResource(g, mcp.Gateway.MessageName, []proto.Message{gateway1, gateway2})
	message, message1 := messages[0], messages[1]

	changeGw := convertToMcpAPIEvent(
		[]proto.Message{message1},
		[]string{"default/gateway3"},
		mcp.Gateway.Collection, mcp.Gateway.MessageName)
	err := controller.Apply(changeGw)

	g.Expect(err).ToNot(gomega.HaveOccurred())
	sharedQueue := queue.SharedWorkQueue().GetQueueByName(utils.ObjectIngestionLayer)
	defaultbkt := utils.Bkt("default", sharedQueue.NumWorkers)
	keys := GetKeysFromQueue(sharedQueue.Workqueue[defaultbkt], 1)
	g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway3"))
	// Let's remove it from the queue.
	sharedQueue.Workqueue[defaultbkt].Done("gateway/default/gateway3")
	// Deletes gateway3
	changeGw = convertToMcpAPIEvent(
		[]proto.Message{message, message1},
		[]string{"default/gateway2", "default/gateway4"},
		mcp.Gateway.Collection, mcp.Gateway.MessageName)
	err = controller.Apply(changeGw)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	// Run the hashing algorithm to figure out which bucket to expect the item
	// Here we should find gateway3 as a key since it was DELETED
	keys = GetKeysFromQueue(sharedQueue.Workqueue[0], 3)
	g.Expect(keys).To(gomega.ContainElement("gateway/default/gateway3"))

}

func GetKeysFromQueue(workqueue workqueue.RateLimitingInterface, length int) []string {
	var keys []string
	var obj interface{}
	for i := 0; i < length; i++ {
		obj, _ = workqueue.Get()
		keys = append(keys, obj.(string))
	}
	return keys
}

func convertToResource(g *gomega.GomegaWithT, messageName string, resources []proto.Message) (messages []proto.Message) {
	// Generate protobuf messages for resources.
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

func convertToMcpAPIEvent(resources []proto.Message, names []string, collection, responseMessageName string) *sink.Change {
	// Generate a sync change data type using protofbufs for various resources.
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
