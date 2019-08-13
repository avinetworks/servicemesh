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

func CloudRestOps(cloudfilename string) bool {
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
	var rest_ops []*utils.RestOp
	avi_rest_client_pool := utils.SharedAVIClients()
	aviclient := avi_rest_client_pool.AviClient[0]

	path := "/api/cloud/"
	rest_op := utils.RestOp{Path: path, Method: "POST", Obj: cloud,
		Tenant: "admin", Model: "Cloud", Version: utils.CtrlVersion}
	rest_ops = append(rest_ops, &rest_op)
	err = avi_rest_client_pool.AviRestOperate(aviclient, rest_ops)
	if err != nil {
		utils.AviLog.Warning.Printf("Couldn't create the cloud name:%s due to the following error :%s ", *cloud.Name, err.Error())
		return false
	} else {
		utils.AviLog.Info.Printf("Successfully created the cloud :%s", *cloud.Name)
	}
	return true
}
