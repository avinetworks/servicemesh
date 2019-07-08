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

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/pkg/istio/nodes"
	"github.com/avinetworks/servicemesh/pkg/utils"
	"github.com/davecgh/go-spew/spew"
)

func AviPoolGroupBuild(pg_meta *nodes.AviPoolGroupNode, cache_obj *utils.AviPGCache) *utils.RestOp {
	name := pg_meta.Name
	cksum := pg_meta.CloudConfigCksum
	cksumString := fmt.Sprint(cksum)
	tenant := fmt.Sprintf("/api/tenant/?name=%s", pg_meta.Tenant)
	svc_mdata_json, _ := json.Marshal(&pg_meta.ServiceMetadata)
	svc_mdata := string(svc_mdata_json)
	members := pg_meta.Members
	cr := utils.OSHIFT_K8S_CLOUD_CONNECTOR
	pg := avimodels.PoolGroup{Name: &name, CloudConfigCksum: &cksumString,
		CreatedBy: &cr, TenantRef: &tenant, ServiceMetadata: &svc_mdata, Members: members}
	// TODO other fields like cloud_ref and lb algo

	macro := utils.AviRestObjMacro{ModelName: "PoolGroup", Data: pg}

	var path string
	var rest_op utils.RestOp
	if cache_obj != nil {
		path = "/api/poolgroup/" + cache_obj.Uuid
		rest_op = utils.RestOp{Path: path, Method: utils.RestPut, Obj: pg,
			Tenant: pg_meta.Tenant, Model: "PoolGroup", Version: utils.CtrlVersion}
	} else {
		path = "/api/macro"
		rest_op = utils.RestOp{Path: path, Method: utils.RestPost, Obj: macro,
			Tenant: pg_meta.Tenant, Model: "PoolGroup", Version: utils.CtrlVersion}
	}

	return &rest_op
}

func AviPGDel(uuid string, tenant string) *utils.RestOp {
	path := "/api/poolgroup/" + uuid
	rest_op := utils.RestOp{Path: path, Method: "DELETE",
		Tenant: tenant, Model: "PoolGroup", Version: utils.CtrlVersion}
	utils.AviLog.Info.Print(spew.Sprintf("PG DELETE Restop %v \n",
		utils.Stringify(rest_op)))
	return &rest_op
}

func AviPGCacheAdd(cache *utils.AviObjCache, rest_op *utils.RestOp, vsKey utils.NamespaceName) error {
	if (rest_op.Err != nil) || (rest_op.Response == nil) {
		utils.AviLog.Warning.Printf("rest_op has err or no reponse for PG")
		return errors.New("Errored rest_op")
	}

	resp_elems, ok := RestRespArrToObjByType(rest_op, "poolgroup")
	if ok != nil || resp_elems == nil {
		utils.AviLog.Warning.Printf("Unable to find pool group obj in resp %v", rest_op.Response)
		return errors.New("poolgroup not found")
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

		pg_cache_obj := utils.AviPGCache{Name: name, Tenant: rest_op.Tenant,
			Uuid:             uuid,
			CloudConfigCksum: cksum, ServiceMetadata: svc_mdata_obj}

		k := utils.NamespaceName{Namespace: rest_op.Tenant, Name: name}
		cache.PgCache.AviCacheAdd(k, &pg_cache_obj)
		// Update the VS object
		vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
		if ok {
			vs_cache_obj, found := vs_cache.(*utils.AviVsCache)
			if found {
				utils.AviLog.Info.Printf("The VS cache before modification by PG creation is :%v", utils.Stringify(vs_cache_obj))
				if vs_cache_obj.PGKeyCollection == nil {
					vs_cache_obj.PGKeyCollection = []utils.NamespaceName{k}
				} else {
					if !Contains(vs_cache_obj.PGKeyCollection, k) {
						vs_cache_obj.PGKeyCollection = append(vs_cache_obj.PGKeyCollection, k)
					}
				}
				utils.AviLog.Info.Printf("Modified the VS cache object for PG collection. The cache now is :%v", utils.Stringify(vs_cache_obj))
			}

		} else {
			vs_cache_obj := utils.AviVsCache{Name: vsKey.Name, Tenant: vsKey.Namespace,
				PGKeyCollection: []utils.NamespaceName{k}}
			cache.VsCache.AviCacheAdd(vsKey, &vs_cache_obj)
			utils.AviLog.Info.Print(spew.Sprintf("Added VS cache key during poolgroup update %v val %v\n", vsKey,
				vs_cache_obj))
		}
		utils.AviLog.Info.Print(spew.Sprintf("Added PG cache k %v val %v\n", k,
			pg_cache_obj))
	}

	return nil
}

func AviPGCacheDel(cache *utils.AviObjCache, rest_op *utils.RestOp, vsKey utils.NamespaceName) error {
	key := utils.NamespaceName{Namespace: rest_op.Tenant, Name: rest_op.ObjName}
	cache.PgCache.AviCacheDelete(key)
	vs_cache, ok := cache.VsCache.AviCacheGet(vsKey)
	if ok {
		vs_cache_obj, found := vs_cache.(*utils.AviVsCache)
		if found {
			vs_cache_obj.PGKeyCollection = Remove(vs_cache_obj.PGKeyCollection, key)
		}
	}

	return nil

}
