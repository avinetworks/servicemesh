GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
BINARY_NAME_AMC=servicemesh-amc
BINARY_NAME_CLOUD=servicemesh-cloud
REL_PATH_AMC=github.com/avinetworks/servicemesh/cmd/amc-main
REL_PATH_CLOUD=github.com/avinetworks/servicemesh/cmd/cloud-main

.PHONY:all
all: build docker

.PHONY: build
build: 
		$(GOBUILD) -o bin/$(BINARY_NAME_AMC) $(REL_PATH_AMC)
		$(GOBUILD) -o bin/$(BINARY_NAME_CLOUD) $(REL_PATH_CLOUD)

.PHONY: clean
clean: 
		$(GOCLEAN)
		rm -f $(BINARY_NAME)

.PHONY: deps
deps:
	dep ensure -v

.PHONY: docker
docker:
	docker build -t $(BINARY_NAME_AMC):latest -f Dockerfile.amc .
	docker build -t $(BINARY_NAME_CLOUD):latest -f Dockerfile.cloud .

.PHONY: test
test:
	go test -v ./amc/pkg/istio/mcp
	go test -v ./amc/pkg/istio/objects
	go test -v ./amc/pkg/istio/mcp/mcptests
	go test -v ./amc//pkg/istio/nodes

