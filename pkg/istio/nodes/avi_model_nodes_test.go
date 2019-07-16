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
var aviVSNode1 *AviVsNode
var aviVSNode2 *AviVsNode
var aviPGNode *AviPoolGroupNode
var aviPGNode1 *AviPoolGroupNode
var aviPGNode2 *AviPoolGroupNode
var aviPN *AviPoolNode
var aviPN1 *AviPoolNode
var aviPN2 *AviPoolNode
var aviObjGraph *AviObjectGraph

func checksumSetup() {
	// setting up the AviVsNode the AviPoolGroupNode and the AviPoolNode variables
	svcMdObj := ServiceMetadataObj{`json:"crud_hash_key"`}
	aviPortHP := AviPortHostProtocol{Port: 80, Protocol: "TCP", Hosts: []string{"10.52.58.82"}}
	vsNode := AviVsNode{Name: "fanout-ingress.default.avi.internal", Tenant: "default", ServiceMetadata: svcMdObj, ApplicationProfile: "System-HTTP", NetworkProfile: "System-TCP-Proxy", PortProto: []AviPortHostProtocol{aviPortHP}, DefaultPool: "", EastWest: true, CloudConfigCksum: 0, DefaultPoolGroup: "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", HTTPChecksum: 0}
	vsNode1 := AviVsNode{Name: "fanout-ingress.default.avi.internal", Tenant: "default", ServiceMetadata: svcMdObj, ApplicationProfile: "System-HTTP", NetworkProfile: "System-TCP-Proxy", PortProto: []AviPortHostProtocol{aviPortHP}, DefaultPool: "", EastWest: true, CloudConfigCksum: 0, DefaultPoolGroup: "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", HTTPChecksum: 0}
	vsNode2 := AviVsNode{Name: "ingress-nginx.ingress-nginx.avi.internal", Tenant: "ingress-nginx", ServiceMetadata: svcMdObj, ApplicationProfile: "System-L4-Application", NetworkProfile: "System-TCP-Proxy", PortProto: []AviPortHostProtocol{aviPortHP}, DefaultPool: "", EastWest: true, CloudConfigCksum: 0, DefaultPoolGroup: "ingress-nginx-poolgroup-https-tcp", HTTPChecksum: 0}
	var irms isRouteMatch_PathSpecifier
	matchList := MatchCriteria{Name: "", Criteria: "", PathSpecifier: irms}
	var model *avimodels.PoolGroupMember
	pg := AviPoolGroupNode{Name: "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", Tenant: "default", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, RuleChecksum: 0, Members: []*avimodels.PoolGroupMember{model}, MatchList: []MatchCriteria{matchList}}
	pg1 := AviPoolGroupNode{Name: "fanout-ingress.default.avi.internal--v2*-aviroute-poolgroup-80-tcp", Tenant: "default", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, RuleChecksum: 0, Members: []*avimodels.PoolGroupMember{model}, MatchList: []MatchCriteria{matchList}}
	pg2 := AviPoolGroupNode{Name: " ingress-nginx-poolgroup-https-tcp", Tenant: "ingress-nginx", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, RuleChecksum: 0, Members: []*avimodels.PoolGroupMember{model}, MatchList: []MatchCriteria{matchList}}
	var ip avimodels.IPAddr
	server := AviPoolMetaServer{Ip: ip, ServerNode: "fanout-ingress.default.avi.internal"}
	poolNode := AviPoolNode{Name: "fanout-ingress.default.avi.internal--*-aviroute-pool-8080-tcp", Tenant: "default", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, Port: 80, PortName: "HTTP", Servers: []AviPoolMetaServer{server}, Protocol: "HTTP", LbAlgorithm: "Least Connections"}
	poolNode1 := AviPoolNode{Name: "fanout-ingress.default.avi.internal--*-aviroute-pool-8080-tcp", Tenant: "default", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, Port: 80, PortName: "HTTP", Servers: []AviPoolMetaServer{server}, Protocol: "HTTP", LbAlgorithm: "Least Connections"}
	poolNode2 := AviPoolNode{Name: "ingress-nginx-pool-https-tcp", Tenant: "ingress-nginx", ServiceMetadata: svcMdObj, CloudConfigCksum: 0, Port: 80, PortName: "HTTP", Servers: []AviPoolMetaServer{server}, Protocol: "HTTP", LbAlgorithm: "Least Connections"}
	aviVSNode = &vsNode
	aviVSNode1 = &vsNode1
	aviVSNode2 = &vsNode2
	aviPGNode = &pg
	aviPGNode1 = &pg1
	aviPGNode2 = &pg2
	aviPN = &poolNode
	aviPN1 = &poolNode1
	aviPN2 = &poolNode2

	// Setting up a new Avi Object Graph
	aviObjGraph = NewAviObjectGraph()
	aviObjGraph.AddModelNode(aviVSNode)
	aviObjGraph.AddModelNode(aviVSNode1)
	aviObjGraph.AddModelNode(aviPGNode)
	aviObjGraph.AddModelNode(aviPGNode1)
	aviObjGraph.AddModelNode(aviPN)
	aviObjGraph.AddModelNode(aviPN1)

}

func TestGetCheckSum(t *testing.T) {
	checksumSetup()
	g := gomega.NewGomegaWithT(t)

	// Setting the checksum value for the AviVsNode, the AviPoolGroupNode and the AviPoolNode
	for _, VSnode := range aviObjGraph.GetAviVS() {
		VSnode.CalculateCheckSum()
	}
	for _, PGNode := range aviObjGraph.GetAviPoolGroups() {
		PGNode.CalculateCheckSum()
	}
	for _, PoolNode := range aviObjGraph.GetAviPools() {
		PoolNode.CalculateCheckSum()
	}

	// Testing the GetCheckSum() method for the AviVsNode, AviPoolGroupNode and for the AviPoolNode

	// Testing the GetChecksum() method for different avi nodes which have the same values
	vsNode := aviObjGraph.GetAviVS()
	g.Expect(vsNode[0].GetCheckSum()).To(gomega.Equal(vsNode[1].GetCheckSum()))
	pgNode := aviObjGraph.GetAviPoolGroups()
	g.Expect(pgNode[0].GetCheckSum()).To(gomega.Equal(pgNode[1].GetCheckSum()))
	poolNode := aviObjGraph.GetAviPools()
	g.Expect(poolNode[0].GetCheckSum()).To(gomega.Equal(poolNode[1].GetCheckSum()))

	// Testing the GetCheckSum() for different avi nodes which have different values
	for _, VSNode := range aviObjGraph.GetAviVS() {
		g.Expect(aviVSNode2.GetCheckSum()).NotTo(gomega.Equal(VSNode.GetCheckSum()))
	}
	for _, PGNode := range aviObjGraph.GetAviPoolGroups() {
		g.Expect(aviPGNode2.GetCheckSum()).NotTo(gomega.Equal(PGNode.GetCheckSum()))
	}
	for _, PoolNode := range aviObjGraph.GetAviPools() {
		g.Expect(aviPN2.GetCheckSum()).NotTo(gomega.Equal(PoolNode.GetCheckSum()))
	}
}

func TestMatchCriteriaMethods(t *testing.T) {
	var irmps isRouteMatch_PathSpecifier
	matchList := MatchCriteria{Name: "", Criteria: "", PathSpecifier: irmps}
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
