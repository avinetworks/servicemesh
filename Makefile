GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
BINARY_NAME=servicemesh
REL_PATH=github.com/avinetworks/servicemesh

.PHONY:all
all: deps build docker

.PHONY: build
build: 
		$(GOBUILD) -o $(BINARY_NAME) $(REL_PATH)

.PHONY: clean
clean: 
		$(GOCLEAN)
		rm -f $(BINARY_NAME)

.PHONY: deps
deps:
		-$(GOGET) -v $(REL_PATH)
		rm -rf $(GOPATH)/src/istio.io/istio/vendor/github.com/gogo/googleapis/google/rpc
		rm -rf $(GOPATH)/src/istio.io/istio/vendor/github.com/gogo/protobuf/types
		rm -rf $(GOPATH)/src/istio.io/istio/vendor/istio.io/api/mcp/v1alpha1

.PHONY: docker
docker:
	docker build -t $(BINARY_NAME):latest -f Dockerfile .
