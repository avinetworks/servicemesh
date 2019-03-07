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
	"strings"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/sdk/go/session"
	"github.com/avinetworks/servicemesh/aviobjects"
	"github.com/avinetworks/servicemesh/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type K8sSvc struct {
	avi_obj_cache        *utils.AviObjCache
	avi_rest_client_pool *utils.AviRestClientPool
	informers            *utils.Informers
	k8s_ep               *K8sEp
}

func NewK8sSvc(avi_obj_cache *utils.AviObjCache, avi_rest_client_pool *utils.AviRestClientPool,
	inf *utils.Informers, k8s_ep *K8sEp) *K8sSvc {
	s := K8sSvc{}
	s.avi_obj_cache = avi_obj_cache
	s.avi_rest_client_pool = avi_rest_client_pool
	s.informers = inf
	s.k8s_ep = k8s_ep
	return &s
}

/*
 * This function is called on a CU of Service event handler
 */

func (s *K8sSvc) K8sObjCrUpd(shard uint32, svc *corev1.Service) ([]*utils.RestOp, error) {
	ep, err := s.informers.EpInformer.Lister().Endpoints(svc.Namespace).Get(svc.Name)
	if err != nil {
		utils.AviLog.Warning.Printf("Ep for Svc Namespace %s Name %s not present",
			svc.Namespace, svc.Name)
		return nil, fmt.Errorf("Svc ep Namespace %s Name %s not found",
			svc.Namespace, svc.Name)
	}

	vs_cache_key := utils.NamespaceName{Namespace: svc.Namespace, Name: svc.Name}
	vs_cache, found := s.avi_obj_cache.VsCache.AviCacheGet(vs_cache_key)
	if found {
		vs_cache_obj, ok := vs_cache.(*utils.AviVsCache)
		if ok {
			if vs_cache_obj.CloudConfigCksum == svc.ResourceVersion {
				utils.AviLog.Info.Printf(`Svc namespace %s name %s has same 
                    resourceversion %s`, svc.Namespace, svc.Name,
					svc.ResourceVersion)
				return nil, nil
			} else {
				utils.AviLog.Info.Printf(`Svc namespace %s name %s resourceversion 
                        %s different from cksum %s`, svc.Namespace, svc.Name,
					svc.ResourceVersion, vs_cache_obj.CloudConfigCksum)
			}
		} else {
			utils.AviLog.Info.Printf("Svc namespace %s name %s not found in cache",
				svc.Namespace, svc.Name)
		}
	}

	crud_hash_key := utils.CrudHashKey("Service", svc)
	svc_mdata := utils.ServiceMetadataObj{CrudHashKey: crud_hash_key}

	var rest_ops []*utils.RestOp

	// Build Pool with Endpoints
	pool_rest_ops, err := s.k8s_ep.K8sObjCrUpd(shard, ep, svc.Name, crud_hash_key)
	if pool_rest_ops != nil {
		rest_ops = append(rest_ops, pool_rest_ops...)
	}
	_, port_protocols := s.k8s_ep.GetValidPorts(ep)
	// Populate poolgroups
	pg_rest_ops := s.CreatePoolGroups(port_protocols, svc, crud_hash_key)
	if pg_rest_ops != nil {
		rest_ops = append(rest_ops, pg_rest_ops...)
	}
	avi_vs_meta := utils.K8sAviVsMeta{Name: svc.Name, Tenant: svc.Namespace,
		CloudConfigCksum: svc.ResourceVersion, ServiceMetadata: svc_mdata,
		EastWest: true}

	if len(svc.Spec.Ports) > 1 {
		avi_vs_meta.PoolGroupMap = make(map[utils.AviPortProtocol]string)
	}

	is_http := true
	is_udp := true
	for _, svc_port := range svc.Spec.Ports {
		var prot string
		if string(svc_port.Protocol) == "" {
			prot = "tcp" // Default
		} else {
			prot = strings.ToLower(string(svc_port.Protocol))
		}

		// Listener is based on Port
		pp := utils.AviPortProtocol{Port: svc_port.Port, Protocol: prot}
		avi_vs_meta.PortProto = append(avi_vs_meta.PortProto, pp)

		if prot != "tcp" {
			is_http = false
		} else {
			is_udp = false
		}

		if len(svc.Spec.Ports) > 1 {
			pg_name := fmt.Sprintf("%s-poolgroup-%v-%s", svc.Name,
				svc_port.TargetPort.String(), prot)
			avi_vs_meta.PoolGroupMap[pp] = pg_name
		}

		if utils.IsSvcHttp(svc_port.Name, svc_port.Port) == false {
			is_http = false
		}
	}

	if len(svc.Spec.Ports) == 1 {
		for _, pg_rest_ops := range pg_rest_ops {
			// TODO (sudswas): What if a poolgroup was created before and the service was created later?
			// We will find it in the cache and hence rest_op won't have it.
			// So we won't patch it.
			if pg_rest_ops.Model == "PoolGroup" {
				macro, ok := pg_rest_ops.Obj.(utils.AviRestObjMacro)
				if !ok {
					utils.AviLog.Warning.Printf("pg_rest_ops %v has unknown Obj type",
						pg_rest_ops)
					break
				}
				poolgroup, ok := macro.Data.(avimodels.PoolGroup)
				if !ok {
					utils.AviLog.Warning.Printf("pg_rest_ops %v has unknown macro type",
						pg_rest_ops)
				} else {
					avi_vs_meta.DefaultPoolGroup = *poolgroup.Name
				}
				break
			}
		}
	}

	if is_http {
		avi_vs_meta.ApplicationProfile = "System-HTTP"
	} else {
		avi_vs_meta.ApplicationProfile = "System-L4-Application"
	}

	if is_udp {
		avi_vs_meta.NetworkProfile = "System-UDP-Fast-Path"
	} else {
		avi_vs_meta.NetworkProfile = "System-TCP-Proxy"
	}

	rops := aviobjects.AviVsBuild(&avi_vs_meta)
	rest_ops = append(rest_ops, rops...)

	aviClient := s.avi_rest_client_pool.AviClient[shard]
	err = s.avi_rest_client_pool.AviRestOperate(aviClient, rest_ops)
	if err != nil {
		utils.AviLog.Warning.Printf("Error %v with rest_ops", err)
		// Iterate over rest_ops in reverse and delete created objs
		for i := len(rest_ops) - 1; i >= 0; i-- {
			if rest_ops[i].Err == nil {
				resp_arr, ok := rest_ops[i].Response.([]interface{})
				if !ok {
					utils.AviLog.Warning.Printf("Invalid resp type for rest_op %v", rest_ops[i])
					continue
				}
				resp, ok := resp_arr[0].(map[string]interface{})
				if ok {
					uuid, ok := resp["uuid"].(string)
					if !ok {
						utils.AviLog.Warning.Printf("Invalid resp type for uuid %v",
							resp)
						continue
					}
					url := utils.AviModelToUrl(rest_ops[i].Model) + "/" + uuid
					err := aviClient.AviSession.Delete(url)
					if err != nil {
						utils.AviLog.Warning.Printf("Error %v deleting url %v", err, url)
					} else {
						utils.AviLog.Info.Printf("Success deleting url %v", url)
					}
				} else {
					utils.AviLog.Warning.Printf("Invalid resp for rest_op %v", rest_ops[i])
				}
			}
		}
	} else {
		// Add to local obj caches
		for _, rest_op := range rest_ops {
			if rest_op.Err == nil {
				if rest_op.Model == "Pool" {
					aviobjects.AviPoolCacheAdd(s.avi_obj_cache.PoolCache, rest_op)
					aviobjects.AviSvcToPoolCacheAdd(s.avi_obj_cache.SvcToPoolCache, rest_op,
						"service", vs_cache_key)
				} else if rest_op.Model == "VirtualService" {
					aviobjects.AviVsCacheAdd(s.avi_obj_cache.VsCache, rest_op)
				} else if rest_op.Model == "PoolGroup" {
					aviobjects.AviPGCacheAdd(s.avi_obj_cache.PgCache, rest_op)
				}
			}
		}
	}

	return nil, err
}

