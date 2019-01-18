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
        "net"
        "net/url"
        "strings"
        "errors"
       )

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

func SvcMdataMapToObj(svc_mdata_map *map[string]interface{}, svc_mdata *ServiceMetadataObj) {
    for k, val := range *svc_mdata_map {
        switch k {
        case "crud_hash_key":
            crkhey, ok := val.(string)
            if ok {
                svc_mdata.CrudHashKey = crkhey
            } else {
                AviLog.Warning.Print("Incorrect type %T in svc_mdata_map %v", val, *svc_mdata_map)
            }
        }
    }
}

func AviUrlToObjType(aviurl string) (string, error) {
    url, err := url.Parse(aviurl)
    if err != nil {
        AviLog.Warning.Print("aviurl %v parse error", aviurl)
        return "", err
    }

    path := url.EscapedPath()

    elems := strings.Split(path, "/")
    return elems[2], nil
}

func RestRespArrToObjByType(rest_op *RestOp, obj_type string) ([]map[string]interface{}, error) {
    var resp_elems []map[string]interface{}

    resp_arr, ok := rest_op.Response.([]interface{})
    if !ok {
        AviLog.Warning.Printf("Response has unknown type %T", rest_op.Response)
        return nil, errors.New("Malformed response")
    }

    for _, resp_elem := range resp_arr {
        resp, ok := resp_elem.(map[string]interface{})
        if !ok {
            AviLog.Warning.Printf("Response has unknown type %T", resp_elem)
            continue
        }

        avi_url, ok := resp["url"].(string)
        if !ok {
            AviLog.Warning.Printf("url not present in response %v", resp)
            continue
        }

        avi_obj_type, err := AviUrlToObjType(avi_url)
        if err == nil && avi_obj_type == obj_type {
            resp_elems = append(resp_elems, resp)
        }
    }

    return resp_elems, nil
}
