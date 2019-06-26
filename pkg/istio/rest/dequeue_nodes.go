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

package rest

import (
	"strings"

	"github.com/avinetworks/servicemesh/pkg/istio/nodes"
	"github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

func DeQueueNodes(key string) {
	// Got the key from the Graph Layer - let's fetch the model
	ok, avimodelIntf := objects.SharedAviGraphLister().Get(key)
	//namespace =
	var rest_ops []*utils.RestOp
	avimodel := avimodelIntf.(*nodes.AviObjectGraph)
	if !ok {
		utils.AviLog.Info.Printf("No model found for the key %s", key)
	}
	// Order would be this: 1. Pools 2. PGs  3. HTTPPolicies. 4. VS
	// Get the pools
	//gatewayNs := extractGatewayNamespace(key)
	pools := avimodel.GetAviPools()
	for _, pool := range pools {
		// check in the pool cache to see if this pool exists in AVI

		restOp := AviPoolBuild(pool)
		rest_ops = append(rest_ops, restOp)
		utils.AviLog.Info.Printf("Pool Rest Ops %s", utils.Stringify(restOp))
	}
	poolGroups := avimodel.GetAviPoolGroups()
	for _, pg := range poolGroups {
		restOp := AviPoolGroupBuild(pg)
		rest_ops = append(rest_ops, restOp)
		utils.AviLog.Info.Printf("PoolGroup Rest Ops %s", utils.Stringify(restOp))
	}
	HTTPPolicies := avimodel.GetAviHttpPolicies()
	for _, policy := range HTTPPolicies {
		restOp := AviHttpPSBuild(policy)
		rest_ops = append(rest_ops, restOp)
		utils.AviLog.Info.Printf("HTTP Policy set %s", utils.Stringify(restOp))
	}
	aviVSes := avimodel.GetAviVS()
	for _, aviVs := range aviVSes {
		restOp := AviVsBuild(aviVs, HTTPPolicies)
		rest_ops = append(rest_ops, restOp...)
		utils.AviLog.Info.Printf("VS Rest Ops %s", utils.Stringify(restOp))
	}
	avi_rest_client_pool := utils.SharedAVIClients()
	aviclient := avi_rest_client_pool.AviClient[0]
	err := avi_rest_client_pool.AviRestOperate(aviclient, rest_ops)
	if err != nil {
		utils.AviLog.Info.Fatalf("There was an error sending the macro %s", err)
	}
}

func extractGatewayNamespace(key string) string {
	segments := strings.Split(key, "/")
	if len(segments) == 2 {
		return segments[0]
	}
	return ""
}