/*
 * key is of the form Service/crud_hash_key/Namespace/Name
 */

func (s *K8sSvc) K8sObjDelete(shard uint32, key string) ([]*utils.RestOp, error) {
	var obj interface{}
	var err error
	var ns, name string

	key_elems := strings.SplitN(key, "/", 3)

	ns, name, err = cache.SplitMetaNamespaceKey(key_elems[2])
	if err != nil {
		utils.AviLog.Warning.Printf("Unable to extract ns name from key %v", key)
		return nil, err
	}

	aviClient := s.avi_rest_client_pool.AviClient[shard]
	SetTenant := session.SetTenant(ns)
	err = SetTenant(aviClient.AviSession)
	err = aviClient.AviSession.GetObjectByName("virtualservice", name, &obj)
	if err != nil {
		utils.AviLog.Warning.Printf("Unable to retrieve VS tenant %s name %s", ns, name)
		return nil, err
	} else {
		utils.AviLog.Info.Printf("Tenant %s name %s VS %v", ns, name, obj)
	}

	payload := utils.AviRestObjMacro{ModelName: "VirtualService", Data: obj}
	path := fmt.Sprintf("/api/macro/?created_by=%s", utils.OSHIFT_K8S_CLOUD_CONNECTOR)

	rerror := aviClient.AviSession.Delete(path, payload)
	if rerror != nil {
		utils.AviLog.Warning.Printf("VS tenant %s name %s delete returned %v",
			ns, name, rerror)
	} else {
		utils.AviLog.Info.Printf("VS tenant %s name %s delete success", ns, name)
	}

	// Delete all service related objs in Avi cache

	cache_key := utils.NamespaceName{Namespace: ns, Name: name}

	/*
	 * ppool_map is of the form:
	 * [service/name-pool-http-tcp] = true
	 * [ingress/name-pool-http-tcp] = true
	 */
	ppool_map, ok := s.avi_obj_cache.SvcToPoolCache.AviMultiCacheGetKey(cache_key)
	if !ok {
		utils.AviLog.Info.Printf("Key %v not found in SvcToPoolCache", cache_key)
	} else {
		for ppool_name_intf := range ppool_map {
			ppool_name, ok := ppool_name_intf.(string)
			if !ok {
				utils.AviLog.Warning.Printf("ppool_name_intf %T is not type string",
					ppool_name_intf)
				continue
			}
			elems := strings.Split(ppool_name, "/")
			if elems[0] == "service" {
				// PoolCache key is of the form {ns, pool_name}
				pcache_key := utils.NamespaceName{Namespace: ns, Name: elems[1]}
				utils.AviLog.Info.Printf("Delete pool %v in PoolCache", pcache_key)
				aviobjects.AviPoolCacheDel(s.avi_obj_cache.PoolCache, pcache_key)
			}
		}
	}
	utils.AviLog.Info.Printf("Delete key %v PG in PG cache", cache_key)
	aviobjects.AviPGCacheDel(s.avi_obj_cache.PgCache, cache_key)
	utils.AviLog.Info.Printf("Delete key %v service in SvcToPoolCache", cache_key)
	aviobjects.AviSvcToPoolCacheDel(s.avi_obj_cache.SvcToPoolCache, "service", cache_key)

	utils.AviLog.Info.Printf("Delete VS %v in VsCache", cache_key)
	aviobjects.AviVsCacheDel(s.avi_obj_cache.VsCache, cache_key)

	return nil, nil
}

