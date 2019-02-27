FROM golang:latest AS build
ENV BUILD_PATH="github.com/avinetworks/servicemesh"
RUN mkdir -p $GOPATH/src/$BUILD_PATH
COPY . $GOPATH/src/$BUILD_PATH
WORKDIR $GOPATH/src/$BUILD_PATH

RUN GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -o $GOPATH/bin/servicemesh $BUILD_PATH

FROM scratch
COPY --from=build $GOPATH/bin/servicemesh $GOPATH/bin/servicemesh
ENTRYPOINT ["/go/bin/servicemesh"]
