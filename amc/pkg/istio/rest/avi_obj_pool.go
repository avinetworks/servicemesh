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
	"strings"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/amc/pkg/istio/nodes"
	"github.com/avinetworks/servicemesh/utils"
	"github.com/davecgh/go-spew/spew"
)

func AviPoolBuild(pool_meta *nodes.AviPoolNode, cache_obj *utils.AviPoolCache) *utils.RestOp {
	name := pool_meta.Name
	cksum := pool_meta.CloudConfigCksum
	cksumString := fmt.Sprint(cksum)
	tenant := fmt.Sprintf("/api/tenant/?name=%s", pool_meta.Tenant)
	svc_mdata_json, _ := json.Marshal(&pool_meta.ServiceMetadata)
	svc_mdata := string(svc_mdata_json)
	cr := utils.OSHIFT_K8S_CLOUD_CONNECTOR
	var poolAlgorithm string
	if pool_meta.LbAlgorithm != "" {
		utils.AviLog.Info.Printf("Pool algorithm obtained from Destination Rule :%s", poolAlgorithm)
		poolAlgorithm = pool_meta.LbAlgorithm
	} else {
		// Default case
		utils.AviLog.Info.Printf("Using Default pool algorithm :%s", poolAlgorithm)
		poolAlgorithm = utils.LeastConnection
	}
	cloudRef := "/api/cloud?name=" + utils.CloudName
	pool := avimodels.Pool{Name: &name, CloudConfigCksum: &cksumString,
		CreatedBy: &cr, TenantRef: &tenant, ServiceMetadata: &svc_mdata, LbAlgorithm: &poolAlgorithm, CloudRef: &cloudRef}

	if pool_meta.ServerClientCert != "" && pool_meta.PkiProfile != "" {
		pool.PkiProfileRef = &pool_meta.PkiProfile
		pool.SslKeyAndCertificateRef = &pool_meta.ServerClientCert
		pool.SslProfileRef = &pool_meta.SSLProfileRef

	}
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
		hm = fmt.Sprintf("/api/healthmonitor/?name=%s", utils.AVI_DEFAULT_UDP_HM)
	} else {
		hm = fmt.Sprintf("/api/healthmonitor/?name=%s", utils.AVI_DEFAULT_TCP_HM)
	}
	pool.HealthMonitorRefs = append(pool.HealthMonitorRefs, hm)

	macro := utils.AviRestObjMacro{ModelName: "Pool", Data: pool}

	// TODO Version should be latest from configmap
	var path string
	var rest_op utils.RestOp
	if cache_obj != nil {
		path = "/api/pool/" + cache_obj.Uuid
		rest_op = utils.RestOp{Path: path, Method: utils.RestPut, Obj: pool,
			Tenant: pool_meta.Tenant, Model: "Pool", Version: utils.CtrlVersion}

	} else {
		path = "/api/macro"
		rest_op = utils.RestOp{Path: path, Method: utils.RestPost, Obj: macro,
			Tenant: pool_meta.Tenant, Model: "Pool", Version: utils.CtrlVersion}
	}

	utils.AviLog.Info.Print(spew.Sprintf("Pool Restop %v K8sAviPoolMeta %v\n",
		utils.Stringify(rest_op), *pool_meta))
	return &rest_op
}

func AviPoolDel(uuid string, tenant string) *utils.RestOp {
	path := "/api/pool/" + uuid
	rest_op := utils.RestOp{Path: path, Method: "DELETE",
		Tenant: tenant, Model: "Pool", Version: utils.CtrlVersion}
	utils.AviLog.Info.Print(spew.Sprintf("Pool DELETE Restop %v \n",
		utils.Stringify(rest_op)))
	return &rest_op
}

