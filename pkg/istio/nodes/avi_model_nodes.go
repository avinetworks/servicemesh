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

package nodes

import (
	"fmt"
	"sort"

	avimodels "github.com/avinetworks/sdk/go/models"
	"github.com/avinetworks/servicemesh/pkg/utils"
)

type AviVsNode struct {
	Name               string
	Tenant             string
	ServiceMetadata    ServiceMetadataObj
	ApplicationProfile string
	NetworkProfile     string
	PortProto          []AviPortHostProtocol // for listeners
	//PoolGroupMap       map[AviPortProtocol]string // for mapping listener to Pools
	DefaultPool      string
	EastWest         bool
	CloudConfigCksum uint32
	DefaultPoolGroup string
	// This field will detect if the HTTP policy set rules have changed.
	HTTPChecksum uint32
	SNIParent    bool
}

type AviVsTLSNode struct {
	VHParentName     string
	VHDomainNames    []string
	CloudConfigCksum uint32
	Name             string
	TLSKeyCert       []*AviTLSKeyCertNode
	*AviVsNode
}

func (v *AviVsTLSNode) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.CloudConfigCksum
}

func (v *AviVsTLSNode) CalculateCheckSum() {
	// A sum of fields for this VS.
	checksum := utils.Hash(v.VHParentName) + utils.Hash(utils.Stringify(v.VHDomainNames))
	v.CloudConfigCksum = checksum
}

func (v *AviVsNode) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.CloudConfigCksum
}

func (v *AviVsNode) CalculateCheckSum() {
	// A sum of fields for this VS.
	checksum := utils.Hash(v.ApplicationProfile) + utils.Hash(v.NetworkProfile) + utils.Hash(utils.Stringify(v.PortProto)) + utils.Hash(fmt.Sprint(v.HTTPChecksum))
	v.CloudConfigCksum = checksum
}

type AviTLSKeyCertNode struct {
	Name             string
	Tenant           string
	CloudConfigCksum uint32
	Key              []byte
	Cert             []byte
	Port             int32
}

func (v *AviTLSKeyCertNode) CalculateCheckSum() {
	// A sum of fields for this SSL cert.
	checksum := utils.Hash(string(v.Key)) + utils.Hash(string(v.Cert))
	v.CloudConfigCksum = checksum
}

func (v *AviTLSKeyCertNode) GetCheckSum() uint32 {
	return v.CloudConfigCksum
}

type AviPoolGroupNode struct {
	Name             string
	Tenant           string
	ServiceMetadata  ServiceMetadataObj
	CloudConfigCksum uint32
	RuleChecksum     uint32
	Members          []*avimodels.PoolGroupMember
	MatchList        []MatchCriteria
}

func (v *AviPoolGroupNode) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.CloudConfigCksum
}

func (v *AviPoolGroupNode) CalculateCheckSum() {
	// A sum of fields for this VS.
	checksum := utils.Hash(utils.Stringify(v.Members)) + utils.Hash(utils.Stringify(v.MatchList))
	v.CloudConfigCksum = checksum
}

type AviPoolNode struct {
	Name             string
	Tenant           string
	ServiceMetadata  ServiceMetadataObj
	CloudConfigCksum uint32
	Port             int32
	PortName         string
	Servers          []AviPoolMetaServer
	Protocol         string
	LbAlgorithm      string
}

func (v *AviPoolNode) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.CloudConfigCksum
}

func (v *AviPoolNode) CalculateCheckSum() {
	// A sum of fields for this VS.
	checksum := utils.Hash(v.Protocol) + utils.Hash(fmt.Sprint(v.Port)) + utils.Hash(v.PortName) + utils.Hash(utils.Stringify(v.Servers)) + utils.Hash(utils.Stringify(v.LbAlgorithm))
	v.CloudConfigCksum = checksum
}

type AviPoolMetaServer struct {
	Ip         avimodels.IPAddr
	ServerNode string
}

type AviPortHostProtocol struct {
	Port     int32
	Protocol string
	Hosts    []string
	Secret   string
}

type AviPortStrProtocol struct {
	Port     string // Can be Port name or int32 string
	Protocol string
}

type AviHostPathPortPoolPG struct {
	Host          []string
	Path          []string
	Port          uint32
	Pool          string
	PoolGroup     string
	MatchCriteria string
}

type AviHttpPolicySetNode struct {
	Name             string
	Tenant           string
	CloudConfigCksum uint32
	HppMap           []AviHostPathPortPoolPG
}

func (v *AviHttpPolicySetNode) GetCheckSum() uint32 {
	// Calculate checksum and return
	return v.CloudConfigCksum
}

func (v *AviHttpPolicySetNode) CalculateCheckSum() {
	// A sum of fields for this VS.
	var checksum uint32
	for _, hpp := range v.HppMap {
		sort.Strings(hpp.Host)
		sort.Strings(hpp.Path)
		checksum = checksum + utils.Hash(utils.Stringify(hpp))
	}
	utils.AviLog.Info.Printf("The HTTP rules during checksum calculation is: %s with checksum: %v", utils.Stringify(v.HppMap), checksum)
	v.CloudConfigCksum = checksum
}

type MatchCriteria struct {
	Name          string
	Criteria      string
	PathSpecifier isRouteMatch_PathSpecifier
	//Scheme        string
	//Method string
	//Headers map[string]string
}

type isRouteMatch_PathSpecifier interface {
	isRouteMatch_PathSpecifier()
}

type RouteMatch_Prefix struct {
	Prefix string
}
type RouteMatch_Path struct {
	Path string
}
type RouteMatch_Regex struct {
	Regex string
}

func (m *MatchCriteria) GetPath() string {
	if x, ok := m.GetPathSpecifier().(*RouteMatch_Path); ok {
		return x.Path
	}
	return ""
}

func (m *MatchCriteria) GetPathSpecifier() isRouteMatch_PathSpecifier {
	if m != nil {
		return m.PathSpecifier
	}
	return nil
}

func (m *MatchCriteria) GetRegex() string {
	if x, ok := m.GetPathSpecifier().(*RouteMatch_Regex); ok {
		return x.Regex
	}
	return ""
}

func (m *MatchCriteria) GetPrefix() string {
	if x, ok := m.GetPathSpecifier().(*RouteMatch_Prefix); ok {
		return x.Prefix
	}
	return ""
}

func (*RouteMatch_Prefix) isRouteMatch_PathSpecifier() {}
func (*RouteMatch_Path) isRouteMatch_PathSpecifier()   {}
func (*RouteMatch_Regex) isRouteMatch_PathSpecifier()  {}

type ServiceMetadataObj struct {
	CrudHashKey string `json:"crud_hash_key"`
}
