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

package utils

import (
	"encoding/json"
	"net/url"
	"sync"

	"github.com/avinetworks/sdk/go/clients"
	"github.com/avinetworks/sdk/go/session"
)

type AviObjCache struct {
	VsCache        *AviCache
	PgCache        *AviCache
	HTTPCache      *AviCache
	PoolCache      *AviCache
	SvcToPoolCache *AviMultiCache
}

func NewAviObjCache() *AviObjCache {
	c := AviObjCache{}
	c.VsCache = NewAviCache()
	c.PgCache = NewAviCache()
	c.HTTPCache = NewAviCache()
	c.PoolCache = NewAviCache()
	c.SvcToPoolCache = NewAviMultiCache()
	return &c
}

var cacheInstance *AviObjCache
var cacheOnce sync.Once

func SharedAviObjCache() *AviObjCache {
	cacheOnce.Do(func() {
		cacheInstance = NewAviObjCache()
	})
	return cacheInstance
}

func (c *AviObjCache) AviPoolCachePopulate(client *clients.AviClient,
	cloud string, vs_uuid string) []NamespaceName {
	var rest_response interface{}
	var svc_mdata_obj ServiceMetadataObj
	var svc_mdata interface{}
	var svc_mdata_map map[string]interface{}
	var err error
	//var pool_name string
	var pool_key_collection []NamespaceName
	// TODO Retrieve just fields we care about
	uri := "/api/pool?include_name=true&cloud_ref.name=" + cloud + "&referred_by=virtualservice:" + vs_uuid
	err = client.AviSession.Get(uri, &rest_response)

	if err != nil {
		AviLog.Warning.Printf(`Pool Get uri %v returned err %v`, uri, err)
	} else {
		resp, ok := rest_response.(map[string]interface{})
		if !ok {
			AviLog.Warning.Printf(`Pool Get uri %v returned %v type %T`, uri,
				rest_response, rest_response)
		} else {
			AviLog.Info.Printf("Pool Get uri %v returned %v pools", uri,
				resp["count"])
			results, ok := resp["results"].([]interface{})
			if !ok {
				AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T`, resp["results"])
				return nil
			}
			for _, pool_intf := range results {
				pool, ok := pool_intf.(map[string]interface{})
				if !ok {
					AviLog.Warning.Printf(`pool_intf not of type map[string]
									 interface{}. Instead of type %T`, pool_intf)
					continue
				}
				svc_mdata_intf, ok := pool["service_metadata"]
				if ok {
					if err := json.Unmarshal([]byte(svc_mdata_intf.(string)),
						&svc_mdata); err == nil {
						svc_mdata_map, ok = svc_mdata.(map[string]interface{})
						if !ok {
							AviLog.Warning.Printf(`resp %v svc_mdata %T has invalid
									 service_metadata type`, pool, svc_mdata)
						} else {
							crkhey, ok := svc_mdata_map["crud_hash_key"]
							if ok {
								svc_mdata_obj.CrudHashKey = crkhey.(string)
							} else {
								AviLog.Warning.Printf(`service_metadata %v 
										  malformed`, svc_mdata_map)
							}
						}
					}
				} else {
					AviLog.Warning.Printf("service_metadata %v malformed", pool)
				}

				var tenant string
				url, err := url.Parse(pool["tenant_ref"].(string))
				if err != nil {
					AviLog.Warning.Printf(`Error parsing tenant_ref %v in 
											   pool %v`, pool["tenant_ref"], pool)
					continue
				} else if url.Fragment == "" {
					AviLog.Warning.Printf(`Error extracting name tenant_ref %v 
										 in pool %v`, pool["tenant_ref"], pool)
					continue
				} else {
					tenant = url.Fragment
				}

				pool_cache_obj := AviPoolCache{Name: pool["name"].(string),
					Tenant: tenant, Uuid: pool["uuid"].(string),
					LbAlgorithm:      pool["lb_algorithm"].(string),
					CloudConfigCksum: pool["cloud_config_cksum"].(string),
					ServiceMetadata:  svc_mdata_obj}

				k := NamespaceName{Namespace: tenant, Name: pool["name"].(string)}

				c.PoolCache.AviCacheAdd(k, &pool_cache_obj)
				pool_key_collection = append(pool_key_collection, k)
				AviLog.Info.Printf("Added Pool cache key %v val %v",
					k, pool_cache_obj)
			}
		}
	}
	return pool_key_collection
}

func (c *AviObjCache) AviObjCachePopulate(client *clients.AviClient,
	version string, cloud string) {
	SetTenant := session.SetTenant("*")
	SetTenant(client.AviSession)
	SetVersion := session.SetVersion(version)
	SetVersion(client.AviSession)

	// Populate the VS cache
	c.AviObjVSCachePopulate(client, cloud)

}

// TODO (sudswas): Should this be run inside a go routine for parallel population
// to reduce bootup time when the system is loaded. Variable duplication expected.
func (c *AviObjCache) AviObjVSCachePopulate(client *clients.AviClient,
	cloud string) {
	var rest_response interface{}
	var svc_mdata interface{}
	var svc_mdata_map map[string]interface{}
	var svc_mdata_obj ServiceMetadataObj
	// TODO Retrieve just fields we care about
	uri := "/api/virtualservice?include_name=true&cloud_ref.name=" + cloud
	err := client.AviSession.Get(uri, &rest_response)

	if err != nil {
		AviLog.Warning.Printf(`Vs Get uri %v returned err %v`, uri, err)
	} else {
		resp, ok := rest_response.(map[string]interface{})
		if !ok {
			AviLog.Warning.Printf(`Vs Get uri %v returned %v type %T`, uri,
				rest_response, rest_response)
		} else {
			AviLog.Info.Printf("Vs Get uri %v returned %v vses", uri,
				resp["count"])
			results, ok := resp["results"].([]interface{})
			if !ok {
				AviLog.Warning.Printf(`results not of type []interface{}
							 Instead of type %T`, resp["results"])
				return
			}
			for _, vs_intf := range results {
				vs, ok := vs_intf.(map[string]interface{})
				if !ok {
					AviLog.Warning.Printf(`vs_intf not of type map[string]
								 interface{}. Instead of type %T`, vs_intf)
					continue
				}
				svc_mdata_intf, ok := vs["service_metadata"]
				if ok {
					if err := json.Unmarshal([]byte(svc_mdata_intf.(string)),
						&svc_mdata); err == nil {
						svc_mdata_map, ok = svc_mdata.(map[string]interface{})
						if !ok {
							AviLog.Warning.Printf(`resp %v svc_mdata %T has invalid
								 service_metadata type`, vs, svc_mdata)
						} else {
							crkhey, ok := svc_mdata_map["crud_hash_key"]
							if ok {
								svc_mdata_obj.CrudHashKey = crkhey.(string)
							} else {
								AviLog.Warning.Printf(`service_metadata %v 
									  malformed`, svc_mdata_map)
							}
						}
					}
				}
				//vsvip_intf, _ := vs["vip"].(map[string]interface{})
				AviLog.Info.Printf("MY GOD :%v", vs["vip"])
				var tenant string
				url, err := url.Parse(vs["tenant_ref"].(string))
				if err != nil {
					AviLog.Warning.Printf(`Error parsing tenant_ref %v in 
										   vs %v`, vs["tenant_ref"], vs)
					continue
				} else if url.Fragment == "" {
					AviLog.Warning.Printf(`Error extracting name tenant_ref %v 
									 in vs %v`, vs["tenant_ref"], vs)
					continue
				} else {
					tenant = url.Fragment
				}
				pg_key_collection := c.AviPGCachePopulate(client, cloud, vs["uuid"].(string))
				pool_key_collection := c.AviPoolCachePopulate(client, cloud, vs["uuid"].(string))
				http_policy_collection := c.AviHTTPPolicyCachePopulate(client, cloud, vs["uuid"].(string))
				vs_cache_obj := AviVsCache{Name: vs["name"].(string),
					Tenant: tenant, Uuid: vs["uuid"].(string), Vip: nil,
					CloudConfigCksum: vs["cloud_config_cksum"].(string),
					ServiceMetadata:  svc_mdata_obj, PGKeyCollection: pg_key_collection, PoolKeyCollection: pool_key_collection, HTTPKeyCollection: http_policy_collection}
				k := NamespaceName{Namespace: tenant, Name: vs["name"].(string)}
				c.VsCache.AviCacheAdd(k, &vs_cache_obj)

				AviLog.Info.Printf("Added Vs cache k %v val %v",
					k, vs_cache_obj)
			}
		}
	}
}

//Design library methods to remove repeatation of code.
func (c *AviObjCache) AviPGCachePopulate(client *clients.AviClient,
	cloud string, vs_uuid string) []NamespaceName {
	var rest_response interface{}
	var svc_mdata interface{}
	var svc_mdata_map map[string]interface{}
	var svc_mdata_obj ServiceMetadataObj
	var pg_key_collection []NamespaceName
	uri := "/api/poolgroup?include_name=true&cloud_ref.name=" + cloud + "&referred_by=virtualservice:" + vs_uuid
	err := client.AviSession.Get(uri, &rest_response)
	if err != nil {
		AviLog.Warning.Printf(`PG Get uri %v returned err %v`, uri, err)
	} else {
		resp, ok := rest_response.(map[string]interface{})
		if !ok {
			AviLog.Warning.Printf(`PG Get uri %v returned %v type %T`, uri,
				rest_response, rest_response)
		} else {
			AviLog.Info.Printf("PG Get uri %v returned %v PGs", uri,
				resp["count"])
			results, ok := resp["results"].([]interface{})
			if !ok {
				AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T for PGs`, resp["results"])
				return nil
			}
			for _, pg_intf := range results {
				pg, ok := pg_intf.(map[string]interface{})
				if !ok {
					AviLog.Warning.Printf(`pg_intf not of type map[string]
									 interface{}. Instead of type %T`, pg_intf)
					continue
				}
				svc_mdata_intf, ok := pg["service_metadata"]
				if ok {
					if err := json.Unmarshal([]byte(svc_mdata_intf.(string)),
						&svc_mdata); err == nil {
						svc_mdata_map, ok = svc_mdata.(map[string]interface{})
						if !ok {
							AviLog.Warning.Printf(`resp %v svc_mdata %T has invalid
									 service_metadata type for PGs`, pg, svc_mdata)
						} else {
							crkhey, ok := svc_mdata_map["crud_hash_key"]
							if ok {
								svc_mdata_obj.CrudHashKey = crkhey.(string)
							} else {
								AviLog.Warning.Printf(`service_metadata %v
										  malformed`, svc_mdata_map)
							}
						}
					}
				}

				var tenant string
				url, err := url.Parse(pg["tenant_ref"].(string))
				if err != nil {
					AviLog.Warning.Printf(`Error parsing tenant_ref %v in
											   PG %v`, pg["tenant_ref"], pg)
					continue
				} else if url.Fragment == "" {
					AviLog.Warning.Printf(`Error extracting name tenant_ref %v
										 in PG %v`, pg["tenant_ref"], pg)
					continue
				} else {
					tenant = url.Fragment
				}

				pg_cache_obj := AviPGCache{Name: pg["name"].(string),
					Tenant: tenant, Uuid: pg["uuid"].(string),
					CloudConfigCksum: pg["cloud_config_cksum"].(string),
					ServiceMetadata:  svc_mdata_obj}
				k := NamespaceName{Namespace: tenant, Name: pg["name"].(string)}
				c.PgCache.AviCacheAdd(k, &pg_cache_obj)
				AviLog.Info.Printf("Added PG cache key %v val %v",
					k, pg_cache_obj)
				pg_key_collection = append(pg_key_collection, k)
			}
		}
	}
	return pg_key_collection
}

