package kube

import (
	"github.com/avi_k8s_controller/pkg/utils"
)

type K8sIngress struct {
	avi_obj_cache        *utils.AviObjCache
	avi_rest_client_pool *AviRestClientPool
	informers            *Informers
	k8s_ep               *K8sEp
}

func NewK8sIngress(avi_obj_cache *AviObjCache, avi_rest_client_pool *AviRestClientPool,
	inf *Informers, k8s_ep *K8sEp) *K8sSvc {
	s := K8sSvc{}
	s.avi_obj_cache = avi_obj_cache
	s.avi_rest_client_pool = avi_rest_client_pool
	s.informers = inf
	s.k8s_ep = k8s_ep
	return &s
}
