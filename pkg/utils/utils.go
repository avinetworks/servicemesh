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
	"hash/fnv"
	"math/rand"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var CtrlVersion string

func init() {
	//Setting the package-wide version
	CtrlVersion = os.Getenv("CTRL_VERSION")
	if CtrlVersion == "" {
		CtrlVersion = "18.2.2"
	}
}

func IsV4(addr string) bool {
	ip := net.ParseIP(addr)
	v4 := ip.To4()
	if v4 == nil {
		return false
	} else {
		return true
	}
}

/*
 * Port name is either "http" or "http-suffix"
 * Following Istio named port convention
 * https://istio.io/docs/setup/kubernetes/spec-requirements/
 * TODO: Define matching ports in configmap and make it configurable
 */

func IsSvcHttp(svc_name string, port int32) bool {
	if svc_name == "http" {
		return true
	} else if strings.HasPrefix(svc_name, "http-") {
		return true
	} else if (port == 80) || (port == 443) || (port == 8080) || (port == 8443) {
		return true
	} else {
		return false
	}
}

func AviUrlToObjType(aviurl string) (string, error) {
	url, err := url.Parse(aviurl)
	if err != nil {
		AviLog.Warning.Printf("aviurl %v parse error", aviurl)
		return "", err
	}

	path := url.EscapedPath()

	elems := strings.Split(path, "/")
	return elems[2], nil
}

/*
 * Hash key to pick workqueue & GoRoutine. Hash needs to ensure that K8S
 * objects that map to the same Avi objects hash to the same wq. E.g.
 * Routes that share the same "host" should hash to the same wq, so "host"
 * is the hash key for Routes. For objects like Service, it can be ns:name
 */

func CrudHashKey(obj_type string, obj interface{}) string {
	var ns, name string
	switch obj_type {
	case "Endpoints":
		ep := obj.(*corev1.Endpoints)
		ns = ep.Namespace
		name = ep.Name
	case "Service":
		svc := obj.(*corev1.Service)
		ns = svc.Namespace
		name = svc.Name
	case "Ingress":
		ing := obj.(*extensions.Ingress)
		ns = ing.Namespace
		name = ing.Name
	default:
		AviLog.Error.Printf("Unknown obj_type %s obj %v", obj_type, obj)
		return ":"
	}
	return ns + ":" + name
}

func Bkt(key string, num_workers uint32) uint32 {
	bkt := Hash(key) & (num_workers - 1)
	return bkt
}

// DeepCopy deepcopies a to b using json marshaling
func DeepCopy(a, b interface{}) {
	byt, _ := json.Marshal(a)
	json.Unmarshal(byt, b)
}

func Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func RandomSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

var informer sync.Once
var informerInstance *Informers

func NewInformers(cs *kubernetes.Clientset) *Informers {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(cs, time.Second*30)
	informer.Do(func() {
		informerInstance = &Informers{
			ServiceInformer: kubeInformerFactory.Core().V1().Services(),
			EpInformer:      kubeInformerFactory.Core().V1().Endpoints(),
			PodInformer:     kubeInformerFactory.Core().V1().Pods(),
			SecretInformer:  kubeInformerFactory.Core().V1().Secrets(),
		}
	})
	return informerInstance
}

func GetInformers() *Informers {
	if informerInstance == nil {
		AviLog.Error.Fatal("Cannot retrieve the informers since it's not initialized yet.")
		return nil
	}
	return informerInstance
}

func Stringify(serialize interface{}) string {
	json_marshalled, _ := json.Marshal(serialize)
	return string(json_marshalled)
}

func ExtractGatewayNamespace(key string) (string, string) {
	segments := strings.Split(key, "/")
	if len(segments) == 2 {
		return segments[0], segments[1]
	}
	return "", ""
}

func HasElem(s interface{}, elem interface{}) bool {
	arrV := reflect.ValueOf(s)

	if arrV.Kind() == reflect.Slice {
		for i := 0; i < arrV.Len(); i++ {
			// XXX - panics if slice element points to an unexported struct field
			// see https://golang.org/pkg/reflect/#Value.Interface
			if arrV.Index(i).Interface() == elem {
				return true
			}
		}
	}

	return false
}
