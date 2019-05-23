# AVI Mesh Controller

##  Architecture

The AVI Mesh Controller is a layered collection of independent interoperable units that are used in conjunction to constitute a multi-cluster service mesh using AVI's Enterprise Grade LoadBalancers and Istio.

The controller ingests the Kubernetes API server object updates, Istio's Galley object updates and pumps them into a unified queue. It translates these objects into AVI's API model by interacting with the AVI Controller over HTTP. AVI Controller then programs the Service Engines deployed in your kubernetes cluster to program all the rules that are expressed in the form of Istio traffic rules. As a first phase of implementation, the AMC works in conjunction with the Istio Gateway functionality by replacing the functionality of Envoy at the edge with AVI's Enterprise grade Service Engines,  with a 0 touch object manipulation of the existing Istio Infrastructure. 

The flow can be visualized as follows:

![Alt text](HighLevelArch.jpg?raw=true "Title")


## Getting started

##  Build process

The project can be built in a couple of ways:

- [Go Build](#native-go-build)
- [Docker Build](#docker-build)


## Native Go Build

Steps:

    - Configure GOPATH in your machine.
    - mkdir -p $(GOPATH)/src/github/avinetworks/
    - cd $(GOPATH)/src/github/avinetworks/
    - git clone https://github.com/avinetworks/servicemesh
    - cd servicemesh
    - make build

This will generate a binary called: `$(GOPATH)/src/github/avinetworks/sevicemesh/servicemesh`

## Docker build

Steps:

    - Ensure you have docker 17.3 or above that supports docker multi-stage build.
    - git clone https://github.com/avinetworks/servicemesh
    - cd servicemesh
    - make docker

This will generate a docker image by name of `servicemesh:latest`


## Running tests

Steps:

    - git clone https://github.com/avinetworks/servicemesh
    - cd servicemesh
    - make test


## Running the service. 

#### Pre-requisites

The AMC service can be run standalone or inside the kubernetes cluster. If this service is being run outside of the Kubernetes cluster, one would need to expose the Galley server to become accessible over IP. For experiments, this can be achieved by exposing the Galley Service in Istio as NodePort service. Besides this, the following are a list of pre-requisites needed before we get started:

    - A Kubernetes cluster with Istio deployed with MCP services enabled.
    - A AVI controller accesible over IP: 
         -  The AVI controller should be configured with the Kubernetes/Openshift cloud. 
         -  The IPAMs for the North/South and East/West services should be configured.
         -  The service syncing for Backend/Frontend services should be disabled on the cloud.
 
 #### Running AMC outside the cluster

 Assuming you have built the project successfully by following the Build Process section. The following environment variables should be exposed if you are running this service outside the kubernetes cluster:
 
 `export ISTIO_ENABLED=True` - Enables the Istio MCP Client
 
 `export CTRL_USERNAME=<username>` - The AVI Controller username.
 
 `export CTRL_PASSWORD=<password>` - The AVI Controller password
 
 `export MCP_URL=<Galley_URL>/<GalleyNodePort>` - The endpoint to contact Galley.
 
 Post these steps - one can simply start the AMC service using: `./servicmesh`

#### Running AMC inside the cluster

 If you are running inside a kubernetes cluster, then one can use Galley exposed via the in cluster fqdn and deploy the AMC service as a POD using the kubernetes templates available that takes care of setting the above enviroment variables by default.
 

