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
	"strings"
	"sync"

	"istio.io/istio/pkg/mcp/sink"
)

type Controller struct {
	syncedMu                sync.Mutex
	synced                  map[string]bool
	descriptorsByCollection map[string]ProtoSchema
}

func NewController() *Controller {
	synced := make(map[string]bool)
	descriptorsByMessageName := make(map[string]ProtoSchema, len(IstioConfigTypes))
	for _, descriptor := range IstioConfigTypes {
		// don't register duplicate descriptors for the same collection
		if _, ok := descriptorsByMessageName[descriptor.Collection]; !ok {
			descriptorsByMessageName[descriptor.Collection] = descriptor
			synced[descriptor.Collection] = false
		}
	}

	return &Controller{
		synced:                  synced,
		descriptorsByCollection: descriptorsByMessageName,
	}
}

// Apply receives changes from MCP server and creates the
// corresponding config
func (c *Controller) Apply(change *sink.Change) error {
	c.syncedMu.Lock()
	c.synced[change.Collection] = true
	c.syncedMu.Unlock()
	for _, obj := range change.Objects {
		namespace, name := extractNameNamespace(obj.Metadata.Name)
		fmt.Println("Got an update for name:  ", name)
		fmt.Println("The namespace updated is: ", namespace)
	}
	return nil
}

func extractNameNamespace(metadataName string) (string, string) {
	segments := strings.Split(metadataName, "/")
	if len(segments) == 2 {
		return segments[0], segments[1]
	}
	return "", segments[0]
}
