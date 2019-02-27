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

package main

import (
	"flag"
	"os"

	"github.com/avinetworks/servicemesh/pkg/k8s"
	"github.com/avinetworks/servicemesh/pkg/mcp"
	"github.com/avinetworks/servicemesh/pkg/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := utils.SetupSignalHandler()
	kubeCluster := false
	// Check if we are running inside kubernetes. Hence try authenticating with service token
	cfg, err := rest.InClusterConfig()
	if err != nil {

		utils.AviLog.Warning.Printf("We are not running inside kubernetes cluster. %s", err.Error())

	} else {
		// TODO (sudswas): Remove the hard coding later.
		istioEnabled := "False"
		istioEnabled = os.Getenv("ISTIO_ENABLED")
		if istioEnabled == "True" {
			stop := make(chan struct{})
			mcpServers := []string{"mcp://istio-galley.istio-system.svc:9901"}
			mcpClient := mcp.MCPClient{MCPServerAddrs: mcpServers}
			_ = mcpClient.InitMCPClient()
			// TODO (sudswas): Need to handle the stop signal
			mcpClient.Start(stop)
		}
		utils.AviLog.Info.Println("We are running inside kubernetes cluster. Won't use kubeconfig files.")
		kubeCluster = true

	}

	if kubeCluster == false {
		cfg, err = clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
		if err != nil {
			utils.AviLog.Error.Fatalf("Error building kubeconfig: %s", err.Error())
		}
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		utils.AviLog.Error.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	informers := NewInformers(kubeClient)

	avi_obj_cache := utils.NewAviObjCache()

	// TODO get API endpoint/username/password from configmap and track configmap
	// for changes and update rest client

	ctrlUsername := os.Getenv("CTRL_USERNAME")
	ctrlPassword := os.Getenv("CTRL_PASSWORD")
	ctrlIpAddress := os.Getenv("CTRL_IPADDRESS")
	if ctrlUsername == "" || ctrlPassword == "" || ctrlIpAddress == "" {
		utils.AviLog.Error.Panic("AVI controller information missing. Update them in kubernetes secret or via environment variables.")
	}
	avi_rest_client_pool, err := utils.NewAviRestClientPool(utils.NumWorkers,
		ctrlIpAddress, ctrlUsername, ctrlPassword)

	k8s_ep := k8s.NewK8sEp(avi_obj_cache, avi_rest_client_pool, informers)
	//k8s_ingr = kube.NewK8sIng(avi_obj_cache, avi_rest_client_pool, informers, k8s_ep)
	k8s_svc := k8s.NewK8sSvc(avi_obj_cache, avi_rest_client_pool, informers, k8s_ep)

	c := NewAviController(utils.NumWorkers, informers, kubeClient, k8s_ep, k8s_svc)

	c.Start(stopCh)

	c.Run(stopCh)
}

func init() {
	def_kube_config := os.Getenv("HOME") + "/.kube/config"
	flag.StringVar(&kubeconfig, "kubeconfig", def_kube_config, "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
