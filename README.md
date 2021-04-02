- [what's this?](#what-s-this-)
- [why?](#why-)
- [how to use](#how-to-use)
  * [1. 首先如果没有安装过protoc需要安装protoc](#1----------protoc----protoc)
  * [2. clone当前项目](#2-clone----)
  * [3. 执行init.sh](#3---initsh)
  * [4. sample](#4-sample)
  * [5. 支持的http的参数转化](#5--支持的http的参数转化)

# what's this?
基于bilibili [kratos](https://github.com/bilibili/kratos)的protoc-gen-bswagger修改而来，通过proto文件生成restful的swagger.json文档

# why?
对于生成的swagger.json有一些定制内容
# how to use
## 1. 首先如果没有安装过protoc需要安装protoc 
https://github.com/protocolbuffers/protobuf
## 2. clone当前项目
```
git clone https://github.com/dangerous1990/protoc-gen-yswagger.git
```
## 3. 执行init.sh
进入protoc-plugins项目执行

```
./init.sh
``` 
## 4. sample
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
protoc -I"${GOPATH}/src/proto" -Iexample example/hello.proto  --yswagger_out=example
# 支持覆盖requestID
protoc -I"${GOPATH}/src/proto" -Iexample example/hello.proto  --yswagger_out=example --yswagger_opt='requestID=request_id'

```

## 5. 支持的http的参数转化

|        | post | get  | delete | put |
|  ----  | ---- | ---- |---- | ---- |
| path  | ✅  | ✅ | ✅ | ✅  |
| query | ❌  | ✅ | ✅ | ✅  |
| form  | ✅  | ❌ | ❌ | ❌  |
| json  | ✅  | ❌ | ❌ | ✅  |

* path参数
```
string id = 1 [(gogoproto.moretags) = 'params:"id"'];
```
* query参数
```
string key1 = 2 [(gogoproto.moretags) = 'query:"key"'];
```
* form参数
```
string key1 = 2 [(gogoproto.moretags) = 'form:"key"'];
```
* json参数
```
string key1 = 2 [(gogoproto.jsontag) = 'key'];
```