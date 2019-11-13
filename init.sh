#!/bin/bash
current_dir=`pwd`
export GOPROXY="https://goproxy.io"

# copy proto
echo "cp -r proto '$GOPATH/src/proto'"
cp -r proto "$GOPATH/src/proto"

# install protoc-gen-go
GIT_TAG="v1.3.0" # change as needed
echo "go get -d -u github.com/golang/protobuf/protoc-gen-go"
go get -d -u github.com/golang/protobuf/protoc-gen-go

# checkout golang protobuf
echo "git -C "$(go env GOPATH)"/src/github.com/golang/protobuf checkout $GIT_TAG"
git -C "$(go env GOPATH)"/src/github.com/golang/protobuf checkout $GIT_TAG

# install protoc-gen-go
echo "go install github.com/golang/protobuf/protoc-gen-go"
go install github.com/golang/protobuf/protoc-gen-go

# install protoc-gen-swagger
echo ""$current_dir/protoc-gen-swagger" && go install"
cd "$current_dir/protoc-gen-swagger" && go install




