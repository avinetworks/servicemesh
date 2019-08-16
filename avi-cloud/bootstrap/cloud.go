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

package bootstrap

import (
	"encoding/json"
	"os"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/utils"
)

func CloudCreate(cloudfilename string, avi_rest_client_pool *utils.AviRestClientPool) bool {
	cloud := HydrateCloudObj(cloudfilename)
	var rest_ops []*utils.RestOp

	path := "/api/cloud/"
	rest_op := utils.RestOp{Path: path, Method: "POST", Obj: cloud,
		Tenant: "admin", Model: "Cloud", Version: utils.CtrlVersion}
	rest_ops = append(rest_ops, &rest_op)
	err := avi_rest_client_pool.AviRestOperate(avi_rest_client_pool.AviClient[0], rest_ops)
	utils.AviLog.Info.Printf("Sent request to create the cloud :%s", *cloud.Name)
	if err != nil {
		utils.AviLog.Warning.Printf("Couldn't create the cloud name:%s due to the following error :%s ", *cloud.Name, err.Error())
		return false
	} else {
		utils.AviLog.Info.Printf("Successfully created the cloud :%s", *cloud.Name)
	}
	return true
}

func CloudUpdate(cloudfilename string, avi_rest_client_pool *utils.AviRestClientPool) bool {

	var rest_response interface{}
	cloudName := os.Getenv("CLOUD_NAME")
	path := "/api/cloud/"
	err := avi_rest_client_pool.AviClient[0].AviSession.Get(path, &rest_response)
	if err != nil {
		utils.AviLog.Warning.Printf(`Cloud Get uri %v returned err %v`, path, err)
		return false
	}
	resp, ok := rest_response.(map[string]interface{})
	if !ok {
		utils.AviLog.Warning.Printf(`Cloud Get uri %v returned %v type %T`, path,
			rest_response, rest_response)
	} else {
		utils.AviLog.Info.Printf("Cloud Get uri %v returned %v clouds", path,
			resp["count"])
		results, ok := resp["results"].([]interface{})
		if !ok {
			utils.AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T`, resp["results"])
			return false
		}
		for _, cloud_intf := range results {
			cloud, ok := cloud_intf.(map[string]interface{})
			if !ok {
				utils.AviLog.Warning.Printf(`cloud_intf not of type map[string]
									 interface{}. Instead of type %T`, cloud_intf)
				continue
			}
			if cloud["name"] == cloudName {
				utils.AviLog.Info.Printf("Cloud with name %s, found!", cloudName)
				// Make a PUT call on this network
				cloud_obj := HydrateCloudObj(cloudfilename)
				if cloudName != "" {
					network_ref := "/api/ipamdnsproviderprofile/?name=ns"
					utils.AviLog.Info.Printf("Setting network reference %s to the cloud :%s", network_ref, cloudName)
					cloud_obj.IPAMProviderRef = &network_ref
					dns_provider_ref := "/api/ipamdnsproviderprofile/?name=nsdns"
					cloud_obj.DNSProviderRef = &dns_provider_ref
					path := "/api/cloud/" + cloud["uuid"].(string)
					var rest_ops []*utils.RestOp
					rest_op := utils.RestOp{Path: path, Method: "PUT", Obj: cloud_obj,
						Tenant: "admin", Model: "Cloud", Version: utils.CtrlVersion}
					rest_ops = append(rest_ops, &rest_op)
					err := avi_rest_client_pool.AviRestOperate(avi_rest_client_pool.AviClient[0], rest_ops)
					if err == nil {
						return true
					} else {
						utils.AviLog.Info.Printf("There was an error in updating the cloud :%v", err)
					}
				}
			}
		}

	}
	return false
}

func HydrateCloudObj(cloudfilename string) avimodels.Cloud {
	file, err := os.Open(cloudfilename)
	if err != nil {
		// This will terminate the process.
		utils.AviLog.Warning.Fatal(err)
	}
	defer file.Close()

	cloud := avimodels.Cloud{}
	jsonParser := json.NewDecoder(file)
	if err = jsonParser.Decode(&cloud); err != nil {
		utils.AviLog.Warning.Printf("parsing cloud config file %s", err.Error())
		os.Exit(1)
	}
	// Let's set some values based on environment variables.
	master_node := os.Getenv("MASTER_NODES")
	token := os.Getenv("SERVICE_TOKEN")
	cloud_name := os.Getenv("CLOUD_NAME")
	if token == "" || master_node == "" || cloud_name == "" {
		utils.AviLog.Info.Printf("Cloud Name, Master Node information and Service Token information are mandatory.")
		os.Exit(1)
	}
	cloud.Oshiftk8sConfiguration.MasterNodes = append(cloud.Oshiftk8sConfiguration.MasterNodes, master_node)
	cloud.Oshiftk8sConfiguration.ServiceAccountToken = &token
	cloud.Name = &cloud_name
	return cloud
}

func GetAndUpdateNetworkRef(networkName string, networkfilename string) bool {
	avi_rest_client_pool := utils.SharedAVIClients()
	aviclient := avi_rest_client_pool.AviClient[0]
	var rest_response interface{}
	path := "/api/network/"
	err := aviclient.AviSession.Get(path, &rest_response)
	if err != nil {
		utils.AviLog.Warning.Printf(`Network Get uri %v returned err %v`, path, err)
		return false
	}
	resp, ok := rest_response.(map[string]interface{})
	if !ok {
		utils.AviLog.Warning.Printf(`Network Get uri %v returned %v type %T`, path,
			rest_response, rest_response)
	} else {
		utils.AviLog.Info.Printf("Network Get uri %v returned %v networks", path,
			resp["count"])
		results, ok := resp["results"].([]interface{})
		if !ok {
			utils.AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T`, resp["results"])
			return false
		}
		for _, network_intf := range results {
			network, ok := network_intf.(map[string]interface{})
			if !ok {
				utils.AviLog.Warning.Printf(`network_intf not of type map[string]
									 interface{}. Instead of type %T`, network_intf)
				continue
			}
			if network["name"] == networkName {
				utils.AviLog.Info.Printf("Network with name %s, found!", networkName)
				// Make a PUT call on this network
				network_obj := HydrateNetwork(networkfilename)
				cloud_name := os.Getenv("CLOUD_NAME")
				if cloud_name != "" {
					cloud_ref := "/api/cloud/?name=" + cloud_name
					utils.AviLog.Info.Printf("Setting cloud reference to the network profile :%s", cloud_name)
					network_obj.CloudRef = &cloud_ref
					path := "/api/network/" + network["uuid"].(string)
					var rest_ops []*utils.RestOp
					rest_op := utils.RestOp{Path: path, Method: "PUT", Obj: network_obj,
						Tenant: "admin", Model: "Network", Version: utils.CtrlVersion}
					rest_ops = append(rest_ops, &rest_op)
					err := avi_rest_client_pool.AviRestOperate(aviclient, rest_ops)
					if err == nil {
						return true
					} else {
						utils.AviLog.Info.Printf("There was an error in updating the network :%v", err)
					}
				}
			}
		}

	}
	return false
}

