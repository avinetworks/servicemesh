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

package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/pkg/istio/nodes"
	"github.com/avinetworks/servicemesh/pkg/utils"
	"github.com/davecgh/go-spew/spew"
)

func AviVsSniBuild(vs_meta *nodes.AviVsTLSNode, httppolicynode []*nodes.AviHttpPolicySetNode, rest_method utils.RestMethod, cache_obj *utils.AviVsCache) []*utils.RestOp {
	name := vs_meta.Name
	cksum := vs_meta.CloudConfigCksum
	checksumstr := fmt.Sprint(cksum)
	cr := utils.OSHIFT_K8S_CLOUD_CONNECTOR

	east_west := false

	sniChild := &avimodels.VirtualService{Name: &name, CloudConfigCksum: &checksumstr,
		CreatedBy:         &cr,
		EastWestPlacement: &east_west}

	//This VS has a TLSKeyCert associated, we need to mark 'type': 'VS_TYPE_VH_PARENT'
	vh_type := "VS_TYPE_VH_CHILD"
	sniChild.Type = &vh_type
	vhParentUuid := "/api/virtualservice/?name=" + vs_meta.VHParentName
	sniChild.VhParentVsUUID = &vhParentUuid
	sniChild.VhDomainName = vs_meta.VHDomainNames
	ignPool := true
	sniChild.IgnPoolNetReach = &ignPool
	for _, sslkeycert := range vs_meta.TLSKeyCert {
		certName := "/api/sslkeyandcertificate/?name=" + sslkeycert.Name
		sniChild.SslKeyAndCertificateRefs = append(sniChild.SslKeyAndCertificateRefs, certName)
	}
	svc_meta_json, _ := json.Marshal(vs_meta.AviVsNode.ServiceMetadata)
	svc_meta := string(svc_meta_json)
	sniChild.ServiceMetadata = &svc_meta
	var rest_ops []*utils.RestOp

	var i int32
	i = 0
	var httpPolicyCollection []*avimodels.HTTPPolicies
	for _, http := range httppolicynode {
		// Update them on the VS object
		var j int32
		j = i + 11
		i = i + 1
		httpPolicy := fmt.Sprintf("/api/httppolicyset/?name=%s", http.Name)
		httpPolicies := &avimodels.HTTPPolicies{HTTPPolicySetRef: &httpPolicy, Index: &j}
		httpPolicyCollection = append(httpPolicyCollection, httpPolicies)
	}
	sniChild.HTTPPolicies = httpPolicyCollection
	var rest_op utils.RestOp
	var path string
	if rest_method == utils.RestPut {

		path = "/api/virtualservice/" + cache_obj.Uuid
		rest_op = utils.RestOp{Path: path, Method: rest_method, Obj: sniChild,
			Tenant: vs_meta.Tenant, Model: "VirtualService", Version: utils.CtrlVersion}
		rest_ops = append(rest_ops, &rest_op)

	} else {

		macro := utils.AviRestObjMacro{ModelName: "VirtualService", Data: sniChild}
		path = "/api/macro"
		rest_op = utils.RestOp{Path: path, Method: rest_method, Obj: macro,
			Tenant: vs_meta.Tenant, Model: "VirtualService", Version: utils.CtrlVersion}
		rest_ops = append(rest_ops, &rest_op)

	}

	utils.AviLog.Info.Print(spew.Sprintf("VS Restop %v K8sAviVsMeta %v\n", utils.Stringify(rest_op),
		*vs_meta))
	return rest_ops
}

