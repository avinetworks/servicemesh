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
	"fmt"
	"strings"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/pkg/istio/objects"
	istio_objs "github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
	networking "istio.io/api/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

const (
	HTTP            = "HTTP"
	HeaderMethod    = ":method"
	HeaderAuthority = ":authority"
	HeaderScheme    = ":scheme"
)

type AviModelNode interface {
	//Each AVIModelNode represents a AVI API object.
	GetCheckSum() uint32
	CalculateCheckSum()
}

type AviObjectGraphIntf interface {
	GetOrderedNodes() []AviModelNode
}

type AviObjectGraph struct {
	modelNodes []AviModelNode
	Name       string
}

func NewAviObjectGraph() *AviObjectGraph {
	return &AviObjectGraph{}
}
func (o *AviObjectGraph) AddModelNode(node AviModelNode) {
	o.modelNodes = append(o.modelNodes, node)
}

func (o *AviObjectGraph) GetOrderedNodes() []AviModelNode {
	return o.modelNodes
}

func (o *AviObjectGraph) GetAviPoolGroups() []*AviPoolGroupNode {
	var poolgroups []*AviPoolGroupNode
	for _, model := range o.modelNodes {
		pg, ok := model.(*AviPoolGroupNode)
		if ok {
			poolgroups = append(poolgroups, pg)
		}
	}
	return poolgroups
}

func (o *AviObjectGraph) GetAviVS() []*AviVsNode {
	var aviVs []*AviVsNode
	for _, model := range o.modelNodes {
		vs, ok := model.(*AviVsNode)
		if ok {
			aviVs = append(aviVs, vs)
		}
	}
	return aviVs
}

func (o *AviObjectGraph) GetAviPools() []*AviPoolNode {
	var aviPools []*AviPoolNode
	for _, model := range o.modelNodes {
		pool, ok := model.(*AviPoolNode)
		if ok {
			aviPools = append(aviPools, pool)
		}
	}
	return aviPools
}

func (o *AviObjectGraph) GetAviHttpPolicies() []*AviHttpPolicySetNode {
	var aviHttpPolicies []*AviHttpPolicySetNode
	for _, model := range o.modelNodes {
		http, ok := model.(*AviHttpPolicySetNode)
		if ok {
			aviHttpPolicies = append(aviHttpPolicies, http)
		}
	}
	return aviHttpPolicies
}

func (o *AviObjectGraph) constructProtocolPortMaps(gwSpec *networking.Gateway) []AviPortHostProtocol {
	var portProtocols []AviPortHostProtocol
	protocolPortMap := make(map[string][]int32)
	for _, server := range gwSpec.Servers {
		// Support HTTP only for now.
		if server.Port.Protocol == HTTP {
			_, ok := protocolPortMap[server.Port.Protocol]
			if ok {
				// Append the port to protocol list
				protocolPortMap[server.Port.Protocol] = append(protocolPortMap[server.Port.Protocol], int32(server.Port.Number))
			} else {
				protocolPortMap[server.Port.Protocol] = []int32{int32(server.Port.Number)}
			}
			pp := AviPortHostProtocol{Port: int32(server.Port.Number), Protocol: HTTP, Hosts: server.Hosts}
			portProtocols = append(portProtocols, pp)
		}
	}
	return portProtocols
}

func (o *AviObjectGraph) generateRandomStringName(vsName string) string {
	// TODO: Watch out for collisions, if need we can increase 10 below.
	random_string := utils.RandomSeq(10)
	utils.AviLog.Info.Printf("Random string generated :%s", random_string)
	pgName := vsName + "-" + random_string
	return pgName
}

func (o *AviObjectGraph) evaluateHTTPMatch(matchrule []*networking.HTTPMatchRequest) uint32 {
	checksum := utils.Hash(utils.Stringify(matchrule))
	utils.AviLog.Info.Printf("Checksum for the HTTP match rules is %d", checksum)
	return checksum
}

