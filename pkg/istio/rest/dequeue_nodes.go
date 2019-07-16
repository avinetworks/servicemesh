/*
* [2013] - [2019] Avi Networks Incorporated
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
	"fmt"

	"github.com/avinetworks/servicemesh/pkg/istio/nodes"
	"github.com/avinetworks/servicemesh/pkg/istio/objects"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

func DeQueueNodes(key string) {
	// Got the key from the Graph Layer - let's fetch the model
	ok, avimodelIntf := objects.SharedAviGraphLister().Get(key)
	if !ok {
		utils.AviLog.Info.Printf("No model found for the key %s", key)
		return
	}
	var rest_ops []*utils.RestOp
	var pools_to_delete []utils.NamespaceName
	var pgs_to_delete []utils.NamespaceName
	var https_to_delete []utils.NamespaceName
	avimodel := avimodelIntf.(*nodes.AviObjectGraph)
	// Order would be this: 1. Pools 2. PGs  3. HTTPPolicies. 4. VS
	// Get the pools
	gatewayNs, vsName := utils.ExtractGatewayNamespace(key)
	pools := avimodel.GetAviPools()
	cache := utils.SharedAviObjCache()
	vsKey := utils.NamespaceName{Namespace: gatewayNs, Name: vsName}
	poolGroups := avimodel.GetAviPoolGroups()
	HTTPPolicies := avimodel.GetAviHttpPolicies()
	aviVSes := avimodel.GetAviVS()
	if len(aviVSes) != 1 {
		utils.AviLog.Error.Fatalf("More than one VS received. One Gateway should always map to one VS")
		return
	}
	vs_cache, found := cache.VsCache.AviCacheGet(vsKey)

	//Decide pool create/delete/update
	if found {
		vs_cache_obj, ok := vs_cache.(*utils.AviVsCache)
		if !ok {
			utils.AviLog.Warning.Printf("Invalid VS object. Cannot cast. Not doing anything")
			return
		}
		pools_in_cache := make([]utils.NamespaceName, len(vs_cache_obj.PoolKeyCollection))
		copy(pools_in_cache, vs_cache_obj.PoolKeyCollection)
		pools_to_delete, rest_ops = PoolCU(pools, pools_in_cache, gatewayNs, cache, rest_ops)
		pgs_in_cache := make([]utils.NamespaceName, len(vs_cache_obj.PGKeyCollection))
		copy(pgs_in_cache, vs_cache_obj.PGKeyCollection)
		pgs_to_delete, rest_ops = PoolGroupCU(poolGroups, pgs_in_cache, gatewayNs, cache, rest_ops)
		httpps_in_cache := make([]utils.NamespaceName, len(vs_cache_obj.HTTPKeyCollection))
		copy(httpps_in_cache, vs_cache_obj.HTTPKeyCollection)
		https_to_delete, rest_ops = HTTPPolicyCU(HTTPPolicies, httpps_in_cache, gatewayNs, rest_ops)
		// The VS cache was found.
		// Cache found. Let's compare the checksums
		if vs_cache_obj.CloudConfigCksum == fmt.Sprint(aviVSes[0].GetCheckSum()) {
			utils.AviLog.Info.Printf("The checksums are same for VS %s, not doing anything", vs_cache_obj.Name)
		} else {
			utils.AviLog.Info.Printf("The stored checksum for VS is %v, and the obtained checksum for VS is: %v", vs_cache_obj.CloudConfigCksum, fmt.Sprint(aviVSes[0].GetCheckSum()))
			// The checksums are different, so it should be a PUT call.
			restOp := AviVsBuild(aviVSes[0], HTTPPolicies, utils.RestPut, vs_cache_obj)
			rest_ops = append(rest_ops, restOp...)
		}
	} else {
		_, rest_ops = PoolCU(pools, nil, gatewayNs, cache, rest_ops)
		_, rest_ops = PoolGroupCU(poolGroups, nil, gatewayNs, cache, rest_ops)
		_, rest_ops = HTTPPolicyCU(HTTPPolicies, nil, gatewayNs, rest_ops)
		// The cache was not found - it's a POST call.
		restOp := AviVsBuild(aviVSes[0], HTTPPolicies, utils.RestPost, nil)
		rest_ops = append(rest_ops, restOp...)
	}

	// Let's populate all the DELETE entries
	rest_ops = HTTPPolicyDelete(https_to_delete, gatewayNs, rest_ops)
	rest_ops = PoolGroupDelete(pgs_to_delete, gatewayNs, rest_ops)
	rest_ops = PoolDelete(pools_to_delete, gatewayNs, rest_ops)

	avi_rest_client_pool := utils.SharedAVIClients()
	aviclient := avi_rest_client_pool.AviClient[0]
	utils.AviLog.Info.Printf("The list of REST OPS: %s", utils.Stringify(rest_ops))
	err := avi_rest_client_pool.AviRestOperate(aviclient, rest_ops)
	if err != nil {
		utils.AviLog.Warning.Printf("There was an error sending the macro %s", err)

		// Iterate over rest_ops in reverse and delete created objs
		for i := len(rest_ops) - 1; i >= 0; i-- {
			if rest_ops[i].Err == nil {
				if rest_ops[i].Method == utils.RestPost {
					resp_arr, ok := rest_ops[i].Response.([]interface{})
					if !ok {
						utils.AviLog.Warning.Printf("Invalid resp type for rest_op %v", rest_ops[i])
						continue
					}
					resp, ok := resp_arr[0].(map[string]interface{})
					if ok {
						uuid, ok := resp["uuid"].(string)
						if !ok {
							utils.AviLog.Warning.Printf("Invalid resp type for uuid %v",
								resp)
							continue
						}
						url := utils.AviModelToUrl(rest_ops[i].Model) + "/" + uuid
						err := aviclient.AviSession.Delete(url)
						if err != nil {
							utils.AviLog.Warning.Printf("Error %v deleting url %v", err, url)
						} else {
							utils.AviLog.Info.Printf("Success deleting url %v", url)
						}
					} else {
						utils.AviLog.Warning.Printf("Invalid resp for rest_op %v", rest_ops[i])
					}
				}
			}
		}
	} else {
		// Add to local obj caches
		for _, rest_op := range rest_ops {
			if rest_op.Err == nil && (rest_op.Method == utils.RestPost || rest_op.Method == utils.RestPut) {
				if rest_op.Model == "Pool" {
					AviPoolCacheAdd(cache, rest_op, vsKey)
				} else if rest_op.Model == "VirtualService" {
					AviVsCacheAdd(cache, rest_op)
				} else if rest_op.Model == "PoolGroup" {
					AviPGCacheAdd(cache, rest_op, vsKey)
				} else if rest_op.Model == "HTTPPolicySet" {
					AviHTTPPolicyCacheAdd(cache, rest_op, vsKey)
				}
			} else {
				if rest_op.Model == "Pool" {
					AviPoolCacheDel(cache, rest_op, vsKey)
				} else if rest_op.Model == "VirtualService" {
					AviVsCacheDel(cache.VsCache, rest_op)
				} else if rest_op.Model == "PoolGroup" {
					AviPGCacheDel(cache, rest_op, vsKey)
				} else if rest_op.Model == "HTTPPolicySet" {
					AviHTTPCacheDel(cache, rest_op, vsKey)
				}
			}
		}

	}
}

func PoolDelete(pools_to_delete []utils.NamespaceName, gatewayNs string, rest_ops []*utils.RestOp) []*utils.RestOp {
	cache := utils.SharedAviObjCache()
	for _, del_pool := range pools_to_delete {
		// fetch trhe pool uuid from cache
		pool_key := utils.NamespaceName{Namespace: gatewayNs, Name: del_pool.Name}
		pool_cache, ok := cache.PoolCache.AviCacheGet(pool_key)
		if ok {
			pool_cache_obj, _ := pool_cache.(*utils.AviPoolCache)
			restOp := AviPoolDel(pool_cache_obj.Uuid, gatewayNs)
			restOp.ObjName = del_pool.Name
			rest_ops = append(rest_ops, restOp)
		}
	}
	return rest_ops
}

func HTTPPolicyDelete(https_to_delete []utils.NamespaceName, gatewayNs string, rest_ops []*utils.RestOp) []*utils.RestOp {
	cache := utils.SharedAviObjCache()
	for _, del_http := range https_to_delete {
		// fetch trhe pool uuid from cache
		http_key := utils.NamespaceName{Namespace: gatewayNs, Name: del_http.Name}
		http_cache, ok := cache.HTTPCache.AviCacheGet(http_key)
		if ok {
			http_cache_obj, _ := http_cache.(*utils.AviHTTPCache)
			restOp := AviHttpPolicyDel(http_cache_obj.Uuid, gatewayNs)
			restOp.ObjName = del_http.Name
			rest_ops = append(rest_ops, restOp)
		}
	}
	return rest_ops
}

func PoolGroupDelete(pgs_to_delete []utils.NamespaceName, gatewayNs string, rest_ops []*utils.RestOp) []*utils.RestOp {
	cache := utils.SharedAviObjCache()
	utils.AviLog.Info.Printf("About to delete the PGs %s", pgs_to_delete)
	for _, del_pg := range pgs_to_delete {
		// fetch trhe pool uuid from cache
		pg_key := utils.NamespaceName{Namespace: gatewayNs, Name: del_pg.Name}
		pg_cache, ok := cache.PgCache.AviCacheGet(pg_key)
		if ok {
			pg_cache_obj, _ := pg_cache.(*utils.AviPGCache)
			restOp := AviPGDel(pg_cache_obj.Uuid, gatewayNs)
			restOp.ObjName = del_pg.Name
			rest_ops = append(rest_ops, restOp)
		}
	}
	return rest_ops
}

func PoolCU(pool_nodes []*nodes.AviPoolNode, cache_pool_nodes []utils.NamespaceName, gatewayNs string, cache *utils.AviObjCache, rest_ops []*utils.RestOp) ([]utils.NamespaceName, []*utils.RestOp) {
	// Default is POST
	if cache_pool_nodes != nil {
		for _, pool := range pool_nodes {
			// check in the pool cache to see if this pool exists in AVI
			pool_key := utils.NamespaceName{Namespace: gatewayNs, Name: pool.Name}
			found := utils.HasElem(cache_pool_nodes, pool_key)
			if found {
				cache_pool_nodes = Remove(cache_pool_nodes, pool_key)
				utils.AviLog.Info.Printf("The cache pool nodes are: %v", cache_pool_nodes)
				pool_cache, ok := cache.PoolCache.AviCacheGet(pool_key)
				if ok {
					pool_cache_obj, _ := pool_cache.(*utils.AviPoolCache)
					// Cache found. Let's compare the checksums
					if pool_cache_obj.CloudConfigCksum == fmt.Sprint(pool.GetCheckSum()) {
						utils.AviLog.Info.Printf("The checksums are same for Pool %s, not doing anything", pool.Name)
					} else {
						// The checksums are different, so it should be a PUT call.
						restOp := AviPoolBuild(pool, pool_cache_obj)
						rest_ops = append(rest_ops, restOp)
					}
				}
			} else {
				// Not found - it should be a POST call.
				restOp := AviPoolBuild(pool, nil)
				rest_ops = append(rest_ops, restOp)
			}

		}
	} else {
		// Everything is a POST call
		for _, pool := range pool_nodes {
			restOp := AviPoolBuild(pool, nil)
			rest_ops = append(rest_ops, restOp)
		}

	}
	utils.AviLog.Info.Printf("The POOLS rest_op is %s", utils.Stringify(rest_ops))
	utils.AviLog.Info.Printf("The POOLs to be deleted are: %s", cache_pool_nodes)
	return cache_pool_nodes, rest_ops
}

func PoolGroupCU(pg_nodes []*nodes.AviPoolGroupNode, cache_pg_nodes []utils.NamespaceName, gatewayNs string, cache *utils.AviObjCache, rest_ops []*utils.RestOp) ([]utils.NamespaceName, []*utils.RestOp) {
	utils.AviLog.Info.Printf("Cached PoolGroups before CU :%v", cache_pg_nodes)
	// Default is POST
	if cache_pg_nodes != nil {
		cache := utils.SharedAviObjCache()
		for _, pg := range pg_nodes {
			pg_key := utils.NamespaceName{Namespace: gatewayNs, Name: pg.Name}
			found := utils.HasElem(cache_pg_nodes, pg_key)
			if found {
				cache_pg_nodes = Remove(cache_pg_nodes, pg_key)
				pg_cache, ok := cache.PgCache.AviCacheGet(pg_key)
				if ok {
					pg_cache_obj, _ := pg_cache.(*utils.AviPGCache)
					// Cache found. Let's compare the checksums
					if pg_cache_obj.CloudConfigCksum == fmt.Sprint(pg.GetCheckSum()) {
						utils.AviLog.Info.Printf("The checksums are same for PG %s, not doing anything", pg_cache_obj.Name)
					} else {
						// The checksums are different, so it should be a PUT call.
						restOp := AviPoolGroupBuild(pg, pg_cache_obj)
						rest_ops = append(rest_ops, restOp)
					}
				}
			} else {
				// Not found - it should be a POST call.
				restOp := AviPoolGroupBuild(pg, nil)
				rest_ops = append(rest_ops, restOp)
			}

		}
	} else {
		// Everything is a POST call
		for _, pg := range pg_nodes {
			restOp := AviPoolGroupBuild(pg, nil)
			rest_ops = append(rest_ops, restOp)
		}

	}
	utils.AviLog.Info.Printf("The PGs rest_op is %s", utils.Stringify(rest_ops))
	utils.AviLog.Info.Printf("The PGs to be deleted are: %s", cache_pg_nodes)
	return cache_pg_nodes, rest_ops
}

func HTTPPolicyCU(http_nodes []*nodes.AviHttpPolicySetNode, cache_http_nodes []utils.NamespaceName, gatewayNs string, rest_ops []*utils.RestOp) ([]utils.NamespaceName, []*utils.RestOp) {
	// Default is POST
	if cache_http_nodes != nil {
		cache := utils.SharedAviObjCache()
		for _, http := range http_nodes {
			http_key := utils.NamespaceName{Namespace: gatewayNs, Name: http.Name}
			found := utils.HasElem(cache_http_nodes, http_key)
			if found {
				http_cache, ok := cache.HTTPCache.AviCacheGet(http_key)
				if ok {
					cache_http_nodes = Remove(cache_http_nodes, http_key)
					http_cache_obj, _ := http_cache.(*utils.AviHTTPCache)
					// Cache found. Let's compare the checksums
					if http_cache_obj.CloudConfigCksum == fmt.Sprint(http.GetCheckSum()) {
						utils.AviLog.Info.Printf("The checksums are same for HTTP cache obj %s, not doing anything", http_cache_obj.Name)
					} else {
						// The checksums are different, so it should be a PUT call.
						restOp := AviHttpPSBuild(http, http_cache_obj)
						rest_ops = append(rest_ops, restOp)
					}
				}
			} else {
				// Not found - it should be a POST call.
				restOp := AviHttpPSBuild(http, nil)
				rest_ops = append(rest_ops, restOp)
			}

		}
	} else {
		// Everything is a POST call
		for _, http := range http_nodes {
			restOp := AviHttpPSBuild(http, nil)
			rest_ops = append(rest_ops, restOp)
		}

	}
	utils.AviLog.Info.Printf("The HTTP Policies rest_op is %s", utils.Stringify(rest_ops))
	return cache_http_nodes, rest_ops
}

func Remove(s []utils.NamespaceName, r utils.NamespaceName) []utils.NamespaceName {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
