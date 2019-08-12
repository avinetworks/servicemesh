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
	"fmt"
	"strings"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/amc/pkg/istio/objects"
	istio_objs "github.com/avinetworks/servicemesh/amc/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/utils"
	networking "istio.io/api/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
)

const (
	HTTP            = "HTTP"
	HeaderMethod    = ":method"
	HeaderAuthority = ":authority"
	HeaderScheme    = ":scheme"
	TLS             = "TLS"
	HTTPS           = "HTTPS"
	TCP             = "TCP"
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
	modelNodes    []AviModelNode
	Name          string
	GraphChecksum uint32
}

func (v *AviObjectGraph) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.GraphChecksum
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

func (o *AviObjectGraph) GetAviSNIVS() []*AviVsTLSNode {
	var aviSniVs []*AviVsTLSNode
	for _, model := range o.modelNodes {
		vs, ok := model.(*AviVsTLSNode)
		if ok {
			aviSniVs = append(aviSniVs, vs)
		}
	}
	return aviSniVs
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

func (o *AviObjectGraph) GetAviSSLCertKeys() []*AviTLSKeyCertNode {
	var aviSSLCertKeys []*AviTLSKeyCertNode
	for _, model := range o.modelNodes {
		http, ok := model.(*AviTLSKeyCertNode)
		if ok {
			aviSSLCertKeys = append(aviSSLCertKeys, http)
		}
	}
	return aviSSLCertKeys
}

func (o *AviObjectGraph) constructProtocolPortMaps(gwSpec *networking.Gateway) []AviPortHostProtocol {
	var portProtocols []AviPortHostProtocol
	for _, server := range gwSpec.Servers {
		// Support HTTP only for now.
		if server.Port.Protocol == HTTP {
			pp := AviPortHostProtocol{Port: int32(server.Port.Number), Protocol: HTTP, Hosts: server.Hosts}
			if server.Tls != nil && server.Tls.HttpsRedirect {
				// Find out which port it should re-direct to.
				pp.Redirect = true
			}
			portProtocols = append(portProtocols, pp)
		} else if server.Port.Protocol == HTTPS {
			secretName := ""
			passthrough := false
			if server.Tls != nil && server.Tls.Mode == networking.Server_TLSOptions_SIMPLE {
				secretName = server.Tls.CredentialName
			} else if server.Tls != nil && server.Tls.Mode == networking.Server_TLSOptions_PASSTHROUGH {
				passthrough = true
			}
			pp := AviPortHostProtocol{Port: int32(server.Port.Number), Protocol: HTTPS, Hosts: server.Hosts, Secret: secretName, Passthrough: passthrough}
			portProtocols = append(portProtocols, pp)
		} else if server.Port.Protocol == TCP {

			pp := AviPortHostProtocol{Port: int32(server.Port.Number), Protocol: TCP, Hosts: server.Hosts}
			portProtocols = append(portProtocols, pp)
		}
	}
	return portProtocols
}

func (o *AviObjectGraph) generateRandomStringName(vsName string) string {
	// TODO: Watch out for collisions, if need we can increase 10 below.
	random_string := utils.RandomSeq(5)
	// TODO: Find a way to avoid collisions
	utils.AviLog.Info.Printf("Random string generated :%s", random_string)
	pgName := vsName + "-" + random_string
	return pgName
}

func (o *AviObjectGraph) evaluateTCPMatch(matchrule []*networking.L4MatchAttributes) uint32 {
	checksum := utils.Hash(utils.Stringify(matchrule))
	utils.AviLog.Info.Printf("Checksum for the HTTP match rules is %d", checksum)
	return checksum
}

func (o *AviObjectGraph) evaluateHTTPMatch(matchrule []*networking.HTTPMatchRequest) uint32 {
	checksum := utils.Hash(utils.Stringify(matchrule))
	utils.AviLog.Info.Printf("Checksum for the TCP match rules is %d", checksum)
	return checksum
}

func (o *AviObjectGraph) evaluateTLSMatch(matchrule []*networking.TLSMatchAttributes) uint32 {
	checksum := utils.Hash(utils.Stringify(matchrule))
	utils.AviLog.Info.Printf("Checksum for the TLS match rules is %d", checksum)
	return checksum
}

func (o *AviObjectGraph) ProcessDRs(drList []string, poolNode *AviPoolNode, namespace string, subset string) map[string]string {
	for _, drName := range drList {
		found, istioObj := istio_objs.SharedDRLister().DestinationRule(namespace).Get(drName)
		drSpec := istioObj.Spec.(*networking.DestinationRule)
		if found {
			if subset != "" {
				// We need to apply the DR's specific subset rule for this pool
				for _, drSubset := range drSpec.Subsets {
					if subset == drSubset.Name {
						lbSettings := drSubset.TrafficPolicy.LoadBalancer
						o.selectPolicy(poolNode, lbSettings)
						// Return the labels to search for.
						utils.AviLog.Info.Printf("The DR subset labels for the pool %s are: %v", poolNode.Name, drSubset.Labels)
						return drSubset.Labels
					}
				}
			} else {
				lbSettings := drSpec.TrafficPolicy.LoadBalancer
				o.selectPolicy(poolNode, lbSettings)
				return nil
			}
			// TODO: Add support for consistent hash
		} else {
			utils.AviLog.Warning.Printf("DR object not found for DR name: %s", drName)
		}
	}
	return nil
}

func (o *AviObjectGraph) selectPolicy(poolNode *AviPoolNode, lbSettings *networking.LoadBalancerSettings) {
	if lbSettings == nil {
		return
	}
	switch lbSettings.GetSimple() {
	case networking.LoadBalancerSettings_LEAST_CONN:
		poolNode.LbAlgorithm = utils.LeastConnection
	case networking.LoadBalancerSettings_RANDOM:
		// AVI does not support this - let's default to LEAST_CONN
		poolNode.LbAlgorithm = utils.LeastConnection
	case networking.LoadBalancerSettings_ROUND_ROBIN:
		poolNode.LbAlgorithm = utils.RoundRobinConnection
	case networking.LoadBalancerSettings_PASSTHROUGH:
		// AVI does not support this - let's default to LEAST_CONN
		poolNode.LbAlgorithm = utils.LeastConnection
	}
	return
}

func (o *AviObjectGraph) evaluateHTTPPools(ns string, randString string, destinations []*networking.HTTPRouteDestination, gatewayNs string) []*AviPoolNode {
	var poolNodes []*AviPoolNode
	for _, destination := range destinations {
		poolNode := o.evaluateDestinations(destination.Destination, destination.Weight, gatewayNs, ns, randString, HTTP)
		poolNodes = append(poolNodes, poolNode)
	}
	utils.AviLog.Info.Printf("Evaluated HTTP Pools: %v", utils.Stringify(poolNodes))
	return poolNodes
}

func (o *AviObjectGraph) evaluateTCPPools(ns string, randString string, destinations []*networking.RouteDestination, gatewayNs string) []*AviPoolNode {
	var poolNodes []*AviPoolNode
	for _, destination := range destinations {
		poolNode := o.evaluateDestinations(destination.Destination, destination.Weight, gatewayNs, ns, randString, TCP)
		poolNodes = append(poolNodes, poolNode)
	}
	utils.AviLog.Info.Printf("Evaluated TCP Pools: %v", utils.Stringify(poolNodes))
	return poolNodes
}

func (o *AviObjectGraph) evaluateDestinations(destination *networking.Destination, weight int32, gatewayNs string, ns string, randString string, protocol string) *AviPoolNode {
	var labels map[string]string
	// For each destination, generate one pool. If weight is not present evalute it to 100.
	serviceName := destination.Host

	// TODO : Right now we are not handling weight - we will do this in the future. Remove this TODO once done.
	if weight == 0 {
		weight = 100
	}
	portName := destination.Port.GetName()
	portNumber := int32(destination.Port.GetNumber())
	var poolName string
	if destination.Subset != "" {
		poolName = serviceName + "-" + destination.Subset + "-" + randString
	} else {
		poolName = serviceName + "-" + randString
	}
	// To be supported: obtain the servers for this service. Naming convention of the service is - svcname.ns.sub-domain
	poolNode := &AviPoolNode{Name: poolName, Tenant: gatewayNs, Port: portNumber, Protocol: protocol}
	epObj, err := utils.GetInformers().EpInformer.Lister().Endpoints(ns).Get(serviceName)
	// Get the destination rules for this service
	found, destinationRules := istio_objs.SharedSvcLister().Service(ns).GetSvcToDR(serviceName)
	utils.AviLog.Info.Printf(" Destination rules :%v obtained for service :%s", destinationRules, serviceName)
	if found {
		// We need to process Destination Rules for this service
		labels = o.ProcessDRs(destinationRules, poolNode, ns, destination.Subset)
	}
	if err != nil || epObj == nil {
		// There's no endpoint object for the service.
		poolNode.Servers = nil
	} else {
		poolNode.Servers = o.extractServers(epObj, portNumber, portName, destination.Subset, ns, labels)
	}
	if portName != "" {
		poolNode.PortName = portName
	} else if portNumber != 0 {
		poolNode.Port = portNumber
	}
	poolNode.CalculateCheckSum()
	o.GraphChecksum = o.GraphChecksum + poolNode.GetCheckSum()
	utils.AviLog.Info.Printf("Computed Graph Checksum after calculating pool nodes is :%v", o.GraphChecksum)
	o.AddModelNode(poolNode)
	return poolNode
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

func (o *AviObjectGraph) evaluateHTTPMatchCriteria(matches []*networking.HTTPMatchRequest) []MatchCriteria {
	var matchCriteria []MatchCriteria
	for _, match := range matches {
		out := translateRoutePathMatch(match)
		matchCriteria = append(matchCriteria, out)
	}
	return matchCriteria
}

func contains(s []int32, e int32) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (o *AviObjectGraph) evaluateTCPMatchCriteria(matches []*networking.L4MatchAttributes, filterPort int32) []string {
	var ports []string
	for _, match := range matches {
		if filterPort == int32(match.Port) {
			ports = append(ports, fmt.Sprint(match.Port))
		}
	}
	return ports
}

func (o *AviObjectGraph) evaluateTLSMatchCriteria(matches []*networking.TLSMatchAttributes, vsNode *AviVsNode, destinations []*networking.RouteDestination) {
	for _, match := range matches {
		randString := o.generateRandomStringName(fmt.Sprint(match.Port))
		// Each Match criteria forms a SNI child
		tlsVSNode := &AviVsTLSNode{Name: "tls-passthrough-" + randString, TLSType: "PASSTHROUGH", VHParentName: vsNode.Name, Tenant: vsNode.Tenant, VHDomainNames: match.SniHosts}
		tlsVSNode.AviVsNode = vsNode
		pools := o.evaluateTLSPools(tlsVSNode.Tenant, "tls-pool-"+randString, destinations)
		o.GraphChecksum = o.GraphChecksum + tlsVSNode.GetCheckSum()
		if len(pools) == 1 {
			tlsVSNode.DefaultPool = pools[0].Name
		}
		tlsVSNode.PoolRefs = pools
		o.AddModelNode(tlsVSNode)
	}
}

func checkPGExistsInModel(pgNodes []*AviPoolGroupNode, evalchecksum uint32) (bool, *AviPoolGroupNode) {
	// Iterate through the PG nodes and check if the node exists with the checksum.
	for _, pgNode := range pgNodes {
		if pgNode.RuleChecksum == evalchecksum {
			//Node exists - return true
			return true, pgNode
		}
	}
	return false, nil
}

func (o *AviObjectGraph) ConstructAviHTTPPGPoolNodes(vs *istio_objs.IstioObject, model_name string, AviVsName string, gatewayNs string, vsNode AviModelNode, isSniNode bool) []*AviPoolGroupNode {
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
	utils.AviLog.Info.Printf("Processing VS object with name :%s. SNI node: %v", vsName, isSniNode)
	var poolGroupNodes []*AviPoolGroupNode
	var prevPoolGroupNodes []utils.NamespaceName
	var prevModelPoolGroupNodes []*AviPoolGroupNode
	// Fetch the model if it exists for the AVI Vs.
	cache := utils.SharedAviObjCache()
	vsKey := utils.NamespaceName{Namespace: gatewayNs, Name: AviVsName}
	vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
	vs_cache_obj, ok := vs_cache.(*utils.AviVsCache)
	if ok {
		// There's a VS Cache - let's check the PGs
		if vs_cache_obj.PGKeyCollection != nil {
			prevPoolGroupNodes = vs_cache_obj.PGKeyCollection
		}
	}

	found, aviModel := objects.SharedAviGraphLister().Get(model_name)
	if found && aviModel != nil {
		if !isSniNode {
			prevModelPoolGroupNodes = aviModel.(*AviObjectGraph).GetAviVS()[0].PoolGroupRefs
			utils.AviLog.Info.Printf("Evaluating for non-SNI VS. The prevModel PGs are: %v", prevModelPoolGroupNodes)
		} else {
			for _, sniNode := range aviModel.(*AviObjectGraph).GetAviSNIVS() {
				utils.AviLog.Info.Printf("Evaluating for SNI VS. The prevModel PGs are: %v", prevModelPoolGroupNodes)
				prevModelPoolGroupNodes = append(prevModelPoolGroupNodes, sniNode.PoolGroupRefs...)
			}
		}
	}

	// HTTP route handling.

	for _, httpRoute := range vsObj.Http {
		// Generate the PG to Rules map
		var pgName string
		var pgNamePrefix string
		rulechecksum := o.evaluateHTTPMatch(httpRoute.Match)
		// Check if the PG already exists or needs to be created
		if !isSniNode {
			pgNamePrefix = gatewayNs + "-" + vsName + "-"
		} else {
			pgNamePrefix = "tls-" + gatewayNs + "-" + vsName + "-"
		}
		pgNameToSearch := pgNamePrefix + fmt.Sprint(rulechecksum)
		for _, pgNodeNsName := range prevPoolGroupNodes {
			if pgNodeNsName.Name == pgNameToSearch {
				utils.AviLog.Info.Printf("Found PG in cache, re-using the same name: %s", pgNameToSearch)
				pgName = pgNodeNsName.Name
			}
		}
		//exists, presentPGNode := checkPGExistsInCache(prevPoolGroupNodes, pgNamePrefix)
		if pgName == "" {
			// Check if it exists in the in memory model or not.
			found, pgNodeFromModel := checkPGExistsInModel(prevModelPoolGroupNodes, rulechecksum)
			if found {
				utils.AviLog.Info.Printf("Found PG in the model nodes that has a match checksum, using name :%s", pgNodeFromModel.Name)
				pgName = pgNodeFromModel.Name
			} else {
				utils.AviLog.Info.Printf("PG : %s not found in cache or model, generating new PG name", pgNameToSearch)
				pgName = o.generateRandomStringName(pgNamePrefix)
			}
		} else {
			utils.AviLog.Info.Printf("The PG %s exists in cache with the same checksum", pgName)
		}
		matchList := o.evaluateHTTPMatchCriteria(httpRoute.Match)
		pgNode := &AviPoolGroupNode{Name: pgName, Tenant: gatewayNs, RuleChecksum: rulechecksum, MatchList: matchList}
		// Get the pools for the PG
		pools := o.evaluateHTTPPools(vs.ConfigMeta.Namespace, pgName, httpRoute.Route, gatewayNs)
		for _, pool := range pools {
			pool_ref := fmt.Sprintf("/api/pool?name=%s", pool.Name)
			pgNode.Members = append(pgNode.Members, &avimodels.PoolGroupMember{PoolRef: &pool_ref})
		}
		pgNode.CalculateCheckSum()
		o.GraphChecksum = o.GraphChecksum + pgNode.GetCheckSum()
		o.AddModelNode(pgNode)
		utils.AviLog.Info.Printf("Evaluated the PG :%v", utils.Stringify(pgNode))
		utils.AviLog.Info.Printf("Computed Graph Checksum after PG node creation is %v", o.GraphChecksum)
		if !isSniNode {
			vsNode.(*AviVsNode).PoolGroupRefs = append(vsNode.(*AviVsNode).PoolGroupRefs, pgNode)
			vsNode.(*AviVsNode).PoolRefs = append(vsNode.(*AviVsNode).PoolRefs, pools...)
			utils.AviLog.Info.Printf("Updating PoolGroupRefs for Parent Node :%v", len(vsNode.(*AviVsNode).PoolGroupRefs))
		} else {
			vsNode.(*AviVsTLSNode).PoolGroupRefs = append(vsNode.(*AviVsTLSNode).PoolGroupRefs, pgNode)
			vsNode.(*AviVsTLSNode).PoolRefs = append(vsNode.(*AviVsTLSNode).PoolRefs, pools...)
			utils.AviLog.Info.Printf("Updating PoolGroupRefs for SNI child Node :%v", len(vsNode.(*AviVsTLSNode).PoolGroupRefs))
		}
		poolGroupNodes = append(poolGroupNodes, pgNode)
	}
	// Right now the assumption is that the TLS SNI Child will have only one pool, hence there's no need of a PG
	// But if we have requirement where there are more than one pool, then we should change this code to include a PG.
	//o.ConstructTLSPassthroughPGs(vsObj, gwNs, vsNode)
	return poolGroupNodes
}

func (o *AviObjectGraph) ConstructAviTCPPGPoolNodes(vs *istio_objs.IstioObject, model_name string, AviVsName string, gatewayNs string, vsNode *AviVsNode, filterPort int32) {
	vsObj, _ := vs.Spec.(*networking.VirtualService)
	var prevTCPModelPoolGroupNodes []*AviPoolGroupNode
	var prevTCPPoolGroupNodesInCache []utils.NamespaceName
	found, aviModel := objects.SharedAviGraphLister().Get(model_name)
	if found && aviModel != nil {
		prevTCPModelPoolGroupNodes = aviModel.(*AviObjectGraph).GetAviVS()[0].TCPPoolGroupRefs
		utils.AviLog.Info.Printf("Evaluating TCP Pool Groups. The prevModel PGs are: %v", prevTCPModelPoolGroupNodes)

	}
	cache := utils.SharedAviObjCache()
	vsKey := utils.NamespaceName{Namespace: gatewayNs, Name: AviVsName}
	vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
	vs_cache_obj, ok := vs_cache.(*utils.AviVsCache)
	if ok {
		// There's a VS Cache - let's check the PGs
		if vs_cache_obj.PGKeyCollection != nil {
			prevTCPPoolGroupNodesInCache = vs_cache_obj.PGKeyCollection
		}
	}
	for _, tcpRoute := range vsObj.Tcp {
		// (sudswas): We don't know what else can be a TCP match criteria than ports. So let's assume we just have to care about ports.
		ports := o.evaluateTCPMatchCriteria(tcpRoute.Match, filterPort)

		if ports == nil {
			utils.AviLog.Info.Printf("This TCP route for VS :%s, has no ports that qualify as per the gateway ports: %v", vs.ConfigMeta.Name, ports)
			continue
		} else if len(ports) > 1 {
			utils.AviLog.Warning.Printf("This TCP route for VS :%s, has more than one port: %v", vs.ConfigMeta.Name, len(ports))
		}

		pgNamePrefix := "tcp-" + fmt.Sprint(filterPort) + "-"

		var pgName string
		// Check if the NamePrefix exists or not
		for _, pgNodeNsName := range prevTCPPoolGroupNodesInCache {
			if strings.HasPrefix(pgNodeNsName.Name, pgNamePrefix) {
				pgName = pgNodeNsName.Name
			}
		}
		// Check if the PG name is present in the model cache.
		if pgName == "" {
			for _, pgNode := range prevTCPModelPoolGroupNodes {
				if strings.HasPrefix(pgNode.Name, pgNamePrefix) {
					pgName = pgNode.Name
				}
			}
		}
		// If the PGName was not found in the cache, generate the name
		if pgName == "" {
			pgName = o.generateRandomStringName(pgNamePrefix)
		}
		pgNode := &AviPoolGroupNode{Name: pgName, Tenant: gatewayNs, Port: fmt.Sprint(filterPort)}
		poolNodes := o.evaluateTCPPools(vs.ConfigMeta.Namespace, pgName, tcpRoute.Route, gatewayNs)
		for _, pool := range poolNodes {
			pool_ref := fmt.Sprintf("/api/pool?name=%s", pool.Name)
			pgNode.Members = append(pgNode.Members, &avimodels.PoolGroupMember{PoolRef: &pool_ref})
		}
		vsNode.PoolRefs = append(vsNode.PoolRefs, poolNodes...)
		utils.AviLog.Info.Printf("Evaluated TCP pool group values :%v", utils.Stringify(pgNode))
		vsNode.TCPPoolGroupRefs = append(vsNode.TCPPoolGroupRefs, pgNode)
	}

}

func (o *AviObjectGraph) evaluateTLSPools(ns string, randString string, destinations []*networking.RouteDestination) []*AviPoolNode {
	var poolNodes []*AviPoolNode
	for _, destination := range destinations {
		var labels map[string]string
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
		// TODO(sudswas): Does the protocol need to change?
		poolNode := &AviPoolNode{Name: poolName, Tenant: ns, Port: portNumber, Protocol: HTTP}
		epObj, err := utils.GetInformers().EpInformer.Lister().Endpoints(ns).Get(serviceName)
		// Get the destination rules for this service
		found, destinationRules := istio_objs.SharedSvcLister().Service(ns).GetSvcToDR(serviceName)
		utils.AviLog.Info.Printf(" Destination rules :%v obtained for service :%s", destinationRules, serviceName)
		if found {
			// We need to process Destination Rules for this service
			labels = o.ProcessDRs(destinationRules, poolNode, ns, destination.Destination.Subset)
		}
		if err != nil || epObj == nil {
			// There's no endpoint object for the service.
			poolNode.Servers = nil
		} else {
			poolNode.Servers = o.extractServers(epObj, portNumber, portName, destination.Destination.Subset, ns, labels)
		}
		if portName != "" {
			poolNode.PortName = portName
		} else if portNumber != 0 {
			poolNode.Port = portNumber
		}
		poolNode.CalculateCheckSum()
		o.GraphChecksum = o.GraphChecksum + poolNode.GetCheckSum()
		utils.AviLog.Info.Printf("Computed Graph Checksum after calculating pool nodes is :%v", o.GraphChecksum)
		poolNodes = append(poolNodes, poolNode)
	}
	utils.AviLog.Info.Printf("Evaluated TLS Pools: %v", utils.Stringify(poolNodes))
	return poolNodes
}

func (o *AviObjectGraph) ConstructTLSPassthroughPGs(vsObj *networking.VirtualService, gatewayNs string, vsNode *AviVsNode) {
	for _, tlsRoute := range vsObj.Tls {
		o.evaluateTLSMatchCriteria(tlsRoute.Match, vsNode, tlsRoute.Route)
	}
}

func (o *AviObjectGraph) ConstructAviVsNode(gwObj *istio_objs.IstioObject) *AviVsNode {
	gatewayName := gwObj.ConfigMeta.Name
	namespace := gwObj.ConfigMeta.Namespace
	gwSpec, _ := gwObj.Spec.(*networking.Gateway)
	// FQDN should come from the cloud. Modify
	avi_vs_meta := &AviVsNode{Name: gatewayName, Tenant: namespace,
		EastWest: false}
	avi_vs_meta.PortProto = o.constructProtocolPortMaps(gwSpec)
	// Default case.
	if avi_vs_meta.ApplicationProfile == "" {
		avi_vs_meta.ApplicationProfile = "System-HTTP"
	}
	// For HTTP it's always System-TCP-Proxy.
	avi_vs_meta.NetworkProfile = "System-TCP-Proxy"
	//o.AddModelNode(avi_vs_meta)
	return avi_vs_meta
}

func matchHttpHosts(vshosts []string, hostprot AviPortHostProtocol) []string {
	// Find out the qualifying hosts that should be part of the VS
	qualifiedHostsMap := make(map[string]bool)
	for _, vshost := range vshosts {
		// Only process the hosts that are either HTTP or are part of Non-passthrough TLS
		if hostprot.Protocol == HTTP || (hostprot.Protocol == HTTPS && !hostprot.Passthrough) {
			for _, host := range hostprot.Hosts {
				if host == "*" || strings.HasSuffix(vshost, strings.Trim(host, "*.")) || host == vshost {
					// Wild card on the gateway port
					if hostprot.Port != 80 && hostprot.Port != 443 {
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

func (o *AviObjectGraph) ConstructAviHttpPolicyNodes(gatewayNs string, vsObj *istio_objs.IstioObject, pgNodes []*AviPoolGroupNode, portHostProto AviPortHostProtocol, isSniNode bool) *AviHttpPolicySetNode {
	// Extract the hosts from the vsObj
	vsSpec := vsObj.Spec.(*networking.VirtualService)
	var httpPolicySet []AviHostPathPortPoolPG
	for _, pgNode := range pgNodes {
		// Let's figure out the host headers for each host.
		hosts := matchHttpHosts(vsSpec.Hosts, portHostProto)
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
	var httppolname string
	if isSniNode {
		httppolname = "tls-" + vsObj.ConfigMeta.Name + "-" + fmt.Sprint(portHostProto.Port)
		utils.AviLog.Info.Printf("Evaluating an SNI node, hence the HTTP policy name is  %s", httppolname)

	} else {
		httppolname = vsObj.ConfigMeta.Name + "-" + fmt.Sprint(portHostProto.Port)
	}
	policyNode := &AviHttpPolicySetNode{Name: httppolname, HppMap: httpPolicySet, Tenant: gatewayNs}
	policyNode.CalculateCheckSum()
	o.GraphChecksum = o.GraphChecksum + policyNode.GetCheckSum()
	utils.AviLog.Info.Printf("The value of HTTP Policy Set is :%s", utils.Stringify(policyNode))
	utils.AviLog.Info.Printf("Computed Checksum for HTTP Policy Set is %v", policyNode.GetCheckSum())
	utils.AviLog.Info.Printf("Computed Graph Checksum after evaluating nodes for HTTP Policy Set is %v", o.GraphChecksum)
	o.AddModelNode(policyNode)
	return policyNode
}

func (o *AviObjectGraph) extractServers(epObj *corev1.Endpoints, port_num int32, port_name string, subsets string, ns string, subsetLabels map[string]string) []AviPoolMetaServer {
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
				if subsets != "" {
					// Only qualify the servers that are part of the subsets
					podObj, err := utils.GetInformers().PodInformer.Lister().Pods(ns).Get(addr.TargetRef.Name)
					utils.AviLog.Info.Printf("The Pod Object labels during subset calculations is :%v and the subset labels from DR are: %v", podObj.Labels, subsetLabels)
					if err == nil {
						for labelkey, label := range podObj.Labels {
							for subset_key, subset_label := range subsetLabels {
								if labelkey == subset_key && label == subset_label {
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
									if !utils.HasElem(pool_meta, server) {
										pool_meta = append(pool_meta, server)
									}
								}
								utils.AviLog.Info.Printf("The POD object labels :%s", label)
							}
						}
					}

				} else {
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
	}
	return pool_meta
}

func (o *AviObjectGraph) CreatePortClassifiedObjects(vsNode *AviVsNode, namespace string, gatewayNs string, gatewayName string, gwObj *istio_objs.IstioObject) {
	var keycertMap map[string][]byte
	//This means we need to create cert and key
	tcpProcessed := false
	utils.AviLog.Info.Printf("Portocol Port mapping for this Gateway :%s is: %s", gatewayName, utils.Stringify(vsNode.PortProto))
	for _, pp := range vsNode.PortProto {
		if pp.Protocol == HTTPS {
			if pp.Secret != "" {
				// Let's retrieve the secret object
				secretObj, err := utils.GetInformers().SecretInformer.Lister().Secrets(vsNode.Tenant).Get(pp.Secret)
				if err != nil || secretObj == nil {
					// We should check if there is a need to process a secret delete here.
					utils.AviLog.Info.Printf("Secret Object not found for secret: %s", pp.Secret)
					continue
				} else {

					vsNode.ApplicationProfile = "System-Secure-HTTP"
					tlsVSNode := &AviVsTLSNode{Name: "tls-" + vsNode.Name, VHParentName: vsNode.Name, VHDomainNames: pp.Hosts, Tenant: vsNode.Tenant}
					tlsVSNode.AviVsNode = vsNode
					//avi_vs_meta.TLSProp = tlsProp
					vsNode.SNIParent = true
					tlsNode := &AviTLSKeyCertNode{Name: vsNode.Name + "-" + fmt.Sprint(pp.Port), Tenant: vsNode.Tenant, Port: pp.Port}
					keycertMap = secretObj.Data
					cert, ok := keycertMap["cert"]
					if ok {
						tlsNode.Cert = cert
					} else {
						utils.AviLog.Info.Printf("Certificate not found for secret: %s", secretObj.Name)
					}
					key, keyfound := keycertMap["key"]
					if keyfound {
						tlsNode.Key = key
					} else {
						utils.AviLog.Info.Printf("Key not found for secret: %s", secretObj.Name)
					}
					tlsVSNode.TLSKeyCert = append(tlsVSNode.TLSKeyCert, tlsNode)
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

								// If a VS is found for this gateway - first check if the hosts in the gateway match the ones on the VS.
								// If it does not match, then don't process it.
								vsSpec := vsObj.Spec.(*networking.VirtualService)
								// (TODO:) Support for only HTTPS based hosts in the VirtualService.
								hosts := matchHttpHosts(vsSpec.Hosts, pp)
								if len(hosts) == 0 {
									// This VS has no eligible hosts. We should return. TODO: Check if we should not even create the PGs in that case.
									utils.AviLog.Info.Printf("No matching SNI hosts found for this VS: %s during http eval for parent VS", vsName)
									continue
								}
								model_name := gatewayNs + "/" + gatewayName
								utils.AviLog.Info.Printf("Calculating PG Nodes for SNI child")
								PGNodes := o.ConstructAviHTTPPGPoolNodes(vsObj, model_name, tlsVSNode.Name, gatewayNs, tlsVSNode, true)
								// Now let's Build the HTTP policy set. More checks here for 'type' of route.
								var hostPortList []AviPortHostProtocol
								hostPortList = append(hostPortList, pp)
								httpPolicyNode := o.ConstructAviHttpPolicyNodes(gatewayNs, vsObj, PGNodes, pp, true)
								if httpPolicyNode != nil {
									tlsVSNode.HttpPoolRefs = append(vsNode.HttpPoolRefs, httpPolicyNode)
									tlsVSNode.HTTPChecksum = httpPolicyNode.GetCheckSum()
								}
							}
						}
					}
					tlsVSNode.SSLKeyCertRefs = append(tlsVSNode.SSLKeyCertRefs, tlsNode)
					o.AddModelNode(tlsVSNode)
				}

			} else if pp.Passthrough {
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
							//model_name := gatewayNs + "/" + gatewayName
							o.ConstructTLSPassthroughPGs(vsObj.Spec.(*networking.VirtualService), gatewayNs, vsNode)
						}
					}

				}
				// This is a TLS PASSTHROUGH case. We need the SNI child but no certs.
				vsNode.ApplicationProfile = "System-Secure-HTTP"
				vsNode.SNIParent = true
				//o.AddModelNode(tlsVSNode)
			}
		} else if pp.Protocol == HTTP && !pp.Redirect {
			utils.AviLog.Info.Printf("Evaluating a HTTP route for Parent VS for port: %v for hosts :%s", pp.Port, pp.Hosts)
			// If we find the HTTP protocol on the Gateway, we should process it only once.
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
						// If a VS is found for this gateway - first check if the hosts in the gateway match the ones on the VS.
						// If it does not match, then don't process it.
						vsSpec := vsObj.Spec.(*networking.VirtualService)
						// (TODO:) Support for only HTTP based hosts in the VirtualService.

						hosts := matchHttpHosts(vsSpec.Hosts, pp)
						if len(hosts) == 0 {
							// This VS has no eligible hosts. We should return. TODO: Check if we should not even create the PGs in that case.
							utils.AviLog.Info.Printf("No matching HTTP hosts found for this VS: %s during http eval for parent VS", vsName)
							continue
						}
						model_name := gatewayNs + "/" + gatewayName
						utils.AviLog.Info.Printf("contructing PGs for parent VS for HTTP route for VS :%s", vsName)
						PGNodes := o.ConstructAviHTTPPGPoolNodes(vsObj, model_name, gatewayName, gatewayNs, vsNode, false)
						// Now let's Build the HTTP policy set. More checks here for 'type' of route.
						httpPolicyNode := o.ConstructAviHttpPolicyNodes(gatewayNs, vsObj, PGNodes, pp, false)
						if httpPolicyNode != nil {
							vsNode.HttpPoolRefs = append(vsNode.HttpPoolRefs, httpPolicyNode)
							vsNode.HTTPChecksum = httpPolicyNode.GetCheckSum()
						}
					}
				}
			} else {
				utils.AviLog.Info.Printf("Gateway to Virtual Service relationships not found for Gateway: %s/%s", gatewayNs, gatewayName)
			}
		} else if pp.Protocol == TCP && !tcpProcessed {
			utils.AviLog.Info.Printf("Evaluating a TCP route for Parent VS")
			// If we find the HTTP protocol on the Gateway, we should process it only once.
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
						// If a VS is found for this gateway - first check if the hosts in the gateway match the ones on the VS.
						// If it does not match, then don't process it.
						vsSpec := vsObj.Spec.(*networking.VirtualService)
						// (TODO:) Support for only HTTP based hosts in the VirtualService.
						if vsSpec.Tcp != nil {
							// For TCP connections - the host field on the Virtual Service should be set to *
							model_name := gatewayNs + "/" + gatewayName
							o.ConstructAviTCPPGPoolNodes(vsObj, model_name, gatewayName, gatewayNs, vsNode, pp.Port)
						}
					}
				}
			}
			tcpProcessed = true
		} else if pp.Protocol == HTTP && pp.Redirect {
			utils.AviLog.Info.Printf("Evaluating a HTTP redirect for Port: %v on Parent VS", pp.Port)
			// Fetch the re-direct hosts and map it to various ports on the gateway where the traffic should be re-directed.
			redirPortToHost := make(map[int32][]string)
			for _, host := range pp.Hosts {
				// Find out which port should this host be redirected to.
				for _, hostproto := range vsNode.PortProto {
					// Skip the port that belongs to this port itself.
					if hostproto.Port != pp.Port {
						if utils.HasElem(hostproto.Hosts, host) {
							// Record the port
							_, ok := redirPortToHost[hostproto.Port]
							if ok {
								redirPortToHost[hostproto.Port] = append(redirPortToHost[hostproto.Port], host)
							} else {
								redirPortToHost[hostproto.Port] = []string{host}
							}
						}
					}
				}
			}
			utils.AviLog.Info.Printf("The re-direct map : %s", utils.Stringify(redirPortToHost))
			if len(redirPortToHost) != 0 {
				o.ConstructHTTPRedirectPolicies(pp.Port, redirPortToHost, gatewayNs, gatewayName, vsNode)
			}
		}

	}

}

