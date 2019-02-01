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
        "strings"
        "fmt"
        "k8s.io/client-go/tools/cache"
        corev1 "k8s.io/api/core/v1"
        "github.com/avinetworks/sdk/go/session"
        avimodels "github.com/avinetworks/sdk/go/models"
        )

type K8sSvc struct {
    avi_obj_cache *AviObjCache
    avi_rest_client_pool *AviRestClientPool
    informers *Informers
    k8s_ep *K8sEp
}

func NewK8sSvc(avi_obj_cache *AviObjCache, avi_rest_client_pool *AviRestClientPool,
               inf *Informers, k8s_ep *K8sEp) *K8sSvc {
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

func (s *K8sSvc) K8sObjCrUpd(shard uint32, svc *corev1.Service) ([]*RestOp, error) {
    ep, err := s.informers.EpInformer.Lister().Endpoints(svc.Namespace).Get(svc.Name)
    if err != nil {
        AviLog.Warning.Printf("Ep for Svc Namespace %s Name %s not present",
                   svc.Namespace, svc.Name)
        return nil, fmt.Errorf("Svc ep Namespace %s Name %s not found", 
                                svc.Namespace, svc.Name)
    }

    vs_cache_key := NamespaceName{Namespace: svc.Namespace, Name: svc.Name}
    vs_cache, found := s.avi_obj_cache.VsCache.AviCacheGet(vs_cache_key)
    if found {
        vs_cache_obj, ok := vs_cache.(*AviVsCache)
        if ok {
            if vs_cache_obj.CloudConfigCksum == svc.ResourceVersion {
                AviLog.Info.Printf(`Svc namespace %s name %s has same 
                    resourceversion %s`, svc.Namespace, svc.Name, 
                    svc.ResourceVersion)
                return nil, nil
            } else {
                AviLog.Info.Printf(`Svc namespace %s name %s resourceversion 
                        %s different from cksum %s`, svc.Namespace, svc.Name, 
                        svc.ResourceVersion, vs_cache_obj.CloudConfigCksum)
            }
        } else {
            AviLog.Info.Printf("Svc namespace %s name %s not found in cache",
                               svc.Namespace, svc.Name)
        }
    }

    crud_hash_key := CrudHashKey("Service", svc)
    svc_mdata := ServiceMetadataObj{CrudHashKey: crud_hash_key}

    var rest_ops []*RestOp

    // Build PG/Pool with Endpoints
    pool_rest_ops, err := s.k8s_ep.K8sObjCrUpd(shard, ep, svc.Name, crud_hash_key)
    if pool_rest_ops != nil {
        rest_ops = append(rest_ops, pool_rest_ops...)
    }

    avi_vs_meta := K8sAviVsMeta{Name: svc.Name, Tenant: svc.Namespace,
        CloudConfigCksum: svc.ResourceVersion, ServiceMetadata: svc_mdata,
        EastWest: true}

    if len(svc.Spec.Ports) > 1 {
        avi_vs_meta.PoolMap = make(map[AviPortProtocol]string)
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
        pp := AviPortProtocol{Port: svc_port.Port, Protocol: prot}
        avi_vs_meta.PortProto = append(avi_vs_meta.PortProto, pp)

        if prot != "tcp" {
            is_http = false
        } else {
            is_udp = false
        }

        if len(svc.Spec.Ports) > 1 {
            pool_name := fmt.Sprintf("%s-pool-%v-%s", svc.Name,
                svc_port.TargetPort.String(), prot)
            avi_vs_meta.PoolMap[pp] = pool_name
        }

        if IsSvcHttp(svc_port.Name, svc_port.Port) == false {
            is_http = false
        }
    }

    if len(svc.Spec.Ports) == 1 {
        for _, pool_rest_op := range pool_rest_ops {
            if pool_rest_op.Model == "Pool" {
                macro, ok := pool_rest_op.Obj.(AviRestObjMacro)
                if !ok {
                    AviLog.Warning.Printf("pool_rest_op %v has unknown Obj type", 
                                  pool_rest_op)
                    break
                }
                pool, ok := macro.Data.(avimodels.Pool)
                if !ok {
                    AviLog.Warning.Printf("pool_rest_op %v has unknown macro type", 
                                  pool_rest_op)
                } else {
                    avi_vs_meta.DefaultPool = *pool.Name
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

    rops := AviVsBuild(&avi_vs_meta)
    rest_ops = append(rest_ops, rops...)

    aviClient := s.avi_rest_client_pool.AviClient[shard]
    err = s.avi_rest_client_pool.AviRestOperate(aviClient, rest_ops)
    if err != nil {
        AviLog.Warning.Printf("Error %v with rest_ops", err)
        // Iterate over rest_ops in reverse and delete created objs
        for i := len(rest_ops)-1; i >= 0; i-- {
            if rest_ops[i].Err == nil {
                resp_arr, ok := rest_ops[i].Response.([]interface{})
                if !ok {
                    AviLog.Warning.Printf("Invalid resp type for rest_op %v", rest_ops[i])
                    continue
                }
                resp, ok := resp_arr[0].(map[string]interface{})
                if ok {
                    uuid, ok := resp["uuid"].(string)
                    if !ok {
                        AviLog.Warning.Printf("Invalid resp type for uuid %v", 
                                              resp)
                        continue
                    }
                    url := AviModelToUrl(rest_ops[i].Model) + "/" + uuid
                    err := aviClient.AviSession.Delete(url)
                    if err != nil {
                        AviLog.Warning.Printf("Error %v deleting url %v", err, url)
                    } else {
                        AviLog.Info.Printf("Success deleting url %v", url)
                    }
                } else {
                    AviLog.Warning.Printf("Invalid resp for rest_op %v", rest_ops[i])
                }
            }
        }
    } else {
        // Add to local obj caches
        for _, rest_op := range(rest_ops) {
            if rest_op.Err == nil {
                if rest_op.Model == "Pool" {
                    AviPoolCacheAdd(s.avi_obj_cache.PoolCache, rest_op)
                    s.k8s_ep.K8sEpSvcToPoolCacheAdd(vs_cache_key, "service",
                                                    rest_op)
                } else if rest_op.Model == "VirtualService" {
                    AviVsCacheAdd(s.avi_obj_cache.VsCache, rest_op)
                }
            }
        }
    }

    return nil, err
}

/*
 * key is of the form Service/crud_hash_key/Namespace/Name
 */

func (s *K8sSvc) K8sObjDelete(shard uint32, key string) ([]*RestOp, error) {
    var obj interface{}
    var err error
    var ns, name string

    key_elems := strings.SplitN(key, "/", 3)

    ns, name, err = cache.SplitMetaNamespaceKey(key_elems[2])
    if err != nil {
        AviLog.Warning.Printf("Unable to extract ns name from key %v", key)
        return nil, err
    }

    aviClient := s.avi_rest_client_pool.AviClient[shard]
    SetTenant := session.SetTenant(ns)
    err = SetTenant(aviClient.AviSession)
    err = aviClient.AviSession.GetObjectByName("virtualservice", name, &obj)
    if err != nil {
        AviLog.Warning.Printf("Unable to retrieve VS tenant %s name %s", ns, name)
        return nil, err
    } else {
        AviLog.Info.Printf("Tenant %s name %s VS %v", ns, name, obj)
    }

    payload := AviRestObjMacro{ModelName: "VirtualService", Data: obj}
    path := fmt.Sprintf("/api/macro/?created_by=%s", OSHIFT_K8S_CLOUD_CONNECTOR)

    rerror := aviClient.AviSession.Delete(path, payload)
    if rerror != nil {
        AviLog.Warning.Printf("VS tenant %s name %s delete returned %v",
                              ns, name, rerror)
    } else {
        AviLog.Info.Printf("VS tenant %s name %s delete success", ns, name)
    }

    // Delete all service related objs in Avi cache

    cache_key := NamespaceName{Namespace: ns, Name: name}

    /*
     * ppool_map is of the form:
     * [service/name-pool-http-tcp] = true
     * [ingress/name-pool-http-tcp] = true
     */
    ppool_map, ok := s.k8s_ep.K8sEpSvcToPoolCacheGet(cache_key)
    if !ok {
        AviLog.Info.Printf("Key %v not found in SvcToPoolCache", cache_key)
    } else {
        for ppool_name_intf := range ppool_map {
            ppool_name, ok := ppool_name_intf.(string)
            if !ok {
                AviLog.Warning.Printf("ppool_name_intf %T is not type string",
                                  ppool_name_intf)
                continue
            }
            elems := strings.Split(ppool_name, "/")
            if elems[0] == "service" {
                // PoolCache key is of the form {ns, pool_name}
                pcache_key := NamespaceName{Namespace: ns, Name: elems[1]}
                AviLog.Info.Printf("Delete pool %v in PoolCache", pcache_key)
                AviPoolCacheDel(s.avi_obj_cache.PoolCache, pcache_key)
            }
        }
    }

    AviLog.Info.Printf("Delete key %v service in SvcToPoolCache", cache_key)
    s.k8s_ep.K8sEpSvcToPoolCacheDel(cache_key, "service")

    AviLog.Info.Printf("Delete VS %v in VsCache", cache_key)
    AviVsCacheDel(s.avi_obj_cache.VsCache, cache_key)

    return nil, nil
}
