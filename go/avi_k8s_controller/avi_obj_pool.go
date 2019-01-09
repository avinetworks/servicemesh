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
        "github.com/golang/glog"
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
    return &rest_op
}

func AviPoolCacheAdd(pool_cache *AviCache, rest_op *RestOp) error {
    if (rest_op.Err != nil) || (rest_op.Response == nil) {
        glog.Warningf("rest_op has err or no reponse")
        return errors.New("Errored rest_op")
    }

    resp, ok := rest_op.Response.(map[string]string)
    if !ok {
        glog.Warningf("Response has unknown type %t", resp)
        return errors.New("Malformed response")
    }

    name, ok := resp["Name"]
    if !ok {
        glog.Warningf("Name not present in response %v", resp)
        return errors.New("Name not present in response")
    }

    uuid, ok := resp["UUID"]
    if !ok {
        glog.Warningf("Uuid not present in response %v", resp)
        return errors.New("Uuid not present in response")
    }

    lb := resp["LbAlgorithm"]
    cksum := resp["CloudConfigCksum"]

    var svc_mdata interface{}
    var svc_mdata_obj ServiceMetadataObj

    if err := json.Unmarshal([]byte(resp["ServiceMetadata"]), &svc_mdata); err != nil {
        svc_mdata_obj, ok = svc_mdata.(ServiceMetadataObj)
        if !ok {
            glog.Warningf("resp %v has invalid ServiceMetadata type", resp)
            svc_mdata_obj = ServiceMetadataObj{}
        }
    } else {
        glog.Warningf("resp %v has invalid ServiceMetadata value", resp)
        svc_mdata_obj = ServiceMetadataObj{}
    }

    pool_cache_obj := AviPoolCache{Name: name, Tenant: rest_op.Tenant,
                    Uuid: uuid, LbAlgorithm: lb,
                    CloudConfigCksum: cksum, ServiceMetadata: svc_mdata_obj}

    k := NamespaceName{Namespace: rest_op.Tenant, Name: name}
    pool_cache.AviCacheAdd(k, pool_cache_obj)

    return nil
}
