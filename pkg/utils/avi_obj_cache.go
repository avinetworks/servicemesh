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
	"fmt"
	"net/url"
	"strings"

	"github.com/avinetworks/sdk/go/clients"
	"github.com/avinetworks/sdk/go/session"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type AviObjCache struct {
	client         *kubernetes.Clientset
	VsCache        *AviCache
	PgCache        *AviCache
	PoolCache      *AviCache
	SvcToPoolCache *AviMultiCache
	SvcToPgCache   *AviMultiCache
	informers      *Informers
}

func NewAviObjCache(client *kubernetes.Clientset, informers *Informers) *AviObjCache {
	c := AviObjCache{client: client, informers: informers}
	c.VsCache = NewAviCache()
	c.PgCache = NewAviCache()
	c.PoolCache = NewAviCache()
	c.SvcToPoolCache = NewAviMultiCache()
	c.SvcToPgCache = NewAviMultiCache()
	return &c
}

func (c *AviObjCache) AviObjCachePopulate(client *clients.AviClient,
	version string, cloud string) {
	SetTenant := session.SetTenant("*")
	SetTenant(client.AviSession)
	SetVersion := session.SetVersion(version)
	SetVersion(client.AviSession)

	var rest_response interface{}
	var svc_mdata_obj ServiceMetadataObj
	var svc_mdata interface{}
	var svc_mdata_map map[string]interface{}
	var err error
	var pool_name string

	avi_pools := make(map[string]bool)

	// TODO Retrieve just fields we care about
	uri := "/api/pool?include_name=true&cloud_ref.name=" + cloud
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
				return
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

				avi_pools[pool_cache_obj.Name] = true

				k := NamespaceName{Namespace: tenant, Name: pool["name"].(string)}
				c.PoolCache.AviCacheAdd(k, &pool_cache_obj)

				AviLog.Info.Printf("Added Pool cache k %v val %v",
					k, pool_cache_obj)
			}
		}
	}

	// TODO Retrieve just fields we care about
	uri = "/api/virtualservice?include_name=true&cloud_ref.name=" + cloud
	err = client.AviSession.Get(uri, &rest_response)

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

				vs_cache_obj := AviVsCache{Name: vs["name"].(string),
					Tenant: tenant, Uuid: vs["uuid"].(string),
					CloudConfigCksum: vs["cloud_config_cksum"].(string),
					ServiceMetadata:  svc_mdata_obj}

				k := NamespaceName{Namespace: tenant, Name: vs["name"].(string)}
				c.VsCache.AviCacheAdd(k, &vs_cache_obj)

				AviLog.Info.Printf("Added Vs cache k %v val %v",
					k, vs_cache_obj)
			}
		}
	}

	// svcs, err := c.informers.ServiceInformer.Lister().List(labels.Everything())
	svcs, err := c.client.CoreV1().Services("").List(v1.ListOptions{})
	if err != nil {
		AviLog.Warning.Printf("Service Lister returned %v", err)
	} else {
		for _, svc := range svcs.Items {
			for _, pp := range svc.Spec.Ports {
				var prot string
				if pp.Protocol == "" {
					prot = "tcp"
				} else {
					prot = strings.ToLower(string(pp.Protocol))
				}
				// pool_name is of the form name_prefix-pool-port-protocol
				// For Service, name_prefix is the Service's name
				pool_name = fmt.Sprintf("%s-pool-%v-%s", svc.Name,
					pp.TargetPort.String(), prot)
			}
			_, pool_pres := avi_pools[pool_name]
			if pool_pres {
				key := NamespaceName{Namespace: svc.Namespace, Name: svc.Name}
				pool_cache_entry := "service/" + pool_name
				c.SvcToPoolCache.AviMultiCacheAdd(key, pool_cache_entry)
				AviLog.Info.Printf(`key %v maps to pool %v in pool cache`,
					key, pool_cache_entry)
			} else {
				AviLog.Warning.Printf(`Service namespace %v name %v pool %v
					 has no corresponding pool`, svc.Namespace, svc.Name,
					pool_name)
			}
		}
	}
}