func AviVsBuild(vs_meta *nodes.AviVsNode, httppolicynode []*nodes.AviHttpPolicySetNode, rest_method utils.RestMethod, cache_obj *utils.AviVsCache) []*utils.RestOp {

	var vip avimodels.Vip
	if rest_method == utils.RestPost {
		auto_alloc := true
		vip = avimodels.Vip{AutoAllocateIP: &auto_alloc}
	} else {
		auto_alloc_put := true
		vipId := "0"
		auto_allocate_floating_ip := false
		vip = avimodels.Vip{AutoAllocateIP: &auto_alloc_put, VipID: &vipId, AutoAllocateFloatingIP: &auto_allocate_floating_ip}
	}
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
	checksumstr := fmt.Sprint(cksum)
	cr := utils.OSHIFT_K8S_CLOUD_CONNECTOR
	vs := avimodels.VirtualService{Name: &name,
		NetworkProfileRef:     &network_prof,
		ApplicationProfileRef: &app_prof,
		CloudConfigCksum:      &checksumstr,
		CreatedBy:             &cr,
		EastWestPlacement:     &east_west}

	if vs_meta.DefaultPoolGroup != "" {
		pool_ref := "/api/poolgroup/?name=" + vs_meta.DefaultPoolGroup
		vs.PoolGroupRef = &pool_ref
	}
	vs.Vip = append(vs.Vip, &vip)
	tenant := fmt.Sprintf("/api/tenant/?name=%s", vs_meta.Tenant)
	vs.TenantRef = &tenant
	svc_meta_json, _ := json.Marshal(&vs_meta.ServiceMetadata)
	svc_meta := string(svc_meta_json)
	vs.ServiceMetadata = &svc_meta

	if vs_meta.SNIParent {
		// This is a SNI parent
		utils.AviLog.Info.Printf("VS %s is a SNI Parent", vs_meta.Name)
		vh_parent := "VS_TYPE_VH_PARENT"
		vs.Type = &vh_parent
	}
	// TODO other fields like cloud_ref, mix of TCP & UDP protocols, etc.

	for _, pp := range vs_meta.PortProto {
		port := pp.Port
		svc := avimodels.Service{Port: &port}
		if pp.Protocol == "tcp" && vs_meta.NetworkProfile == "System-UDP-Fast-Path" {
			onw_profile := "/api/networkprofile/?name=System-TCP-Proxy"
			svc.OverrideNetworkProfileRef = &onw_profile
		} else if pp.Protocol == "udp" && vs_meta.NetworkProfile == "System-TCP-Proxy" {
			onw_profile := "/api/networkprofile/?name=System-UDP-Fast-Path"
			svc.OverrideNetworkProfileRef = &onw_profile
		}
		if pp.Secret != "" {
			ssl_enabled := true
			svc.EnableSsl = &ssl_enabled
		}
		vs.Services = append(vs.Services, &svc)
	}

	var rest_ops []*utils.RestOp

	var i int32
	i = 0
	var httpPolicyCollection []*avimodels.HTTPPolicies
	for _, http := range httppolicynode {
		// Update them on the VS object
		var j int32
		j = i + 11
		i = i + 1
		httpPolicy := fmt.Sprintf("/api/httppolicyset/?name=%s", http.Name)
		httpPolicies := &avimodels.HTTPPolicies{HTTPPolicySetRef: &httpPolicy, Index: &j}
		httpPolicyCollection = append(httpPolicyCollection, httpPolicies)
	}
	vs.HTTPPolicies = httpPolicyCollection
	var rest_op utils.RestOp
	var path string
	if rest_method == utils.RestPut {
		path = "/api/virtualservice/" + cache_obj.Uuid
		rest_op = utils.RestOp{Path: path, Method: rest_method, Obj: vs,
			Tenant: vs_meta.Tenant, Model: "VirtualService", Version: utils.CtrlVersion}
		rest_ops = append(rest_ops, &rest_op)

	} else {
		macro := utils.AviRestObjMacro{ModelName: "VirtualService", Data: vs}
		path = "/api/macro"
		rest_op = utils.RestOp{Path: path, Method: rest_method, Obj: macro,
			Tenant: vs_meta.Tenant, Model: "VirtualService", Version: utils.CtrlVersion}
		rest_ops = append(rest_ops, &rest_op)

	}

	utils.AviLog.Info.Print(spew.Sprintf("VS Restop %v K8sAviVsMeta %v\n", utils.Stringify(rest_op),
		*vs_meta))
	return rest_ops
}

