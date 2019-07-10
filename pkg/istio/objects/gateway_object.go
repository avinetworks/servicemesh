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

//This module should give a wrapper over underlying objects to give easy API access for Gateway.

package objects

import (
	"sync"

	"github.com/avinetworks/servicemesh/pkg/utils"
	networking "istio.io/api/networking/v1alpha3"
)

var gwlisterinstance *GatewayLister
var gwonce sync.Once

func SharedGatewayLister() *GatewayLister {
	gwonce.Do(func() {
		GWStore := NewObjectStore()
		gwvsStore := NewObjectStore()
		gwlisterinstance = &GatewayLister{}
		gwlisterinstance.gwvsstore = gwvsStore
		gwlisterinstance.gwstore = GWStore
		gwlisterinstance.gwsecretstore = NewObjectStore()
	})
	return gwlisterinstance
}

type IstioGateway interface {
	Gateway(ns string) GatewayNameSpaceIntf
	//List() *[]ObjectStore
}

type GatewayLister struct {
	gwstore       *ObjectStore
	gwvsstore     *ObjectStore
	gwsecretstore *ObjectStore
}

func (v *GatewayLister) Gateway(ns string) *GatewayNSCache {
	nsGwObjects := v.gwstore.GetNSStore(ns)
	nsGwVsObjects := v.gwvsstore.GetNSStore(ns)
	gwToSecretInstance := v.gwsecretstore.GetNSStore(ns)
	secretInstance := SharedSecretLister()
	return &GatewayNSCache{namespace: ns, gwobjects: nsGwObjects, gwvsobjects: nsGwVsObjects, gwToSecretInstance: gwToSecretInstance, secretInstance: secretInstance}
}

func (v *GatewayLister) GetAllGateways() map[string]map[string]string {
	// This method should return a map that looks like this: {ns: [obj1, obj2]}
	// This is particularly useful if we want to know what are the vs names
	// present in a namespace without affecting the actual store objects.
	allNamespaces := v.gwstore.GetAllNamespaces()
	allGateways := make(map[string]map[string]string)
	if len(allNamespaces) != 0 {
		// Iterate over each namespace and formulate the map
		for _, ns := range allNamespaces {
			allGateways[ns] = v.Gateway(ns).GetAllGatewayNameVers()
		}
	}
	return allGateways

}

type GatewayNameSpaceIntf interface {
	Get(name string) (bool, *IstioObject)
	Update(obj IstioObject) bool
	List() map[string]*IstioObject
	Delete(name string) bool
}

type GatewayNSCache struct {
	namespace          string
	gwobjects          *ObjectMapStore
	gwvsobjects        *ObjectMapStore
	gwToSecretInstance *ObjectMapStore
	secretInstance     *SecretLister
}

func (v *GatewayNSCache) GetAllGatewayNameVers() map[string]string {
	// Obtain the object for this Gateway
	allObjects := v.gwobjects.GetAllObjectNames()
	objVersionsMap := make(map[string]string)
	// Now let's parse the object names and their corresponding resourceversions in a Map
	for _, obj := range allObjects {
		objVersionsMap[obj.(*IstioObject).ConfigMeta.Name] = obj.(*IstioObject).ConfigMeta.ResourceVersion
	}
	return objVersionsMap
}

func (v *GatewayNSCache) Get(name string) (bool, *IstioObject) {
	found, obj := v.gwobjects.Get(name)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.(*IstioObject)
	}
}

func (v *GatewayNSCache) GetVSMapping(gwname string) (bool, []string) {
	found, obj := v.gwvsobjects.Get(gwname)
	if !found {
		// Do error wrapping here
		return false, nil
	} else {
		return true, obj.([]string)
	}
}

func (v *GatewayNSCache) DeleteGwVSMapping(gwname string) bool {
	return v.gwvsobjects.Delete(gwname)
}

func (v *GatewayNSCache) Update(obj *IstioObject) {
	v.gwobjects.AddOrUpdate(obj.Name, obj)
}