func (o *AviObjectGraph) evaluateHTTPPools(ns string, randString string, destinations []*networking.HTTPRouteDestination, gatewayNs string) []*AviPoolNode {
	var poolNodes []*AviPoolNode
	for _, destination := range destinations {
		// For each destination, generate one pool. If weight is not present evalute it to 100.
		serviceName := destination.Destination.Host
		weight := destination.Weight
		if weight == 0 {
			weight = 100
		}
		portName := destination.Destination.Port.GetName()
		portNumber := int32(destination.Destination.Port.GetNumber())
		poolName := serviceName + "-" + randString
		// To be supported: obtain the servers for this service. Naming convention of the service is - svcname.ns.sub-domain
		poolNode := &AviPoolNode{Name: poolName, Tenant: gatewayNs, Port: portNumber, Protocol: HTTP}
		epObj, _ := utils.GetInformers().EpInformer.Lister().Endpoints(ns).Get(serviceName)
		poolNode.Servers = o.extractServers(epObj, portNumber, portName)
		if portName != "" {
			poolNode.PortName = portName
		} else if portNumber != 0 {
			poolNode.Port = portNumber
		}
		poolNode.CalculateCheckSum()
		o.AddModelNode(poolNode)
		poolNodes = append(poolNodes, poolNode)
	}
	utils.AviLog.Info.Printf("Evaluated Pools: %v", utils.Stringify(poolNodes))
	return poolNodes
}

// translateRouteMatch translates match condition
func translateRoutePathMatch(in *networking.HTTPMatchRequest) MatchCriteria {
	out := MatchCriteria{PathSpecifier: &RouteMatch_Prefix{Prefix: "/"}}
	if in == nil {
		return out
	}

	if in.Uri != nil {
		switch m := in.Uri.MatchType.(type) {
		case *networking.StringMatch_Exact:
			out.PathSpecifier = &RouteMatch_Path{Path: m.Exact}
		case *networking.StringMatch_Prefix:
			out.PathSpecifier = &RouteMatch_Prefix{Prefix: m.Prefix}
		case *networking.StringMatch_Regex:
			out.PathSpecifier = &RouteMatch_Regex{Regex: m.Regex}
		}
	}

	//out.CaseSensitive = &types.BoolValue{Value: !in.IgnoreUriCase}

	// if in.Method != nil {
	// 	matcher := translateHeaderMatch(HeaderMethod, in.Method)
	// 	out.Headers = append(out.Headers, &matcher)
	// 	out.Method = in.Method
	// }

	// if in.Authority != nil {
	// 	matcher := translateHeaderMatch(HeaderAuthority, in.Authority)
	// 	out.Headers = append(out.Headers, &matcher)
	// }

	// if in.Scheme != nil {
	// 	matcher := translateHeaderMatch(HeaderScheme, in.Scheme)
	// 	out.Headers = append(out.Headers, &matcher)
	// }
	// for name, stringMatch := range in.QueryParams {
	// 	matcher := translateQueryParamMatch(name, stringMatch)
	// 	out.QueryParameters = append(out.QueryParameters, &matcher)
	// }

	return out
}

func (o *AviObjectGraph) evaluateMatchCriteria(matches []*networking.HTTPMatchRequest) []MatchCriteria {
	var matchCriteria []MatchCriteria
	for _, match := range matches {
		out := translateRoutePathMatch(match)
		matchCriteria = append(matchCriteria, out)
	}
	return matchCriteria
}

func checkPGExists(pgNodes []*AviPoolGroupNode, evalchecksum uint32) (bool, *AviPoolGroupNode) {
	// Iterate through the PG nodes and check if the node exists with the checksum.
	for _, pgNode := range pgNodes {
		if pgNode.RuleChecksum == evalchecksum {
			//Node exists - return true
			return true, pgNode
		}
	}
	return false, nil
}

