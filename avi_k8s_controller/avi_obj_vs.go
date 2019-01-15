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

func AviVsBuild(vs_meta *K8sAviVsMeta) *RestOp {
    auto_alloc := true
    vip := avimodels.Vip{AutoAllocateIP: &auto_alloc}

    // E/W placement subnet is don't care, just needs to be a valid subnet
    mask := int32(24)
    addr := "172.18.0.0"
    atype := "V4"
    sip := avimodels.IPAddr{Type: &atype, Addr: &addr}
    ew_subnet := avimodels.IPAddrPrefix{IPAddr: &sip, Mask: &mask}
    var east_west bool
    if vs_meta.EastWest == true {
        vip.Subnet = &ew_subnet
        east_west = true
    } else {
        east_west = false
    }

    network_prof := "/api/networkprofile/?name=" + vs_meta.NetworkProfile
    app_prof := "/api/applicationprofile/?name=" + vs_meta.ApplicationProfile
    // TODO use PoolGroup and use policies if there are > 1 pool, etc.
    name := vs_meta.Name
    cksum := vs_meta.CloudConfigCksum
    cr := OSHIFT_K8S_CLOUD_CONNECTOR

    vs := avimodels.VirtualService{Name: &name,
          NetworkProfileRef: &network_prof,
          ApplicationProfileRef: &app_prof,
          CloudConfigCksum: &cksum,
          CreatedBy: &cr,
          EastWestPlacement: &east_west}

    if vs_meta.DefaultPool != "" {
        pool_ref := "/api/pool/?name=" + vs_meta.DefaultPool
        vs.PoolRef = &pool_ref
    }

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

    AviLog.Info.Print(spew.Sprintf("VS Restop %v K8sAviVsMeta %v\n", rest_op,
                                   *vs_meta))
    return &rest_op
}

func AviVsCacheAdd(vs_cache *AviCache, rest_op *RestOp) error {
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
        AviLog.Warning.Printf("resp %v has invalid service_metadata value", resp)
        svc_mdata_obj = ServiceMetadataObj{}
    }

    vs_cache_obj := AviVsCache{Name: name, Tenant: rest_op.Tenant,
                    Uuid: uuid, CloudConfigCksum: cksum,
                    ServiceMetadata: svc_mdata_obj}

    k := NamespaceName{Namespace: rest_op.Tenant, Name: name}
    vs_cache.AviCacheAdd(k, &vs_cache_obj)

    AviLog.Info.Print(spew.Sprintf("VS cache key %v val %v\n", k, vs_cache_obj))

    return nil
}

