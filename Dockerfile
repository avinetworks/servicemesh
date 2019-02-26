FROM golang:latest AS build
ENV BUILD_PATH="github.com/avinetworks/servicemesh"
RUN mkdir -p $GOPATH/src/$BUILD_PATH
COPY . $GOPATH/src/$BUILD_PATH
WORKDIR $GOPATH/src/$BUILD_PATH
RUN go get $BUILD_PATH; exit 0
RUN rm -rf $GOPATH/src/istio.io/istio/vendor/github.com/gogo/googleapis/google/rpc
RUN rm -rf $GOPATH/src/istio.io/istio/vendor/github.com/gogo/protobuf/types
RUN rm -rf $GOPATH/src/istio.io/istio/vendor/istio.io/api/mcp/v1alpha1

RUN GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -o $GOPATH/bin/servicemesh $BUILD_PATH

FROM scratch
COPY --from=build $GOPATH/bin/servicemesh $GOPATH/bin/servicemesh
ENTRYPOINT ["/go/bin/servicemesh"]