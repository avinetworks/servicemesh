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

package objects

import (
	"sync"
)

//This package gives relationship APIs to manage a kubernetes service object.

var secretlisterinstance *SecretLister
var secretonce sync.Once

func SharedSecretLister() *SecretLister {
	secretonce.Do(func() {
		secretlisterinstance = &SecretLister{}
		secretlisterinstance.secretGwStore = NewObjectStore()
	})
	return secretlisterinstance
}

type SecretLister struct {
	secretGwStore *ObjectStore
}

type SecretNSCache struct {
	namespace       string
	secretGwobjects *ObjectMapStore
}

func (v *SecretLister) Secret(ns string) *SecretNSCache {
	namespacedsecretGwObjs := SharedSecretLister().secretGwStore.GetNSStore(ns)
	return &SecretNSCache{namespace: ns, secretGwobjects: namespacedsecretGwObjs}
}

func (v *SecretNSCache) GetSecretToGW(secretName string) (bool, []string) {
	// Need checks if it's found or not?
	found, gwNames := v.secretGwobjects.Get(secretName)
	if !found {
		return false, nil
	}
	return true, gwNames.([]string)
}

func (v *SecretNSCache) DeleteSecretToGWMapping(secretName string) bool {
	// Need checks if it's found or not?
	success := v.secretGwobjects.Delete(secretName)
	return success
}

func (v *SecretNSCache) UpdateSecretToGwMapping(secretName string, gwList []string) {
	v.secretGwobjects.AddOrUpdate(secretName, gwList)
}