func AviPoolCacheAdd(cache *utils.AviObjCache, rest_op *utils.RestOp, vsKey utils.NamespaceName) error {
	if (rest_op.Err != nil) || (rest_op.Response == nil) {
		utils.AviLog.Warning.Printf("rest_op has err or no reponse")
		return errors.New("Errored rest_op")
	}

	resp_elems, ok := RestRespArrToObjByType(rest_op, "pool")
	utils.AviLog.Warning.Printf("The pool object response %v", rest_op.Response)
	if ok != nil || resp_elems == nil {
		utils.AviLog.Warning.Printf("Unable to find pool obj in resp %v", rest_op.Response)
		return errors.New("pool not found")
	}

	for _, resp := range resp_elems {
		name, ok := resp["name"].(string)
		if !ok {
			utils.AviLog.Warning.Printf("Name not present in response %v", resp)
			continue
		}

		uuid, ok := resp["uuid"].(string)
		if !ok {
			utils.AviLog.Warning.Printf("Uuid not present in response %v", resp)
			continue
		}

		lb := resp["lb_algorithm"].(string)
		cksum := resp["cloud_config_cksum"].(string)

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
			utils.AviLog.Warning.Printf(`resp %v has invalid service_metadata value 
                                  err %v`, resp, err)
			svc_mdata_obj = utils.ServiceMetadataObj{}
		}

		pool_cache_obj := utils.AviPoolCache{Name: name, Tenant: rest_op.Tenant,
			Uuid: uuid, LbAlgorithm: lb,
			CloudConfigCksum: cksum, ServiceMetadata: svc_mdata_obj}

		k := utils.NamespaceName{Namespace: rest_op.Tenant, Name: name}
		cache.PoolCache.AviCacheAdd(k, &pool_cache_obj)
		// Update the VS object
		vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
		if ok {
			vs_cache_obj, found := vs_cache.(*utils.AviVsCache)
			if found {
				if vs_cache_obj.PoolKeyCollection == nil {
					vs_cache_obj.PoolKeyCollection = []utils.NamespaceName{k}
				} else {
					if !utils.HasElem(vs_cache_obj.PoolKeyCollection, k) {
						utils.AviLog.Info.Printf("Before adding pool collection %v and key :%v", vs_cache_obj.PoolKeyCollection, k)
						vs_cache_obj.PoolKeyCollection = append(vs_cache_obj.PoolKeyCollection, k)
					}
				}
				utils.AviLog.Info.Printf("Modified the VS cache object for Pool Collection. The cache now is :%v", utils.Stringify(vs_cache_obj))
			}
		} else {
			vs_cache_obj := utils.AviVsCache{Name: vsKey.Name, Tenant: vsKey.Namespace,
				PoolKeyCollection: []utils.NamespaceName{k}}
			cache.VsCache.AviCacheAdd(vsKey, &vs_cache_obj)
			utils.AviLog.Info.Print(spew.Sprintf("Added VS cache key during pool update %v val %v\n", vsKey,
				vs_cache_obj))
		}
		utils.AviLog.Info.Print(spew.Sprintf("Added Pool cache k %v val %v\n", k,
			pool_cache_obj))
	}

	return nil
}

func SvcMdataMapToObj(svc_mdata_map *map[string]interface{}, svc_mdata *utils.ServiceMetadataObj) {
	for k, val := range *svc_mdata_map {
		switch k {
		case "crud_hash_key":
			crkhey, ok := val.(string)
			if ok {
				svc_mdata.CrudHashKey = crkhey
			} else {
				utils.AviLog.Warning.Printf("Incorrect type %T in svc_mdata_map %v", val, *svc_mdata_map)
			}
		}
	}
}