func (o *AviObjectGraph) ConstructAviPGPoolNodes(vs *istio_objs.IstioObject, model_name string, gatewayNs string) []*AviPoolGroupNode {
	// spec:
	//   hosts:
	//   - reviews.prod.svc.cluster.local
	//   - uk.bookinfo.com
	//   - eu.bookinfo.com
	//   gateways:
	//   - some-config-namespace/my-gateway
	//   - mesh # applies to all the sidecars in the mesh
	//   http:
	//   - match:
	//     - headers:
	//         cookie:
	//           user: dev-123
	//     route:
	//     - destination:
	//         port:
	//           number: 7777
	//         host: reviews.qa.svc.cluster.local
	//   - match:
	//       uri:
	//         prefix: /reviews/
	//     route:
	//     - destination:
	//         port:
	//           number: 9080 # can be omitted if its the only port for reviews
	//         host: reviews.prod.svc.cluster.local
	//       weight: 80
	//     - destination:
	//         host: reviews.qa.svc.cluster.local
	//       weight: 20
	//
	// Derive the pools/poolgroups based on the 'route' information.
	vsObj, _ := vs.Spec.(*networking.VirtualService)
	vsName := vs.ConfigMeta.Name
	var poolGroupNodes []*AviPoolGroupNode
	var prevPoolGroupNodes []*AviPoolGroupNode
	// Fetch the model if it exists for the AVI Vs.
	found, aviModel := objects.SharedAviGraphLister().Get(model_name)
	if found {
		prevPoolGroupNodes = aviModel.(*AviObjectGraph).GetAviPoolGroups()
	}
	// HTTP route handling.

	for _, httpRoute := range vsObj.Http {
		// Generate the PG to Rules map
		rulechecksum := o.evaluateHTTPMatch(httpRoute.Match)
		// Check if the PG already exists or needs to be created
		exists, presentPGNode := checkPGExists(prevPoolGroupNodes, rulechecksum)
		var pgName string
		if !exists {
			pgName = o.generateRandomStringName(vsName)
		} else {
			utils.AviLog.Info.Printf("The PG %s exists in cache with the same checksum", presentPGNode.Name)
			pgName = presentPGNode.Name
		}
		matchList := o.evaluateMatchCriteria(httpRoute.Match)
		pgNode := &AviPoolGroupNode{Name: pgName, Tenant: gatewayNs, RuleChecksum: rulechecksum, MatchList: matchList}
		// Get the pools for the PG
		pools := o.evaluateHTTPPools(vs.ConfigMeta.Namespace, pgName, httpRoute.Route, gatewayNs)
		for _, pool := range pools {
			pool_ref := fmt.Sprintf("/api/pool?name=%s", pool.Name)
			pgNode.Members = append(pgNode.Members, &avimodels.PoolGroupMember{PoolRef: &pool_ref})
		}
		pgNode.CalculateCheckSum()
		o.AddModelNode(pgNode)
		utils.AviLog.Info.Printf("Evaluated the PG :%v", utils.Stringify(pgNode))
		poolGroupNodes = append(poolGroupNodes, pgNode)
	}
	return poolGroupNodes
}

func (o *AviObjectGraph) ConstructAviVsNode(gwObj *istio_objs.IstioObject) *AviVsNode {
	gatewayName := gwObj.ConfigMeta.Name
	namespace := gwObj.ConfigMeta.Namespace
	gwSpec, _ := gwObj.Spec.(*networking.Gateway)
	// FQDN should come from the cloud. Modify
	avi_vs_meta := &AviVsNode{Name: gatewayName, Tenant: namespace,
		EastWest: false}
	avi_vs_meta.PortProto = o.constructProtocolPortMaps(gwSpec)
	// Hard coded right now but will change based as we support more app types.
	avi_vs_meta.ApplicationProfile = "System-HTTP"
	// For HTTP it's always System-TCP-Proxy.
	avi_vs_meta.NetworkProfile = "System-TCP-Proxy"
	avi_vs_meta.CalculateCheckSum()
	utils.AviLog.Info.Printf("Checksum  for AVI VS object %v", avi_vs_meta.GetCheckSum())
	utils.AviLog.Info.Printf("Evaluated PortProto %v", avi_vs_meta.PortProto)
	//o.AddModelNode(avi_vs_meta)
	return avi_vs_meta
}

func matchHosts(vshosts []string, hostPortList []AviPortHostProtocol) []string {
	// Find out the qualifying hosts that should be part of the VS
	qualifiedHostsMap := make(map[string]bool)
	for _, vshost := range vshosts {
		for _, hostprot := range hostPortList {
			for _, host := range hostprot.Hosts {
				if host == "*" || strings.HasSuffix(vshost, strings.Trim(host, "*.")) {
					// Wild card on the gateway port
					if hostprot.Port != 80 {
						qualifiedHostsMap[vshost+":"+fmt.Sprint(hostprot.Port)] = true
					} else {
						qualifiedHostsMap[vshost] = true
					}
				}
			}
		}
	}
	qualifiedHosts := make([]string, 0)
	for key := range qualifiedHostsMap {
		qualifiedHosts = append(qualifiedHosts, key)
	}
	return qualifiedHosts
}

