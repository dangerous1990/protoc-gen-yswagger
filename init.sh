#!/bin/bash
export GOPROXY="https://goproxy.io"

# copy proto
echo "cp -r proto '$GOPATH/src/proto'"
cp -r proto "$GOPATH/src/proto"

# install protoc-gen-go
echo "go get -u github.com/golang/protobuf/protoc-gen-go"
go get -u github.com/golang/protobuf/protoc-gen-go

# install protoc-gen-go
echo "go install github.com/golang/protobuf/protoc-gen-go"
go install github.com/golang/protobuf/protoc-gen-go

# install protoc-gen-yswagger
echo "go get -u github.com/dangerouslm1990/protoc-gen-yswagger"
go get -u github.com/dangerouslm1990/protoc-gen-yswagger





