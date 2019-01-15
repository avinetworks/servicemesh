GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
BINARY_NAME=avi_k8s_controller_go
REL_PATH=github.com/avinetworks/avi_k8s_controller

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
		$(GOGET) -v $(REL_PATH)

.PHONY: docker
docker:
	docker build -t $(BINARY_NAME):latest -f Dockerfile .

