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

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/avinetworks/avi_k8s_controller/pkg/utils"
	avimodels "github.com/avinetworks/sdk/go/models"
	corev1 "k8s.io/api/core/v1"
)

/* AviCache for storing * Service to E/W Pools and Route/Ingress Pools.
 * Of the form:
 * map[{namespace: string, name: string}]map[pool_name_prefix:string]bool
 */

type K8sEp struct {
	avi_obj_cache        *utils.AviObjCache
	avi_rest_client_pool *AviRestClientPool
	svc_to_pool_cache    *utils.AviMultiCache
	svc_to_pg_cache      *utils.AviMultiCache
	informers            *Informers
}

func NewK8sEp(avi_obj_cache *utils.AviObjCache, avi_rest_client_pool *AviRestClientPool,
	inf *Informers) *K8sEp {
	p := K8sEp{}
	p.svc_to_pool_cache = utils.NewAviMultiCache()
	p.svc_to_pg_cache = utils.NewAviMultiCache()
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
	name_prefix string, crud_hash_key string) ([]*RestOp, error) {
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

	port_protocols := make(map[AviPortStrProtocol]bool)
	svc, err := p.informers.ServiceInformer.Lister().Services(ep.Namespace).Get(ep.Name)
	if err != nil {
		utils.AviLog.Warning.Printf(`Service for Endpoint Namespace %v Name %v 
            doesn't exist`, ep.Namespace, ep.Name)
		return nil, &SkipSyncError{"Skip sync"}
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
				pp := AviPortStrProtocol{Port: tgt_port, Protocol: prot}
				port_protocols[pp] = true
			}
		}
	}

	var pool_names []string
	var service_metadata ServiceMetadataObj

	process_pool := true
	k := NamespaceName{Namespace: ep.Namespace, Name: ep.Name}
	/*
	 * If  name_prefix is nil, this is a Ep CU event from event handler; See
	 * if pools/pgs exist already. If so, let's perform U
	 */
	if name_prefix == "" {
		var pools_cache interface{}
		var pools map[interface{}]bool
		var ok bool
		pools_cache, process_pool = p.svc_to_pool_cache.AviMultiCacheGetKey(k)
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
				pool_key := NamespaceName{Namespace: tenant, Name: pool_name}
				pool_cache, ok1 := p.avi_obj_cache.PoolCache.AviCacheGet(pool_key)
				if !ok1 {
					utils.AviLog.Warning.Printf(`Pool %s not present in Obj cache but
                                           present in Pool cache`, pool_name)
				} else {
					pool_cache_obj, ok := pool_cache.(*AviPoolCache)
					if ok {
						service_metadata = pool_cache_obj.ServiceMetadata
					} else {
						utils.AviLog.Warning.Printf("Pool %s cache incorrect type",
							pool_name)
						service_metadata = ServiceMetadataObj{}
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
		service_metadata = ServiceMetadataObj{CrudHashKey: crud_hash_key}
	}

	if !process_pool {
		utils.AviLog.Info.Printf("Endpoint %v is not present in Pool/Pg cache.", k)
		return nil, nil
	}

	var rest_ops []*RestOp

	for _, pool_name := range pool_names {
		// Check if resourceVersion is same as cksum from cache. If so, skip upd
		pool_key := NamespaceName{Namespace: tenant, Name: pool_name}
		pool_cache, ok := p.avi_obj_cache.PoolCache.AviCacheGet(pool_key)
		if !ok {
			utils.AviLog.Warning.Printf("Namespace %s Pool %s not present in Pool cache",
				tenant, pool_name)
		} else {
			pool_cache_obj, ok := pool_cache.(*AviPoolCache)
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
		pool_meta := K8sAviPoolMeta{Name: pool_name,
			Tenant:           tenant,
			ServiceMetadata:  service_metadata,
			CloudConfigCksum: ep.ResourceVersion}
		s := strings.Split(pool_name, "-pool-")
		s1 := strings.Split(s[1], "-")
		port := s1[0]
		port_num, _ := strconv.Atoi(port)
		protocol := s1[1]
		pool_meta.Protocol = protocol
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
					server := AviPoolMetaServer{Ip: a}
					if addr.NodeName != nil {
						server.ServerNode = *addr.NodeName
					}
					pool_meta.Servers = append(pool_meta.Servers, server)
				}
			}
		}

		rest_op := AviPoolBuild(&pool_meta)
		rest_ops = append(rest_ops, rest_op)
	}

	if name_prefix == "" {
		p.avi_rest_client_pool.AviRestOperate(p.avi_rest_client_pool.AviClient[shard], rest_ops)
		for _, rest_op := range rest_ops {
			if rest_op.Err == nil {
				AviPoolCacheAdd(p.avi_obj_cache.PoolCache, rest_op)
			}
		}
		return nil, nil
	} else {
		return rest_ops, nil
	}
}

/*
 * key is of the form Service/crud_hash_key/Namespace/Name
 */

func (p *K8sEp) K8sObjDelete(shard uint32, key string) ([]*RestOp, error) {
	return nil, nil
}

func (p *K8sEp) K8sEpSvcToPoolCacheAdd(key NamespaceName,
	prefix string, rest_op *RestOp) error {
	err := AviSvcToPoolCacheAdd(p.svc_to_pool_cache, rest_op, prefix, key)

	return err
}

func (p *K8sEp) K8sEpSvcToPoolCacheGet(key NamespaceName) (map[interface{}]bool, bool) {
	return p.svc_to_pool_cache.AviMultiCacheGetKey(key)
}

func (p *K8sEp) K8sEpSvcToPoolCacheDel(key NamespaceName, prefix string) error {
	err := AviSvcToPoolCacheDel(p.svc_to_pool_cache, prefix, key)

	return err
}
