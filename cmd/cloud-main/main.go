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

	cloud "github.com/avinetworks/servicemesh/avi-cloud/bootstrap"
)

// This code base should be kept independent from the rest of the AMC since it's only meant for bootstraping
func main() {
	cloud.IPAMRestOps("nsipam.json")
	cloud.IPAMProviderProfileRestOps("ipamprofile.json")
	cloud.IPAMDNSProfileRestOps("ipamdnsprofile.json")
	// At this time, the above 3 commands may result into failures given there might be network profiles pre-created. So we would not care
	// about their return status. However, for now assume that the cloud creation should succeed and only then we will proceed.
	success := cloud.CloudRestOps("cloud.json")
	if !success {
		os.Exit(1)
	}
}