func (o *AviObjectGraph) ConstructHTTPRedirectPolicies(vsport int32, redirPortToHost map[int32][]string, gatewayNs string, gatewayName string, vsNode *AviVsNode) {
	httpPolicyName := "redirect-" + gatewayNs + "-" + gatewayName + fmt.Sprint(vsport)
	var AviRedirectPortList []AviRedirectPort
	for port, hosts := range redirPortToHost {
		redirPort := AviRedirectPort{Hosts: hosts, RedirectPort: port, StatusCode: "HTTP_REDIRECT_STATUS_CODE_301", VsPort: vsport}
		AviRedirectPortList = append(AviRedirectPortList, redirPort)
	}
	httpPolicyNode := &AviHttpPolicySetNode{Name: httpPolicyName, RedirectPorts: AviRedirectPortList, Tenant: gatewayNs}
	vsNode.HttpPoolRefs = append(vsNode.HttpPoolRefs, httpPolicyNode)
	httpPolicyNode.CalculateCheckSum()
	o.GraphChecksum = o.GraphChecksum + httpPolicyNode.GetCheckSum()
	o.AddModelNode(httpPolicyNode)
}

func (o *AviObjectGraph) BuildAviObjectGraph(namespace string, gatewayNs string, gatewayName string, gwObj *istio_objs.IstioObject) {
	// We use the gateway fields to arrive at various AVI VS Node object.
	var VsNode *AviVsNode

	VsNode = o.ConstructAviVsNode(gwObj)
	o.CreatePortClassifiedObjects(VsNode, namespace, gatewayNs, gatewayName, gwObj)
	o.AddModelNode(VsNode)
	VsNode.CalculateCheckSum()
	o.GraphChecksum = o.GraphChecksum + VsNode.GetCheckSum()
	utils.AviLog.Info.Printf("Checksum  for AVI VS object %v", VsNode.GetCheckSum())
	utils.AviLog.Info.Printf("Computed Graph Checksum for VS is: %v", o.GraphChecksum)
}