func (o *AviObjectGraph) ConstructAviHttpPolicyNodes(gatewayNs string, vsObj *istio_objs.IstioObject, pgNodes []*AviPoolGroupNode, portHostProto []AviPortHostProtocol) *AviHttpPolicySetNode {
	// Extract the hosts from the vsObj
	vsSpec := vsObj.Spec.(*networking.VirtualService)
	var httpPolicySet []AviHostPathPortPoolPG
	for _, pgNode := range pgNodes {
		// Let's figure out the host headers for each host.
		hosts := matchHosts(vsSpec.Hosts, portHostProto)
		if len(hosts) == 0 {
			// This VS has no eligible hosts. We should return. TODO: Check if we should not even create the PGs in that case.
			return nil
		}
		httpPGPath := AviHostPathPortPoolPG{Host: hosts}
		// Examine the criteria in each pgNode and populate the HTTP rule.
		for _, match := range pgNode.MatchList {
			// Right now the assumption is that each match criteria would be either a exact/contains/regex match.
			// If a mix match inside the same match criteria is supported, we will be able to alter the  AviHostPathPortPoolPG struct and support that.
			if match.GetPath() != "" {
				httpPGPath.Path = append(httpPGPath.Path, match.GetPath())
				httpPGPath.MatchCriteria = "EQUALS"
			} else if match.GetPrefix() != "" {
				httpPGPath.Path = append(httpPGPath.Path, match.GetPrefix())
				httpPGPath.MatchCriteria = "BEGINS_WITH"
			} else if match.GetRegex() != "" {
				httpPGPath.Path = append(httpPGPath.Path, match.GetRegex())
				httpPGPath.MatchCriteria = "REGEX_MATCH"
			}
		}
		httpPGPath.PoolGroup = pgNode.Name
		httpPolicySet = append(httpPolicySet, httpPGPath)
	}
	policyNode := &AviHttpPolicySetNode{Name: vsObj.ConfigMeta.Name, HppMap: httpPolicySet, Tenant: gatewayNs}
	policyNode.CalculateCheckSum()
	utils.AviLog.Info.Printf("The value of HTTP Policy Set is :%s", utils.Stringify(policyNode))
	utils.AviLog.Info.Printf("Computed Checksum for HTTP Policy Set is %v", policyNode.GetCheckSum())
	o.AddModelNode(policyNode)
	return policyNode
}

func (o *AviObjectGraph) extractServers(epObj *corev1.Endpoints, port_num int32, port_name string) []AviPoolMetaServer {
	//TODO: The POD based subsets will be handled subsequently.
	var pool_meta []AviPoolMetaServer
	for _, ss := range epObj.Subsets {
		//var epp_port int32
		port_match := false
		for _, epp := range ss.Ports {
			if (int32(port_num) == epp.Port) || (port_name == epp.Name) {
				port_match = true
				//epp_port = epp.Port
				break
			}
		}
		if port_match {
			//pool_meta.Port = epp_port
			for _, addr := range ss.Addresses {
				var atype string
				ip := addr.IP
				if utils.IsV4(addr.IP) {
					atype = "V4"
				} else {
					atype = "V6"
				}
				a := avimodels.IPAddr{Type: &atype, Addr: &ip}
				server := AviPoolMetaServer{Ip: a}
				if addr.NodeName != nil {
					server.ServerNode = *addr.NodeName
				}
				pool_meta = append(pool_meta, server)
			}
		}
	}
	return pool_meta
}

func (o *AviObjectGraph) BuildAviObjectGraph(namespace string, gatewayNs string, gatewayName string, gwObj *istio_objs.IstioObject) {
	// We use the gateway fields to arrive at various AVI VS Node object.
	var VsNode *AviVsNode

	VsNode = o.ConstructAviVsNode(gwObj)
	// Let's see if the Gateway has associated VSes?
	relExists, vsNames := istio_objs.SharedGatewayLister().Gateway(gatewayNs).GetVSMapping(gatewayName)
	// Does the VS exist?
	if relExists {
		for _, vsName := range vsNames {
			virtualNs := namespace
			namespacedVs := strings.Contains(vsName, "/")
			if namespacedVs {
				nsVs := strings.Split(vsName, "/")
				virtualNs = nsVs[0]
				vsName = nsVs[1]
			}
			vsFound, vsObj := istio_objs.SharedVirtualServiceLister().VirtualService(virtualNs).Get(vsName)
			if vsFound {
				model_name := gatewayNs + "/" + gatewayName
				PGNodes := o.ConstructAviPGPoolNodes(vsObj, model_name, gatewayNs)
				// Now let's Build the HTTP policy set. More checks here for 'type' of route.
				httpPolicyNode := o.ConstructAviHttpPolicyNodes(gatewayNs, vsObj, PGNodes, VsNode.PortProto)
				if httpPolicyNode != nil {
					VsNode.HTTPChecksum = httpPolicyNode.GetCheckSum()
				}
			}
		}
	}

}