func AviVsCacheAdd(cache *utils.AviObjCache, rest_op *utils.RestOp) error {
	if (rest_op.Err != nil) || (rest_op.Response == nil) {
		utils.AviLog.Warning.Printf("rest_op has err or no reponse")
		return errors.New("Errored rest_op")
	}

	resp_elems, ok := RestRespArrToObjByType(rest_op, "virtualservice")
	if ok != nil || resp_elems == nil {
		utils.AviLog.Warning.Printf("Unable to find pool obj in resp %v", rest_op.Response)
		return errors.New("pool not found")
	}

	for _, resp := range resp_elems {
		name, ok := resp["name"].(string)
		if !ok {
			utils.AviLog.Warning.Printf("Name not present in response %v", resp)
			return errors.New("Name not present in response")
		}

		uuid, ok := resp["uuid"].(string)
		if !ok {
			utils.AviLog.Warning.Printf("Uuid not present in response %v", resp)
			return errors.New("Uuid not present in response")
		}

		cksum := resp["cloud_config_cksum"].(string)
		utils.AviLog.Info.Printf("VS INFORMATION %s", utils.Stringify(resp))
		var svc_mdata interface{}
		var svc_mdata_map map[string]interface{}
		var svc_mdata_obj utils.ServiceMetadataObj

		if err := json.Unmarshal([]byte(resp["service_metadata"].(string)),
			&svc_mdata); err == nil {
			svc_mdata_map, ok = svc_mdata.(map[string]interface{})
			if !ok {
				utils.AviLog.Warning.Printf(`resp %v svc_mdata %T has invalid 
								service_metadata type`, resp, svc_mdata)
				svc_mdata_obj = utils.ServiceMetadataObj{}
			} else {
				SvcMdataMapToObj(&svc_mdata_map, &svc_mdata_obj)
			}
		} else {
			utils.AviLog.Warning.Printf("resp %v has invalid service_metadata value",
				resp)
			svc_mdata_obj = utils.ServiceMetadataObj{}
		}
		vh_parent_uuid, found_parent := resp["vh_parent_vs_ref"]
		if found_parent {
			// the uuid is expected to be in the format: "https://IP:PORT/api/virtualservice/virtualservice-88fd9718-f4f9-4e2b-9552-d31336330e0e#mygateway"
			vs_uuid := ExtractVsUuid(vh_parent_uuid.(string))
			utils.AviLog.Info.Printf("Extracted the vs uuid from parent ref: %s", vs_uuid)
			// Now let's get the VS key from this uuid
			vsKey, foundvscache := cache.VsCache.AviCacheGetKeyByUuid(vs_uuid)
			utils.AviLog.Info.Printf("Extracted the VS key from the uuid :%s", vsKey)
			if foundvscache {
				vs_obj := getVsCacheObj(vsKey.(utils.NamespaceName))
				if !utils.HasElem(vs_obj.SNIChildCollection, uuid) {
					vs_obj.SNIChildCollection = append(vs_obj.SNIChildCollection, uuid)
				}
			} else {
				vs_cache_obj := utils.AviVsCache{Name: ExtractVsName(vh_parent_uuid.(string)), Tenant: rest_op.Tenant,
					SNIChildCollection: []string{uuid}}
				cache.VsCache.AviCacheAdd(vsKey, &vs_cache_obj)
				utils.AviLog.Info.Print(spew.Sprintf("Added VS cache key during SNI update %v val %v\n", vsKey,
					vs_cache_obj))
			}
		}
		k := utils.NamespaceName{Namespace: rest_op.Tenant, Name: name}
		vs_cache, ok := cache.VsCache.AviCacheGet(k)
		if ok {
			vs_cache_obj, found := vs_cache.(*utils.AviVsCache)
			if found {
				vs_cache_obj.Uuid = uuid
				vs_cache_obj.CloudConfigCksum = cksum
				utils.AviLog.Info.Print(spew.Sprintf("Updated VS cache key %v val %v\n", k,
					utils.Stringify(vs_cache_obj)))
			}
		} else {
			vs_cache_obj := utils.AviVsCache{Name: name, Tenant: rest_op.Tenant,
				Uuid: uuid, CloudConfigCksum: cksum,
				ServiceMetadata: svc_mdata_obj}
			cache.VsCache.AviCacheAdd(k, &vs_cache_obj)
			utils.AviLog.Info.Print(spew.Sprintf("Added VS cache key %v val %v\n", k,
				vs_cache_obj))
		}

	}

	return nil
}

func ExtractVsUuid(word string) string {
	r, _ := regexp.Compile("virtualservice-.*.#")
	result := r.FindAllString(word, -1)
	if len(result) == 1 {
		return result[0][:len(result[0])-1]
	}
	return ""
}

func ExtractVsName(word string) string {
	r, _ := regexp.Compile("#.*")
	result := r.FindAllString(word, -1)
	if len(result) == 1 {
		return result[0][1:]
	}
	return ""
}

func AviVsCacheDel(vs_cache *utils.AviCache, rest_op *utils.RestOp) error {

	key := utils.NamespaceName{Namespace: rest_op.Tenant, Name: rest_op.ObjName}
	vs_cache.AviCacheDelete(key)

	return nil
}

func AviVSDel(uuid string, tenant string) *utils.RestOp {
	path := "/api/virtualservice/" + uuid
	rest_op := utils.RestOp{Path: path, Method: "DELETE",
		Tenant: tenant, Model: "VirtualService", Version: utils.CtrlVersion}
	utils.AviLog.Info.Print(spew.Sprintf("VirtualService DELETE Restop %v \n",
		utils.Stringify(rest_op)))
	return &rest_op
}
