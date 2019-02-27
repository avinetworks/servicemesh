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

package k8s

import (
	"fmt"
	"strconv"
	"strings"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/aviobjects"
	"github.com/avinetworks/servicemesh/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

/* AviCache for storing * Service to E/W Pools and Route/Ingress Pools.
 * Of the form:
 * map[{namespace: string, name: string}]map[pool_name_prefix:string]bool
 */

type K8sEp struct {
	avi_obj_cache        *utils.AviObjCache
	avi_rest_client_pool *utils.AviRestClientPool
	informers            *utils.Informers
}

func NewK8sEp(avi_obj_cache *utils.AviObjCache, avi_rest_client_pool *utils.AviRestClientPool,
	inf *utils.Informers) *K8sEp {
	p := K8sEp{}
	p.avi_obj_cache = avi_obj_cache
	p.avi_rest_client_pool = avi_rest_client_pool
	p.informers = inf
	return &p
}

/*
 * This function is called on a CU of Endpoint event handler or on a CU of a
 * Service, Route or Ingress.
 * 1) name_prefix will be "" if called directly from ep event handler. In such
 * cases, simply update existing pg/pools affected by the Endpoint. Ignore if
 * pg/pool doesn't exist
 * 2) name_prefix will be non-nil if called from the CU of a Service, Route or
 * Ingress. In such cases, CU the pg/pool
 * TODO: PoolGroup, MicroService, MicroServiceGroup, etc.
 */

func (p *K8sEp) K8sObjCrUpd(shard uint32, ep *corev1.Endpoints,
	name_prefix string, crud_hash_key string) ([]*utils.RestOp, error) {
	/*
	 * Endpoints.Subsets is an array with each subset having a list of
	 * ready/not-ready addresses and ports. If a endpoint has 2 ports, one ready
	 * and the other not ready, there will be 2 subset elements with the same
	 * IP as "ready" in one element and "not ready" in the other. Same IP won't
	 * be present in both ready and not ready in same element
	 *
	 * Create a list of all ports with ready endpoints. Lookup Service object
	 * in cache and extract targetPort from Service Object. If Service isnt
	 * present yet, wait for it to be synced
	 */

	port_protocols := make(map[utils.AviPortStrProtocol]bool)
	svc, err := p.informers.ServiceInformer.Lister().Services(ep.Namespace).Get(ep.Name)
	if err != nil {
		utils.AviLog.Warning.Printf(`Service for Endpoint Namespace %v Name %v 
            doesn't exist`, ep.Namespace, ep.Name)
		return nil, &utils.SkipSyncError{"Skip sync"}
	}

	tenant := ep.Namespace
	for _, ss := range ep.Subsets {
		if len(ss.Addresses) > 0 {
			/*
			 * If name is present in EndpointPort, try to match with name in
			 * ServicePort and return corresponding targetPort. If name is
			 * absent, there's just a single port. Return that targetPort
			 */
			for _, ep_port := range ss.Ports {
				var tgt_port string
				if ep_port.Name != "" {
					tgt_port = func(svc *corev1.Service, name string) string {
						for _, pp := range svc.Spec.Ports {
							if pp.Name == name {
								return pp.TargetPort.String()
							}
						}
						utils.AviLog.Warning.Printf(`Matching name %v not found 
                                in Svc namespace %s name %s Ports %v`, name,
							svc.Namespace, svc.Name, svc.Spec.Ports)
						return ""
					}(svc, ep_port.Name)

					if tgt_port == "" {
						utils.AviLog.Warning.Printf(`Matching port %v name %v not 
                                found in Svc`, ep_port.Port, ep_port.Name)
						return nil, nil
					}
				} else {
					tgt_port = svc.Spec.Ports[0].TargetPort.String()
				}

				var prot string
				if string(ep_port.Protocol) == "" {
					prot = "tcp" // Default
				} else {
					prot = strings.ToLower(string(ep_port.Protocol))
				}
				pp := utils.AviPortStrProtocol{Port: tgt_port, Protocol: prot}
				port_protocols[pp] = true
			}
		}
	}

	var pool_names []string
	var pools_service_metadata utils.ServiceMetadataObj

	process_pool := true
	k := utils.NamespaceName{Namespace: ep.Namespace, Name: ep.Name}
	/*
	 * If  name_prefix is nil, this is a Ep CU event from event handler; See
	 * if pools/pgs exist already. If so, let's perform U
	 */
	if name_prefix == "" {
		var pools_cache interface{}
		var pools map[interface{}]bool
		var ok bool
		pools_cache, process_pool = p.avi_obj_cache.SvcToPoolCache.AviMultiCacheGetKey(k)
		pools, ok = pools_cache.(map[interface{}]bool)
		if process_pool && ok {
			// ppool_name is of the form service/name-pool-http-tcp, ingress/name-pool-http-tcp
			for ppool_name_intf := range pools {
				ppool_name, ok := ppool_name_intf.(string)
				if !ok {
					utils.AviLog.Warning.Printf("ppool_name_intf %T not string",
						ppool_name_intf)
					continue
				}
				elems := strings.Split(ppool_name, "/")
				pool_name := elems[1]
				pool_names = append(pool_names, pool_name)
				var pool_cache interface{}
				pool_key := utils.NamespaceName{Namespace: tenant, Name: pool_name}
				pool_cache, ok1 := p.avi_obj_cache.PoolCache.AviCacheGet(pool_key)
				if !ok1 {
					utils.AviLog.Warning.Printf(`Pool %s not present in Obj cache but
                                           present in Pool cache`, pool_name)
				} else {
					pool_cache_obj, ok := pool_cache.(*utils.AviPoolCache)
					if ok {
						pools_service_metadata = pool_cache_obj.ServiceMetadata
					} else {
						utils.AviLog.Warning.Printf("Pool %s cache incorrect type",
							pool_name)
						pools_service_metadata = utils.ServiceMetadataObj{}
					}
				}
			}
		}
	} else {
		for pp, _ := range port_protocols {
			pool_name := fmt.Sprintf("%s-pool-%v-%s", name_prefix, pp.Port,
				pp.Protocol)
			pool_names = append(pool_names, pool_name)
		}
		pools_service_metadata = utils.ServiceMetadataObj{CrudHashKey: crud_hash_key}
	}
	// TODO (sudswas): We always assume that endpoints would be there in the cache if it's an endpoint event.
	// This may not be the case, if initially the service didn't have pods - and hence the ep.Subsets was empty.
	// Fix this by creating a pool/poolgroup for every service even if the endpoint subset is empty but the targetPorts are present.
	if !process_pool {
		utils.AviLog.Info.Printf("Endpoint %v is not present in Pool/Pg cache.", k)
		return nil, nil
	}

	var rest_ops []*utils.RestOp

	for _, pool_name := range pool_names {
		// Check if resourceVersion is same as cksum from cache. If so, skip upd
		pool_key := utils.NamespaceName{Namespace: tenant, Name: pool_name}
		pool_cache, ok := p.avi_obj_cache.PoolCache.AviCacheGet(pool_key)
		if !ok {
			utils.AviLog.Warning.Printf("Namespace %s Pool %s not present in Pool cache",
				tenant, pool_name)
		} else {
			pool_cache_obj, ok := pool_cache.(*utils.AviPoolCache)
			if ok {
				if ep.ResourceVersion == pool_cache_obj.CloudConfigCksum {
					utils.AviLog.Info.Printf("Pool namespace %s name %s has same cksum %s",
						tenant, pool_name, ep.ResourceVersion)
					continue
				} else {
					utils.AviLog.Info.Printf(`Pool namespace %s name %s has diff 
                            cksum %s resourceVersion %s`, tenant, pool_name,
						pool_cache_obj.CloudConfigCksum, ep.ResourceVersion)
				}
			} else {
				utils.AviLog.Warning.Printf("Pool %s cache incorrect type", pool_name)
			}
		}
		pool_meta := utils.K8sAviPoolMeta{Name: pool_name,
			Tenant:           tenant,
			ServiceMetadata:  pools_service_metadata,
			CloudConfigCksum: ep.ResourceVersion}
		s := strings.Split(pool_name, "-pool-")
		s1 := strings.Split(s[1], "-")
		port := s1[0]
		port_num, _ := strconv.Atoi(port)
		protocol := s1[1]
		pool_meta.Protocol = protocol
		if len(ep.Subsets) == 0 {
			// If this is an update on existing pool that had servers earlier but now the endpoint
			// update has made the subsets to 0, we must set all the servers to 0 for all the pools for this service.
			pool_meta.Servers = pool_meta.Servers[:0]
		}
		for _, ss := range ep.Subsets {
			var epp_port int32
			port_match := false
			for _, epp := range ss.Ports {
				if ((int32(port_num) == epp.Port) || (port == epp.Name)) &&
					(protocol == strings.ToLower(string(epp.Protocol))) {
					port_match = true
					epp_port = epp.Port
					break
				}
			}
			if port_match {
				pool_meta.Port = epp_port
				for _, addr := range ss.Addresses {
					var atype string
					ip := addr.IP
					if utils.IsV4(addr.IP) {
						atype = "V4"
					} else {
						atype = "V6"
					}
					a := avimodels.IPAddr{Type: &atype, Addr: &ip}
					server := utils.AviPoolMetaServer{Ip: a}
					if addr.NodeName != nil {
						server.ServerNode = *addr.NodeName
					}
					pool_meta.Servers = append(pool_meta.Servers, server)
				}
			}
		}
		rest_op := aviobjects.AviPoolBuild(&pool_meta)
		rest_ops = append(rest_ops, rest_op)
	}
	rest_ops = p.CreatePoolGroup(name_prefix, port_protocols, tenant, rest_ops, ep, crud_hash_key)
	if name_prefix == "" {
		p.avi_rest_client_pool.AviRestOperate(p.avi_rest_client_pool.AviClient[shard], rest_ops)
		for _, rest_op := range rest_ops {
			if rest_op.Err == nil && rest_op.Model == "Pool" {
				aviobjects.AviPoolCacheAdd(p.avi_obj_cache.PoolCache, rest_op)
			} else if rest_op.Err == nil && rest_op.Model == "PoolGroup" {
				if rest_op.Err == nil && rest_op.Model == "PoolGroup" {
					aviobjects.AviPGCacheAdd(p.avi_obj_cache.PgCache, rest_op)
				}
			}
		}
		return nil, nil
	} else {
		return rest_ops, nil
	}
}

func (p *K8sEp) CreatePoolGroup(name_prefix string, port_protocols map[utils.AviPortStrProtocol]bool, tenant string, rest_ops []*utils.RestOp, ep *corev1.Endpoints, crud_hash_key string) []*utils.RestOp {
	key := utils.NamespaceName{Namespace: ep.Namespace, Name: ep.Name}
	var pg_names []string
	var pgs map[interface{}]bool
	var pg_cache interface{}
	var pgs_service_metadata utils.ServiceMetadataObj
	process_pg := true
	var ok bool
	if name_prefix == "" {
		pg_cache, process_pg = p.avi_obj_cache.SvcToPgCache.AviMultiCacheGetKey(key)
		pgs, ok = pg_cache.(map[interface{}]bool)
		if process_pg && ok {
			for pg_name_intf := range pgs {
				ppg_name, ok := pg_name_intf.(string)
				if !ok {
					utils.AviLog.Warning.Printf("pg_name_intf %T not string",
						pg_name_intf)
					continue
				}
				elems := strings.Split(ppg_name, "/")
				pg_name := elems[1]
				pg_names = append(pg_names, pg_name)
				var pg_cache interface{}
				pg_key := utils.NamespaceName{Namespace: tenant, Name: pg_name}
				pg_cache, ok1 := p.avi_obj_cache.PgCache.AviCacheGet(pg_key)
				if !ok1 {
					utils.AviLog.Warning.Printf(`PG %s not present in Obj cache but
										present in PG cache`, pg_name)
				} else {
					pg_cache_obj, ok := pg_cache.(*utils.AviPGCache)
					if ok {
						pgs_service_metadata = pg_cache_obj.ServiceMetadata
					} else {
						utils.AviLog.Warning.Printf("PG %s cache incorrect type",
							pg_name)
						pgs_service_metadata = utils.ServiceMetadataObj{}
					}
				}
			}
		}
	} else {
		for pp, _ := range port_protocols {
			pg_name := fmt.Sprintf("%s-poolgroup-%v-%s", name_prefix, pp.Port,
				pp.Protocol)
			pg_names = append(pg_names, pg_name)
		}
		pgs_service_metadata = utils.ServiceMetadataObj{CrudHashKey: crud_hash_key}
	}
	for _, pg_name := range pg_names {
		// Check if resourceVersion is same as cksum from cache. If so, skip upd
		pg_key := utils.NamespaceName{Namespace: tenant, Name: pg_name}
		pg_cache, ok := p.avi_obj_cache.PgCache.AviCacheGet(pg_key)
		if !ok {
			utils.AviLog.Warning.Printf("Namespace %s PG %s not present in PG cache",
				tenant, pg_name)
		} else {
			pg_cache_obj, ok := pg_cache.(*utils.AviPGCache)
			if ok {
				if ep.ResourceVersion == pg_cache_obj.CloudConfigCksum {
					utils.AviLog.Info.Printf("PG namespace %s name %s has same cksum %s",
						tenant, pg_name, ep.ResourceVersion)
					continue
				} else {
					utils.AviLog.Info.Printf(`PG namespace %s name %s has diff
                            cksum %s resourceVersion %s`, tenant, pg_name,
						pg_cache_obj.CloudConfigCksum, ep.ResourceVersion)
				}
			} else {
				utils.AviLog.Warning.Printf("PG %s cache incorrect type", pg_name)
			}
		}
		s := strings.Split(pg_name, "-poolgroup-")
		s1 := strings.Split(s[1], "-")
		port := s1[0]
		protocol := s1[1]
		pg_name := fmt.Sprintf("%s-poolgroup-%v-%s", s[0], port,
			protocol)
		pool_name := fmt.Sprintf("%s-pool-%v-%s", s[0], port,
			protocol)
		pg_meta := utils.K8sAviPoolGroupMeta{Name: pg_name,
			Tenant:           tenant,
			ServiceMetadata:  pgs_service_metadata,
			CloudConfigCksum: ep.ResourceVersion}
		pool_ref := fmt.Sprintf("/api/pool?name=%s", pool_name)
		// TODO (sudswas): Add priority label, Ratio
		pg_meta.Members = append(pg_meta.Members, &avimodels.PoolGroupMember{PoolRef: &pool_ref})
		rest_op := aviobjects.AviPoolGroupBuild(&pg_meta)
		rest_ops = append(rest_ops, rest_op)
	}
	return rest_ops
}

/*
 * key is of the form Service/crud_hash_key/Namespace/Name
 */
func (p *K8sEp) K8sObjDelete(shard uint32, key string) ([]*utils.RestOp, error) {
	return nil, nil
}
