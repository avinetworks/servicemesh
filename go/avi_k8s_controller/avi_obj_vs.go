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

func AviVsBuild(vs_meta *K8sAviVsMeta) *RestOp {
    auto_alloc := true
    vip := avimodels.Vip{AutoAllocateIP: &auto_alloc}

    // E/W placement subnet is don't care, just needs to be a valid subnet
    mask := int32(24)
    addr := "172.18.0.0"
    atype := "V4"
    sip := avimodels.IPAddr{Type: &atype, Addr: &addr}
    ew_subnet := avimodels.IPAddrPrefix{IPAddr: &sip, Mask: &mask}
    if vs_meta.EastWest == true {
        vip.Subnet = &ew_subnet
    }

    network_prof := "/api/networkprofile/?name=" + vs_meta.NetworkProfile
    app_prof := "/api/applicationprofile/?name=" + vs_meta.ApplicationProfile
    // TODO use PoolGroup and use policies if there are > 1 pool, etc.
    pool_ref := "/api/pool/?name=" + vs_meta.DefaultPool

    name := vs_meta.Name
    cksum := vs_meta.CloudConfigCksum
    cr := OSHIFT_K8S_CLOUD_CONNECTOR

    vs := avimodels.VirtualService{Name: &name,
          NetworkProfileRef: &network_prof,
          ApplicationProfileRef: &app_prof,
          PoolRef: &pool_ref,
          CloudConfigCksum: &cksum,
          CreatedBy: &cr}

    vs.Vip = append(vs.Vip, &vip)

    tenant := fmt.Sprintf("/api/tenant/?name=%s", vs_meta.Tenant)
    vs.TenantRef = &tenant
    svc_meta_json, _ := json.Marshal(&vs_meta.ServiceMetadata)
    svc_meta := string(svc_meta_json)
    vs.ServiceMetadata = &svc_meta

    // TODO other fields like cloud_ref, mix of TCP & UDP protocols, etc.

    for _, pp := range vs_meta.PortProto {
        port := pp.Port
        svc := avimodels.Service{Port: &port}
        vs.Services = append(vs.Services, &svc)
    }

    macro := AviRestObjMacro{ModelName: "VirtualService", Data: vs}

    // TODO Version from configmap
    rest_op := RestOp{Path: "/api/macro", Method: RestPost, Obj: macro,
        Tenant: vs_meta.Tenant, Model: "VirtualService", Version: "18.1.5"}
    return &rest_op
}

func AviVsCacheAdd(vs_cache *AviCache, rest_op *RestOp) error {
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

    vs_cache_obj := AviVsCache{Name: name, Tenant: rest_op.Tenant,
                    Uuid: uuid, CloudConfigCksum: cksum,
                    ServiceMetadata: svc_mdata_obj}

    k := NamespaceName{Namespace: rest_op.Tenant, Name: name}
    vs_cache.AviCacheAdd(k, vs_cache_obj)

    return nil
}

