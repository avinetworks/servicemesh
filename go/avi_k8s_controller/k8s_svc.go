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
                AviLog.Info.Printf("Svc namespace %s name %s has same resourceversion %s",
                        svc.Namespace, svc.Name, svc.ResourceVersion)
                return nil, nil
            } else {
                AviLog.Info.Printf("Svc namespace %s name %s resourceversion %s different from cksum %s",
                        svc.Namespace, svc.Name, svc.ResourceVersion, 
                        vs_cache_obj.CloudConfigCksum)
            }
        } else {
            AviLog.Info.Printf("Svc namespace %s name %s not found in cache",
                               svc.Namespace, svc.Name)
        }
    }

    crud_hash_key := svc.Namespace + ":" + svc.Name
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

    is_http := true
    is_tcp := false
    for _, svc_port := range svc.Spec.Ports {
        // Listener is based on Port
        pp := AviPortProtocol{Port: svc_port.Port, 
                Protocol: strings.ToLower(string(svc_port.Protocol))}
        avi_vs_meta.PortProto = append(avi_vs_meta.PortProto, pp)

        if pp.Protocol == "tcp" {
            is_tcp = true
        }

        // Switching policy is based on TargetPort
        psp := AviPortStrProtocol{Port: svc_port.TargetPort.String(),
                          Protocol: string(svc_port.Protocol)}
        avi_vs_meta.PortStrProto = append(avi_vs_meta.PortStrProto, psp)

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

    if is_tcp {
        avi_vs_meta.NetworkProfile = "System-TCP-Proxy"
    } else {
        avi_vs_meta.NetworkProfile = "System-UDP-Per-Pkt"
    }

    rop := AviVsBuild(&avi_vs_meta)
    rest_ops = append(rest_ops, rop)

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
        // Add to local obj cache
        for _, rest_op := range(rest_ops) {
            if rest_op.Err == nil {
                if rest_op.Model == "Pool" {
                    AviPoolCacheAdd(s.avi_obj_cache.PoolCache, rest_op)
                } else if rest_op.Model == "VirtualService" {
                    AviVsCacheAdd(s.avi_obj_cache.VsCache, rest_op)
                }
            }
        }
    }

    return nil, err
}

func (s *K8sSvc) K8sObjDelete(shard uint32, svc *corev1.Service) ([]*RestOp, error) {
    var obj interface{}

    aviClient := s.avi_rest_client_pool.AviClient[shard]
    SetTenant := session.SetTenant(svc.Namespace)
    err := SetTenant(aviClient.AviSession)
    err = aviClient.AviSession.GetObjectByName("virtualservice",
                                        svc.Name, &obj)
    if err != nil {
        AviLog.Warning.Printf("Unable to retrieve VS tenant %s name %s", svc.Namespace,
                   svc.Name)
        return nil, nil
    } else {
        AviLog.Info.Printf("Tenant %s name %s VS %v", svc.Namespace, svc.Name, obj)
    }


    payload := AviRestObjMacro{ModelName: "VirtualService", Data: obj}
    path := fmt.Sprintf("/api/macro/?created_by=%s", OSHIFT_K8S_CLOUD_CONNECTOR)

    _, rerror := aviClient.AviSession.DeleteRaw(path, payload)
    if rerror != nil {
        AviLog.Warning.Printf("VS tenant %s name %s delete returned %v", svc.Namespace, 
                   svc.Name, rerror)
    } else {
        AviLog.Info.Printf("VS tenant %s name %s delete success", svc.Namespace,
                   svc.Name)
    }

    return nil, nil
}
