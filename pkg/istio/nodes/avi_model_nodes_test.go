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
	"testing"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/onsi/gomega"
)

var aviVSNode *AviVsNode
var aviPGNode *AviPoolGroupNode
var aviPN *AviPoolNode
var aviObjGraph *AviObjectGraph

// set up an AVI VS node struct to test methods from the avi_model_nodes.go file
func checksumSetup() {
	// setting up the AviVsNode the AviPoolGroupNode and the AviPoolNode variables
	svcMdObj := ServiceMetadataObj{`json:"crud_hash_key"`}
	aviPortHP := AviPortHostProtocol{Port: 80, Protocol: "TCP", Hosts: []string{"10.52.58.82"}}
	vsNode := AviVsNode{"fanout-ingress.default.avi.internal", "default", svcMdObj, "System-TCP-Proxy", "System-HTTP", []AviPortHostProtocol{aviPortHP}, "", true, 0, "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", 0}
	var irms isRouteMatch_PathSpecifier
	matchList := MatchCriteria{"", "", irms}
	var model *avimodels.PoolGroupMember
	pg := AviPoolGroupNode{Name: "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", Tenant: "default", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, RuleChecksum: 0, Members: []*avimodels.PoolGroupMember{model}, MatchList: []MatchCriteria{matchList}}
	var ip avimodels.IPAddr
	server := AviPoolMetaServer{Ip: ip, ServerNode: "fanout-ingress.default.avi.internal"}
	poolNode := AviPoolNode{"fanout-ingress.default.avi.internal--*-aviroute-pool-8080-tcp", "default", svcMdObj, 0, 80, "HTTP", []AviPoolMetaServer{server}, "HTTP"}

	aviVSNode = &vsNode
	aviPGNode = &pg
	aviPN = &poolNode
	// Setting up a new Avi Object Graph
	aviObjGraph = NewAviObjectGraph()

}

func TestGetCheckSum(t *testing.T) {
	checksumSetup()
	g := gomega.NewGomegaWithT(t)

	aviObjGraph.AddModelNode(aviVSNode)
	aviObjGraph.AddModelNode(aviPGNode)
	aviObjGraph.AddModelNode(aviPN)

	// Setting the checksum value for the AviVsNode, the AviPoolGroupNode and the AviPoolNode
	aviVSNode.CalculateCheckSum()
	aviPGNode.CalculateCheckSum()
	aviPN.CalculateCheckSum()

	avipgnodes := aviObjGraph.GetAviPoolGroups()
	avivsnodes := aviObjGraph.GetAviVS()
	avipoolnodes := aviObjGraph.GetAviPools()

	// Testing the GetCheckSum() method for the AviVsNode, AviPoolGroupNode and for the AviPoolNode
	g.Expect(avipgnodes[0].GetCheckSum()).To(gomega.Equal(aviPGNode.GetCheckSum()))
	g.Expect(avivsnodes[0].GetCheckSum()).To(gomega.Equal(aviVSNode.GetCheckSum()))
	g.Expect(avipoolnodes[0].GetCheckSum()).To(gomega.Equal(aviPN.GetCheckSum()))

}

func TestMatchCriteriaMethods(t *testing.T) {
	var irmps isRouteMatch_PathSpecifier
	matchList := MatchCriteria{"", "", irmps}
	m := &matchList
	g := gomega.NewGomegaWithT(t)
	g.Expect(m.GetPathSpecifier()).To(gomega.BeNil())
	g.Expect(matchList.PathSpecifier).To(gomega.BeNil())
	// Testing if the GetPath() method returns an empty string since the path variable has not been set
	g.Expect(m.GetPath()).To(gomega.Equal(""))
	// Testing if the GetPrefix() method retiurns an empty string since the Prefix variable has not been set
	g.Expect(m.GetPrefix()).To(gomega.Equal(""))
	// Testing if the GetRegex() method returns an empty string since the Regex variable has not been set
	g.Expect(m.GetRegex()).To(gomega.Equal(""))

}
