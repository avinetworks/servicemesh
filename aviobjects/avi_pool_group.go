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

package aviobjects

import (
	"encoding/json"
	"errors"
	"fmt"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/pkg/utils"
	"github.com/davecgh/go-spew/spew"
)

func AviPoolGroupBuild(pg_meta *utils.K8sAviPoolGroupMeta) *utils.RestOp {
	name := pg_meta.Name
	cksum := pg_meta.CloudConfigCksum
	tenant := fmt.Sprintf("/api/tenant/?name=%s", pg_meta.Tenant)
	svc_mdata_json, _ := json.Marshal(&pg_meta.ServiceMetadata)
	svc_mdata := string(svc_mdata_json)
	members := pg_meta.Members
	cr := utils.OSHIFT_K8S_CLOUD_CONNECTOR

	pg := avimodels.PoolGroup{Name: &name, CloudConfigCksum: &cksum,
		CreatedBy: &cr, TenantRef: &tenant, ServiceMetadata: &svc_mdata, Members: members}

	// TODO other fields like cloud_ref and lb algo

	macro := utils.AviRestObjMacro{ModelName: "PoolGroup", Data: pg}

	rest_op := utils.RestOp{Path: "/api/macro", Method: utils.RestPost, Obj: macro,
		Tenant: pg_meta.Tenant, Model: "PoolGroup", Version: CtrlVersion}

	utils.AviLog.Info.Print(spew.Sprintf("PoolGroup Restop %v K8sAviPoolGroupMeta %v\n",
		utils.Stringify(rest_op), *pg_meta))
	return &rest_op
}

func AviPGCacheAdd(pg_cache *utils.AviCache, rest_op *utils.RestOp) error {
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
		pg_cache.AviCacheAdd(k, &pg_cache_obj)

		utils.AviLog.Info.Print(spew.Sprintf("Added PG cache k %v val %v\n", k,
			pg_cache_obj))
	}

	return nil
}