func (c *AviObjCache) AviHTTPPolicyCachePopulate(client *clients.AviClient,
	cloud string, vs_uuid string) []NamespaceName {
	var rest_response interface{}
	var http_key_collection []NamespaceName
	uri := "/api/httppolicyset?include_name=true&referred_by=virtualservice:" + vs_uuid
	err := client.AviSession.Get(uri, &rest_response)
	if err != nil {
		AviLog.Warning.Printf(`HTTPPolicySet Get uri %v returned err %v`, uri, err)
	} else {
		resp, ok := rest_response.(map[string]interface{})
		if !ok {
			AviLog.Warning.Printf(`HTTPPolicySet Get uri %v returned %v type %T`, uri,
				rest_response, rest_response)
		} else {
			AviLog.Info.Printf("HTTPPolicySet Get uri %v returned %v HTTP Policies", uri,
				resp["count"])
			results, ok := resp["results"].([]interface{})
			if !ok {
				AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T for HTTP Policies`, resp["results"])
				return nil
			}
			for _, http_intf := range results {
				http_pol, ok := http_intf.(map[string]interface{})
				if !ok {
					AviLog.Warning.Printf(`http_intf not of type map[string]
									 interface{}. Instead of type %T`, http_intf)
					continue
				}

				var tenant string
				url, err := url.Parse(http_pol["tenant_ref"].(string))
				if err != nil {
					AviLog.Warning.Printf(`Error parsing tenant_ref %v in
											   HTTP Policy %v`, http_pol["tenant_ref"], http_pol)
					continue
				} else if url.Fragment == "" {
					AviLog.Warning.Printf(`Error extracting name tenant_ref %v
										 in HTTP Policy set %v`, http_pol["tenant_ref"], http_pol)
					continue
				} else {
					tenant = url.Fragment
				}
				if http_pol != nil {
					http_cache_obj := AviHTTPCache{Name: http_pol["name"].(string),
						Tenant: tenant, Uuid: http_pol["uuid"].(string),
						CloudConfigCksum: http_pol["cloud_config_cksum"].(string)}
					k := NamespaceName{Namespace: tenant, Name: http_pol["name"].(string)}
					c.HTTPCache.AviCacheAdd(k, &http_cache_obj)
					AviLog.Info.Printf("Added HTTP Policy cache key %v val %v",
						k, http_cache_obj)
					http_key_collection = append(http_key_collection, k)
				}
			}
		}
	}
	return http_key_collection
}