func (p *K8sSvc) CreatePoolGroups(port_protocols map[utils.AviPortStrProtocol]bool, svc *corev1.Service, crud_hash_key string) []*utils.RestOp {
	var pg_names []string
	var rest_ops []*utils.RestOp
	var pg_service_metadata utils.ServiceMetadataObj
	for pp, _ := range port_protocols {
		pg_name := fmt.Sprintf("%s-poolgroup-%v-%s", svc.Name, pp.Port,
			pp.Protocol)
		pg_names = append(pg_names, pg_name)
	}
	pg_service_metadata = utils.ServiceMetadataObj{CrudHashKey: crud_hash_key}
	for _, pg_name := range pg_names {
		// Check if resourceVersion is same as cksum from cache. If so, skip upd
		pg_key := utils.NamespaceName{Namespace: svc.Namespace, Name: pg_name}
		pg_cache, ok := p.avi_obj_cache.PgCache.AviCacheGet(pg_key)
		if !ok {
			utils.AviLog.Info.Printf("Namespace %s PG %s not present in PG cache",
				svc.Namespace, pg_name)
		} else {
			pg_cache_obj, ok := pg_cache.(*utils.AviPGCache)
			if ok {
				if svc.ResourceVersion == pg_cache_obj.CloudConfigCksum {
					utils.AviLog.Info.Printf("PG namespace %s name %s has same cksum %s",
						svc.Namespace, pg_name, svc.ResourceVersion)
					continue
				} else {
					utils.AviLog.Info.Printf(`PG namespace %s name %s has diff
                            cksum %s resourceVersion %s`, svc.Namespace, pg_name,
						pg_cache_obj.CloudConfigCksum, svc.ResourceVersion)
				}
			} else {
				utils.AviLog.Warning.Printf("PG %s cache incorrect type", pg_name)
			}
		}
		s := strings.Split(pg_name, "-poolgroup-")
		s1 := strings.Split(s[1], "-")
		port := s1[0]
		protocol := s1[1]
		pool_name := fmt.Sprintf("%s-pool-%v-%s", s[0], port,
			protocol)
		pg_meta := utils.K8sAviPoolGroupMeta{Name: pg_name,
			Tenant:           svc.Namespace,
			ServiceMetadata:  pg_service_metadata,
			CloudConfigCksum: svc.ResourceVersion}
		pool_ref := fmt.Sprintf("/api/pool?name=%s", pool_name)
		// TODO (sudswas): Add priority label, Ratio
		pg_meta.Members = append(pg_meta.Members, &avimodels.PoolGroupMember{PoolRef: &pool_ref})
		rest_op := aviobjects.AviPoolGroupBuild(&pg_meta)
		rest_ops = append(rest_ops, rest_op)
	}
	return rest_ops
}
