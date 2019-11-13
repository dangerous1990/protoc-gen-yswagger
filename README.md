- [what's this?](#what-s-this-)
- [why?](#why-)
- [how to use](#how-to-use)
  * [安装protoc](#--protoc)
  * [执行init.sh](#--initsh)
  * [sample](#sample)

# what's this?
基于bilibili [kratos](https://github.com/bilibili/kratos)的protoc-gen-bswagger修改而来，通过proto文件生成restful的swagger.json文档

# why?
对于生成的swagger.json有一些定制内容
# how to use
## 首先如果没有过protoc需要安装protoc 
https://github.com/protocolbuffers/protobuf
##
```
git clone https://github.com/dangerous1990/protoc-plugins.git
```
## 执行init.sh
```
./init.sh
``` 
## sample
- hello.proto
```
syntax = "proto3";
import "google/api/annotations.proto";

service HelloWorld {
    // hello
    rpc Hello (HelloReq) returns (HelloReply) {
        option (google.api.http) = {
            post:"/hello"
        };
    };
}
message HelloReq{
}
message HelloReply{
    string name =1;
}
```
- 使用protoc生成swagger.json
```
protoc -I"${GOPATH}/src/proto" -Iexample example/hello.proto  --swagger_out=example

```