func (v *GatewayNSCache) UpdateGWVSMapping(gwName string, vsList []string) {
	v.gwvsobjects.AddOrUpdate(gwName, vsList)
}

func (v *GatewayNSCache) Delete(name string) bool {
	return v.gwobjects.Delete(name)
}

func (v *GatewayNSCache) List() map[string]*IstioObject {
	// TODO (sudswas): Let's check if we can abstract out the store objects
	// completely. There's still a possibility that if we pass the references
	// we maybe allowing upper layers to modify the object that would directly
	// impact the store objects.
	convertedMap := make(map[string]*IstioObject)
	// Change the empty interface to IstioObject. Avoid Duck Typing.
	for key, value := range v.gwobjects.ObjectMap {
		convertedMap[key] = value.(*IstioObject)
	}
	return convertedMap
}

// --> All Secrets relationships follow

func (v *GatewayNSCache) UpdateSecretGWRefs(obj *IstioObject) {

	secrets := v.GetSecretFromGW(obj)
	v.gwToSecretInstance.AddOrUpdate(obj.ConfigMeta.Name, secrets)
	utils.AviLog.Info.Printf("The Secrets associated with GW: %s is %s ", obj.ConfigMeta.Name, secrets)
	for _, secret := range secrets {
		_, gwList := v.secretInstance.Secret(obj.ConfigMeta.Namespace).GetSecretToGW(secret)
		if !utils.HasElem(gwList, obj.ConfigMeta.Name) {
			gwList = append(gwList, obj.ConfigMeta.Name)
			v.secretInstance.Secret(obj.ConfigMeta.Namespace).UpdateSecretToGwMapping(secret, gwList)
		}
	}
}

func (v *GatewayNSCache) GetSecretsForGateway(gwName string) []string {
	found, secrets := v.gwToSecretInstance.Get(gwName)
	if found {
		return secrets.([]string)
	} else {
		return nil
	}
}

func (v *GatewayNSCache) RemoveSecretGWRefs(gwName string, gwNs string) {
	_, secrets := v.gwToSecretInstance.Get(gwName)
	utils.AviLog.Info.Printf("The Secrets associated with GW during DELETE: %s is %s ", gwName, secrets)
	for _, secret := range secrets.([]string) {
		_, gwList := v.secretInstance.Secret(gwNs).GetSecretToGW(secret)
		if utils.HasElem(gwList, gwName) {
			gwList = Remove(gwList, gwName)
			v.secretInstance.Secret(gwNs).UpdateSecretToGwMapping(secret, gwList)
		}
	}
}

func (v *GatewayNSCache) DeleteGwSecretMapping(gwName string) {
	// First update the SVC to VS relationship
	v.gwToSecretInstance.Delete(gwName)
}

func (v *GatewayNSCache) GetSecretFromGW(obj *IstioObject) []string {
	// The secret is present in the tls section
	/*
		apiVersion: networking.istio.io/v1alpha3
		kind: Gateway
		metadata:
		  name: mygateway
		spec:
		  selector:
		    istio: ingressgateway # use istio default ingress gateway
		  servers:
		  - port:
		      number: 443
		      name: https
		      protocol: HTTPS
		    tls:
		      mode: SIMPLE
		      credentialName: "httpbin-credential" # must be the same as secret
		    hosts:
			- "httpbin.example.com" */

	gwObj, ok := obj.Spec.(*networking.Gateway)
	if !ok {
		// This is not the right object to cast to VirtualService return error.
		utils.AviLog.Warning.Printf("Wrong object passed. Expecting a Gateway object %v", gwObj)
		return nil
	}
	var secretName []string
	for _, server := range gwObj.Servers {
		if server.Tls != nil {
			if server.Tls.Mode == networking.Server_TLSOptions_SIMPLE {
				secretName = append(secretName, server.Tls.CredentialName)
			}
		}
	}
	return secretName
}