func CheckTenantSync() bool {
	cloud_name := os.Getenv("CLOUD_NAME")
	if cloud_name == "" {
		utils.AviLog.Info.Printf("Cloud Name, Master Node information and Service Token information are mandatory.")
		os.Exit(1)
	}

	avi_rest_client_pool := utils.SharedAVIClients()
	aviclient := avi_rest_client_pool.AviClient[0]
	var rest_response interface{}
	path := "/api/tenant/"
	err := aviclient.AviSession.Get(path, &rest_response)
	if err != nil {
		utils.AviLog.Warning.Printf(`Tenant Get uri %v returned err %v`, path, err)
		return false
	}
	resp, ok := rest_response.(map[string]interface{})
	if !ok {
		utils.AviLog.Warning.Printf(`Tenant Get uri %v returned %v type %T`, path,
			rest_response, rest_response)
	} else {
		utils.AviLog.Info.Printf("Tenant Get uri %v returned %v tenants", path,
			resp["count"])
		results, ok := resp["results"].([]interface{})
		if !ok {
			utils.AviLog.Warning.Printf(`results not of type []interface{}
								 Instead of type %T`, resp["results"])
			return false
		}
		for _, tenant_intf := range results {
			tenant, ok := tenant_intf.(map[string]interface{})
			if !ok {
				utils.AviLog.Warning.Printf(`tenant_intf not of type map[string]
									 interface{}. Instead of type %T`, tenant_intf)
				continue
			}
			// The istio-system tenant signifies that Istio's tenants have been synced to AVI.
			if tenant["name"] == "istio-system" {
				return true
			}
		}

	}
	return false
}