// TODO (sudswas): Let's move this to utils when we move types.go to utils.
func RestRespArrToObjByType(rest_op *utils.RestOp, obj_type string) ([]map[string]interface{}, error) {
	var resp_elems []map[string]interface{}

	if rest_op.Method == utils.RestPost {
		resp_arr, ok := rest_op.Response.([]interface{})

		if !ok {
			utils.AviLog.Warning.Printf("Response has unknown type %T", rest_op.Response)
			return nil, errors.New("Malformed response")
		}

		for _, resp_elem := range resp_arr {
			resp, ok := resp_elem.(map[string]interface{})
			if !ok {
				utils.AviLog.Warning.Printf("Response has unknown type %T", resp_elem)
				continue
			}

			avi_url, ok := resp["url"].(string)
			if !ok {
				utils.AviLog.Warning.Printf("url not present in response %v", resp)
				continue
			}

			avi_obj_type, err := utils.AviUrlToObjType(avi_url)
			if err == nil && avi_obj_type == obj_type {
				resp_elems = append(resp_elems, resp)
			}
		}
	} else {
		// The PUT calls are specific for the resource
		resp_elems = append(resp_elems, rest_op.Response.(map[string]interface{}))
	}

	return resp_elems, nil
}

func AviPoolCacheDel(cache *utils.AviObjCache, rest_op *utils.RestOp, vsKey utils.NamespaceName) error {
	key := utils.NamespaceName{Namespace: rest_op.Tenant, Name: rest_op.ObjName}
	cache.PoolCache.AviCacheDelete(key)
	// Delete the pool from the vs cache as well.
	vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
	if ok {
		vs_cache_obj, found := vs_cache.(*utils.AviVsCache)
		if found {
			vs_cache_obj.PoolKeyCollection = Remove(vs_cache_obj.PoolKeyCollection, key)
		}
	}

	return nil
}

func AviSvcToPoolCacheAdd(svc_to_pool_cache *utils.AviMultiCache, rest_op *utils.RestOp,
	prefix string, key utils.NamespaceName) error {
	if (rest_op.Err != nil) || (rest_op.Response == nil) {
		utils.AviLog.Warning.Printf("rest_op has err or no reponse")
		return errors.New("Errored rest_op")
	}

	resp_elems, ok := RestRespArrToObjByType(rest_op, "pool")
	if ok != nil || resp_elems == nil {
		utils.AviLog.Warning.Printf("Unable to find pool obj in resp %v", rest_op.Response)
		return errors.New("pool not found")
	}

	/*
	 * SvcToPoolCache is of the form:
	 * (ns, name) -> (service/name-pool-http-tcp, route/name-route-pool-http-tcp)
	 * Set of all Pools that are affected by change in same endpoints
	 */

	for _, resp := range resp_elems {
		name, ok := resp["name"].(string)
		if !ok {
			utils.AviLog.Warning.Printf("Name not present in response %v", resp)
			continue
		}

		pool_cache_entry := prefix + "/" + name

		svc_to_pool_cache.AviMultiCacheAdd(key, pool_cache_entry)
		utils.AviLog.Info.Printf("Added key %v pool %v to SvcToPoolCache", key,
			pool_cache_entry)
	}

	return nil
}

func AviSvcToPoolCacheDel(svc_to_pool_cache *utils.AviMultiCache,
	prefix string, key utils.NamespaceName) error {
	/*
	 * mkey_map is of the form:
	 * [service/name-pool-http-tcp] = true
	 * [ingress/name-pool-http-tcp] = true
	 */
	mkey_map, ok := svc_to_pool_cache.AviMultiCacheGetKey(key)

	if !ok {
		utils.AviLog.Info.Printf("Key %v not found in svc_to_pool_cache", key)
		return nil
	}

	for ppool_name_intf := range mkey_map {
		ppool_name, ok := ppool_name_intf.(string)
		if !ok {
			utils.AviLog.Warning.Printf("ppool_name_intf %T is not type string",
				ppool_name_intf)
			continue
		}
		elems := strings.Split(ppool_name, "/")
		if prefix == elems[0] {
			utils.AviLog.Info.Printf("Key %v val %s deleted in svc_to_pool_cache",
				key, ppool_name)
			svc_to_pool_cache.AviMultiCacheDeleteVal(key, ppool_name)
		}
	}

	return nil
}
