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
	"fmt"

	avimodels "github.com/avinetworks/sdk/go/models"
	coreinformers "k8s.io/client-go/informers/core/v1"
	extinformers "k8s.io/client-go/informers/extensions/v1beta1"
)

type EvType string

const NumWorkers uint32 = 2

const (
	CreateEv EvType = "CREATE"
	UpdateEv EvType = "UPDATE"
	DeleteEv EvType = "DELETE"
)

const (
	OSHIFT_K8S_CLOUD_CONNECTOR string = "oshift-k8s-cloud-connector"
)

const (
	AVI_DEFAULT_TCP_HM string = "System-TCP"
	AVI_DEFAULT_UDP_HM string = "System-UDP"
)

type Informers struct {
	ServiceInformer coreinformers.ServiceInformer
	EpInformer      coreinformers.EndpointsInformer
	IngInformer     extinformers.IngressInformer
}

type AviRestObjMacro struct {
	ModelName string      `json:"model_name"`
	Data      interface{} `json:"data"`
}

type RestMethod string

const (
	RestPost   RestMethod = "POST"
	RestPut    RestMethod = "PUT"
	RestDelete RestMethod = "DELETE"
	RestPatch  RestMethod = "PATCH"
	RestGet    RestMethod = "GET"
)

type RestOp struct {
	Path     string
	Method   RestMethod
	Obj      interface{}
	Tenant   string
	PatchOp  string
	Response interface{}
	Err      error
	Model    string
	Version  string
}

type ServiceMetadataObj struct {
	CrudHashKey string `json:"crud_hash_key"`
}

type NamespaceName struct {
	Namespace string
	Name      string
}

/*
 * Meta data passed to Avi Rest Crud by Ep Crud
 */

type AviPoolMetaServer struct {
	Ip         avimodels.IPAddr
	ServerNode string
}

type K8sAviPoolMeta struct {
	Name             string
	Tenant           string
	ServiceMetadata  ServiceMetadataObj
	CloudConfigCksum string
	Port             int32
	Servers          []AviPoolMetaServer
	Protocol         string
}

type AviPortProtocol struct {
	Port     int32
	Protocol string
}

type AviPortStrProtocol struct {
	Port     string // Can be Port name or int32 string
	Protocol string
}

type AviHostPathPortPoolPG struct {
	Host      string
	Path      string
	Port      uint32
	Pool      string
	PoolGroup string
}

type K8sAviVsMeta struct {
	Name               string
	Tenant             string
	ServiceMetadata    ServiceMetadataObj
	ApplicationProfile string
	NetworkProfile     string
	PortProto          []AviPortProtocol          // for listeners
	PoolMap            map[AviPortProtocol]string // for mapping listener to Pools
	DefaultPool        string
	EastWest           bool
	CloudConfigCksum   string
}

/*
 * Obj cache
 */

type AviPoolCache struct {
	Name             string
	Tenant           string
	Uuid             string
	LbAlgorithm      string
	ServiceMetadata  ServiceMetadataObj
	CloudConfigCksum string
}

type AviVsCache struct {
	Name             string
	Tenant           string
	Uuid             string
	ServiceMetadata  ServiceMetadataObj
	CloudConfigCksum string
}

type AviHttpPolicySetMeta struct {
	Name             string
	Tenant           string
	CloudConfigCksum string
	HppMap           []AviHostPathPortPoolPG
}

type SkipSyncError struct {
	Msg string
}

type WebSyncError struct {
	err       error
	operation string
}

func (e *WebSyncError) Error() string  { return fmt.Sprintf("Error during %s: %v", e.operation, e.err) }
func (e *SkipSyncError) Error() string { return e.Msg }
