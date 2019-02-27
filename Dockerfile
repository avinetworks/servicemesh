FROM golang:latest AS build
ENV BUILD_PATH="github.com/avinetworks/servicemesh"
RUN mkdir -p $GOPATH/src/$BUILD_PATH
COPY . $GOPATH/src/$BUILD_PATH
WORKDIR $GOPATH/src/$BUILD_PATH

RUN GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -o $GOPATH/bin/servicemesh $BUILD_PATH

FROM alpine:latest
COPY --from=build /go/bin/servicemesh .
ENTRYPOINT ["./servicemesh"]
