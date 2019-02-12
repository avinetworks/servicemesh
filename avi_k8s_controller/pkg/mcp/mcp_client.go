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
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	mcpapi "istio.io/api/mcp/v1alpha1"
	istio_networking_v1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/mcp/client"
	"istio.io/istio/pkg/mcp/configz"
	"istio.io/istio/pkg/mcp/monitoring"
	"istio.io/istio/pkg/mcp/sink"
)

const (

	// DefaultMCPMaxMsgSize is the default maximum message size
	DefaultMCPMaxMsgSize = 1024 * 1024 * 4
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
	fmt.Println("We will no-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

func ValidateGateway(name, namespace string, msg proto.Message) (errs error) {
	fmt.Println("We will no-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

func ValidateServiceEntry(name, namespace string, msg proto.Message) (errs error) {
	fmt.Println("We will no-op for now")
	// This is a bogus print log here to initialize the "istio.io/api/networking/v1alpha3"
	// Can be removed when we use this package for more work.
	fmt.Println(istio_networking_v1alpha3.TLSSettings_ISTIO_MUTUAL)
	return
}

type ConfigDescriptor []ProtoSchema

type ProtoSchema struct {
	ClusterScoped bool

	Type string

	Plural string

	Group string

	Version string

	MessageName string

	Validate func(name, namespace string, config proto.Message) error

	Collection string
}

type MCPClient struct {
	MCPServerAddrs []string
	startFuncs     []startFunc
}

func (c *MCPClient) Start(stop <-chan struct{}) error {
	// Now start all of the components.
	for _, fn := range c.startFuncs {
		if err := fn(stop); err != nil {
			return err
		}
	}

	return nil
}

func (c *MCPClient) addStartFunc(fn startFunc) {
	c.startFuncs = append(c.startFuncs, fn)
}

type startFunc func(stop <-chan struct{}) error

func (c *MCPClient) InitMCPClient() error {
	clientNodeID := ""
	collections := make([]sink.CollectionOptions, len(IstioConfigTypes))
	for i, model := range IstioConfigTypes {
		collections[i] = sink.CollectionOptions{
			Name: model.Collection,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var clients []*client.Client
	var conns []*grpc.ClientConn

	reporter := monitoring.NewStatsContext("gocontroller/mcp/sink")

	for _, addr := range c.MCPServerAddrs {
		u, err := url.Parse(addr)
		if err != nil {
			cancel()
			return err
		}
		fmt.Println("The MCP server address", u.Host)
		securityOption := grpc.WithInsecure()
		msgSizeOption := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(DefaultMCPMaxMsgSize))
		conn, err := grpc.DialContext(ctx, u.Host, securityOption, msgSizeOption)
		if err != nil {
			fmt.Errorf("Unable to dial MCP Server %q: %v", u.Host, err)
			cancel()
			return err
		}
		cl := mcpapi.NewAggregatedMeshConfigServiceClient(conn)
		mcpController := NewController()
		sinkOptions := &sink.Options{
			CollectionOptions: collections,
			Updater:           mcpController,
			ID:                clientNodeID,
			Reporter:          reporter,
		}
		mcpClient := client.New(cl, sinkOptions)
		configz.Register(mcpClient)
		fmt.Println("Successfully registered the client")
		clients = append(clients, mcpClient)
		conns = append(conns, conn)
	}

	c.addStartFunc(func(stop <-chan struct{}) error {
		var wg sync.WaitGroup

		for i := range clients {
			client := clients[i]
			wg.Add(1)
			go func() {
				client.Run(ctx)
				wg.Done()
			}()
		}

		go func() {
			<-stop

			// Stop the MCP clients and any pending connection.
			cancel()

			// Close all of the open grpc connections once the mcp
			// client(s) have fully stopped.
			wg.Wait()
			for _, conn := range conns {
				_ = conn.Close() // nolint: errcheck
			}

			reporter.Close()
		}()

		return nil
	})
	return nil
}
