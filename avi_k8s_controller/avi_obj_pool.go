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
        "errors"
        "encoding/json"
        "github.com/davecgh/go-spew/spew"
        avimodels "github.com/avinetworks/sdk/go/models"
       )

func AviPoolBuild(pool_meta *K8sAviPoolMeta) *RestOp {
    name := pool_meta.Name
    cksum := pool_meta.CloudConfigCksum
    tenant := fmt.Sprintf("/api/tenant/?name=%s", pool_meta.Tenant)
    svc_mdata_json, _ := json.Marshal(&pool_meta.ServiceMetadata)
    svc_mdata := string(svc_mdata_json)
    cr := OSHIFT_K8S_CLOUD_CONNECTOR

    pool := avimodels.Pool{Name: &name, CloudConfigCksum: &cksum,
          CreatedBy: &cr, TenantRef: &tenant, ServiceMetadata: &svc_mdata}

    // TODO other fields like cloud_ref and lb algo

    for _, server := range pool_meta.Servers {
        sip := server.Ip
        port := pool_meta.Port
        s := avimodels.Server{IP: &sip, Port: &port}
        if server.ServerNode != "" {
            sn := server.ServerNode
            s.ServerNode = &sn
        }
        pool.Servers = append(pool.Servers, &s)
    }

    var hm string
    if pool_meta.Protocol == "udp" {
        hm = fmt.Sprintf("/api/healthmonitor/?name=%s", AVI_DEFAULT_UDP_HM)
    } else {
        hm = fmt.Sprintf("/api/healthmonitor/?name=%s", AVI_DEFAULT_TCP_HM)
    }
    pool.HealthMonitorRefs = append(pool.HealthMonitorRefs, hm)

    macro := AviRestObjMacro{ModelName: "Pool", Data: pool}

    // TODO Version should be latest from configmap
    rest_op := RestOp{Path: "/api/macro", Method: RestPost, Obj: macro,
        Tenant: pool_meta.Tenant, Model: "Pool", Version: "18.1.5"}

    AviLog.Info.Print(spew.Sprintf("Pool Restop %v K8sAviPoolMeta %v\n", 
                                   rest_op, *pool_meta))
    return &rest_op
}

func AviPoolCacheAdd(pool_cache *AviCache, rest_op *RestOp) error {
    if (rest_op.Err != nil) || (rest_op.Response == nil) {
        AviLog.Warning.Printf("rest_op has err or no reponse")
        return errors.New("Errored rest_op")
    }

    resp_arr, ok := rest_op.Response.([]interface{})
    if !ok {
        AviLog.Warning.Printf("Response has unknown type %T", rest_op.Response)
        return errors.New("Malformed response")
    }

    resp, ok := resp_arr[0].(map[string]interface{})
    if !ok {
        AviLog.Warning.Printf("Response has unknown type %T", resp_arr[0])
        return errors.New("Malformed response")
    }

    name, ok := resp["name"].(string)
    if !ok {
        AviLog.Warning.Printf("Name not present in response %v", resp)
        return errors.New("Name not present in response")
    }

    uuid, ok := resp["uuid"].(string)
    if !ok {
        AviLog.Warning.Printf("Uuid not present in response %v", resp)
        return errors.New("Uuid not present in response")
    }

    lb := resp["lb_algorithm"].(string)
    cksum := resp["cloud_config_cksum"].(string)

    var svc_mdata interface{}
    var svc_mdata_map map[string]interface{}
    var svc_mdata_obj ServiceMetadataObj

    if err := json.Unmarshal([]byte(resp["service_metadata"].(string)), &svc_mdata); err == nil {
        svc_mdata_map, ok = svc_mdata.(map[string]interface{})
        if !ok {
            AviLog.Warning.Printf("resp %v svc_mdata %T has invalid service_metadata type", resp, svc_mdata)
            svc_mdata_obj = ServiceMetadataObj{}
        } else {
            SvcMdataMapToObj(&svc_mdata_map, &svc_mdata_obj)
        }
    } else {
        AviLog.Warning.Printf("resp %v has invalid service_metadata value err %v", resp, err)
        svc_mdata_obj = ServiceMetadataObj{}
    }

    pool_cache_obj := AviPoolCache{Name: name, Tenant: rest_op.Tenant,
                    Uuid: uuid, LbAlgorithm: lb,
                    CloudConfigCksum: cksum, ServiceMetadata: svc_mdata_obj}

    k := NamespaceName{Namespace: rest_op.Tenant, Name: name}
    pool_cache.AviCacheAdd(k, &pool_cache_obj)

    AviLog.Info.Print(spew.Sprintf("Pool cache k %v val %v\n", k, pool_cache_obj))

    return nil
}
