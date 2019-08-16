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

package main

import (
	"os"
	"time"

	cloud "github.com/avinetworks/servicemesh/avi-cloud/bootstrap"
	"github.com/avinetworks/servicemesh/utils"
)

// This code base should be kept independent from the rest of the AMC since it's only meant for bootstraping
func main() {
	avi_rest_client_pool := utils.SharedAVIClients()
	if avi_rest_client_pool == nil || len(avi_rest_client_pool.AviClient) < 1 {
		utils.AviLog.Info.Printf("Couldn't reach the provided controller IP with the credentials. Pls check the config and re-run this operation")
		os.Exit(1)
	}
	// At this time, the above 3 commands may result into failures given there might be network profiles pre-created. So we would not care
	// about their return status. However, for now assume that the cloud creation should succeed and only then we will proceed.
	cloud.CloudCreate("cloud.json", avi_rest_client_pool)

	//cloud.GetAndUpdateNetworkRef("northsouthnetwork", "nsipam.json")

	// Check if the namespaces got synced in the AVI cloud
	synced := retry(3, 10*time.Second, cloud.CheckTenantSync)
	if !synced {
		utils.AviLog.Info.Printf("Giving up re-trying on namespace sync after %v attempts", 3)
		os.Exit(1)
	}
	cloud.IPAMRestOps("nsipam.json", avi_rest_client_pool)
	cloud.IPAMProviderProfileRestOps("ipamprofile.json", avi_rest_client_pool)
	cloud.IPAMDNSProfileRestOps("ipamdnsprofile.json", avi_rest_client_pool)
	cloud.CloudUpdate("cloud.json", avi_rest_client_pool)
}

func retry(attempts int, sleep time.Duration, fn func() bool) bool {
	if tenant_sync := fn(); !tenant_sync {
		if attempts--; attempts > 0 {
			utils.AviLog.Info.Printf("Sleeping for %v seconds", sleep)
			time.Sleep(sleep)
			return retry(attempts, 2*sleep*time.Second, fn)
		}
		return false
	}
	return true
}
