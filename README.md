# AVI Servicemesh

##  Architecture

##  Project structure

The project has a specific rule with respect to packaging. The dependency should be followed by every developer in order to maintain sanity.

- [servicemesh/pkg](https://github.com/avinetworks/servicemesh/pkg) -- This contains pure library go code. This also hosts code that can be imported by other places. Ensure none of these packages are importing anything outside of this directory.

- [servicemesh/aviobjects](https://github.com/avinetworks/servicemesh/aviobjects) -- This package should contain all information related to AVI objects.

- [servicemesh/k8s_tmpl](https://github.com/avinetworks/servicemesh/k8s_tmpl) -- This folder contains kubernetes related artifacts. These are not part of the build process right now and should be edited before use as per the deployment requirement.


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
    - make deps build

This will generate a binary called: `$(GOPATH)/src/github/avinetworks/sevicemesh/servicemesh`

## Docker build

Steps:

    - Ensure you have docker 17.3 or above that supports docker multi-stage build.
    - git clone https://github.com/avinetworks/servicemesh
    - cd servicemesh
    - make docker

This will generate a docker image by name of `servicemesh:latest`

## How to contribute

## Running tests
