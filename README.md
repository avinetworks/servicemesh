# AVI Mesh Controller

##  Architecture

The AVI Mesh Controller is a layered collection of independent interoperable units that are used in conjunction to constitute a multi-cluster service mesh using AVI's Enterprise Grade LoadBalancers and Istio.

The controller ingests the Kubernetes API server object updates, Istio's Galley object updates and pumps them into a unified queue. In order to ensure completness of information to formulate an AVI REST API call, every kubernetes or Istio resource is traced back to a Gateway resource. For example, for a kubernetes update of an endpoint, AMC traces to the gateways that are impacted by this change. If the gateway relationship cannot be established, then it's assumed that a gateway resource is not directly or indirectly related to this resource update and that leads to discarding the update for any further action. However, if the gateway can be traced, then a complete walk of the relationships is performed as follows:

    Gateways --> VirtualServices --> Services (Applicable Destination Rules) --> Endpoints --> Servers (based on pod labels)

A bunch of AVI object nodes are generated corresponding to these Istio/K8s objects in the "Object Graph Transformation" layer and these are published to another queue that is eventually consumed by the API execution layer. These AVI object nodes are the `intended` AVI objects that are compared with the AMC cache to arrive at a decision of either CREATE, UPDATE or DELETE a corresponding AVI object via the API execution layer. The API execution layer calls the appropriate REST APIs of the AVI controller to create the corresponding objects in AVI. 

As a consequence to this, the AVI controller  programs the Service Engines deployed in your kubernetes cluster to program all the rules that are expressed in the form of Istio traffic rules. As a first phase of implementation, the AMC works in conjunction with the Istio Gateway functionality by replacing the functionality of Envoy at the edge with AVI's Enterprise grade Service Engines, with a 0 touch object manipulation of the existing Istio Infrastructure. 

The AMC flow can be visualized as follows:

![Alt text](Arch_AVI_Layers.png?raw=true "Title")

The object translation from Istio to AVI roughly looks like the below diagram (for HTTP based routes):

![Alt text](AVI_Object_Transform.png?raw=true "Title")
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
    - A AVI controller accesible over IP.
 
 #### Running AMC outside the cluster

 Assuming you have built the project successfully by following the Build Process section. The following environment variables should be exposed if you are running this service outside the kubernetes cluster:
 
 `export ISTIO_ENABLED=True` - Enables the Istio MCP Client
 
 `export CTRL_USERNAME=<username>` - The AVI Controller username.
 
 `export CTRL_PASSWORD=<password>` - The AVI Controller password
 
 `export MCP_URL=<Galley_URL>:<GalleyNodePort>` - The endpoint to contact Galley.
 
 `export CTRL_IPADDRESS=<AVI_API_SERVER_ADDR>` - The AVI controller API endpoint with port if applicable.
 
 `export STATIC_RANGE_START=<N/S IP Range start>` - The start address of your n/s IPAM range.
 
 `export STATIC_RANGE_END=<N/S IP Range end>` - The end address of your n/s IPAM range.
 
 `export CIDR=<ip>/<mask>` - The CIDR information of your n/s IPAM.
 
 `export CLOUD_NAME=<String>` - The name of your kubernetes cloud in AVI.
 
 `export MASTER_NODES=<ip:port>` - The IP:port information of the Kubernetes API server.
 
 `export SERVICE_TOKEN=<string>` - The service token to authenticate with the kube API server. Must have admin privileges.
 
  Execute the following commands in order:
    -  ./servicemesh-cloud
    -  ./servicemesh-amc

#### Deploy AMC using Helm

 If you are running inside a kubernetes cluster, then a lot of automation is provided out of the box using helm charts. Please follow the below steps to run it:
 
 - Clone the code.
 - Ensure tiller/helm is installed in your cluster.
 - cd servicemesh/helm/amc
 - Edit the `values.yaml` file and add appropriate values.
 - cd ../helm
 - helm install ./amc
 - You may want to build the docker images using multi-stage docker builds (requires docker version > 17) by simply using `make docker` from the root directory of the project.

After you execute the above helm command successfully, ensure that your AMC deployment is running as expected. Following changes can be observed:

   -   A new cloud with the name provided in the values.yaml should be created in your AVI Controller.
   -   A service engine should be placed on your kubernetes cluster running in the form of a DaemonSet.
   -   All appropriate network/dns should be configured as mentioned in the values.yaml file and wired up to this cloud.
   -   The istio resources would be synced to AVI in the form of AVI data model objects.
 
