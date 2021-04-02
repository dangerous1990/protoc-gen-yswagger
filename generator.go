package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/tool/protobuf/pkg/extensions/gogoproto"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/gen"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/generator"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/naming"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/tag"
	"github.com/go-kratos/kratos/tool/protobuf/pkg/typemap"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

type swaggerGen struct {
	generator.Base
	// defsMap will fill into swagger's definitions
	// key is full qualified proto name
	defsMap map[string]*typemap.MessageDefinition
}

// NewSwaggerGenerator a swagger generator
func NewSwaggerGenerator() *swaggerGen {
	return &swaggerGen{}
}

type BasicParam struct {
	generator.ParamsBase
	m map[string]string
}

func (b *BasicParam) GetBase() *generator.ParamsBase {
	return &b.ParamsBase
}
func (b *BasicParam) SetParam(key string, value string) error {
	if b.m == nil {
		b.m = make(map[string]string)
	}
	b.m[key] = value
	return nil
}
func (b *BasicParam) GetParam(key string) (val string, ok bool) {
	val, ok = b.m[key]
	return
}

func (t *swaggerGen) Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
	params := &BasicParam{}
	t.Setup(in, params)

	resp := &plugin.CodeGeneratorResponse{}

	for _, f := range t.GenFiles {
		if len(f.Service) == 0 {
			continue
		}
		respFile := t.generateSwagger(f, params)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}

	return resp
}

// getPathFieldName  url path parameter
func getPathFieldName(field *descriptor.FieldDescriptorProto) string {
	uri := getTagValue(field, "uri") // gin
	if uri != "" {
		return uri
	}
	return getTagValue(field, "param") // echo
}

func getTagValue(field *descriptor.FieldDescriptorProto, key string) string {
	if field == nil {
		return ""
	}
	tags := tag.GetMoreTags(field)
	if tags != nil {
		tag := reflect.StructTag(*tags)
		fName := tag.Get(key)
		if fName != "" {
			i := strings.Index(fName, ",")
			if i != -1 {
				fName = fName[:i]
			}
			return fName
		}
	}
	return ""
}

func isPathField(field *descriptor.FieldDescriptorProto) bool {
	return getPathFieldName(field) != ""
}

func isQueryField(field *descriptor.FieldDescriptorProto) bool {
	return getTagValue(field, "query") != ""
}

var versionRexp = regexp.MustCompile(`v(\\d+)$`)

func (t *swaggerGen) generateSwagger(file *descriptor.FileDescriptorProto, basicParam *BasicParam) *plugin.CodeGeneratorResponse_File {
	var pkg = file.GetPackage()

	strs := versionRexp.FindStringSubmatch(pkg)
	var vStr string
	if len(strs) >= 2 {
		vStr = strs[1]
	} else {
		vStr = ""
	}
	var swaggerObj = &swaggerObject{
		Paths:   swaggerPathsObject{},
		Swagger: "2.0",
		Info: swaggerInfoObject{
			Title:   file.GetName(),
			Version: vStr,
		},
		Schemes:  []string{"http", "https"},
		Consumes: []string{"application/json", "multipart/form-data"},
		Produces: []string{"application/json"},
	}
	t.defsMap = map[string]*typemap.MessageDefinition{}

	out := &plugin.CodeGeneratorResponse_File{}
	name := naming.GoFileName(file, ".swagger.json")
	for _, svc := range file.Service {
		for _, meth := range svc.Method {
			if !t.ShouldGenForMethod(file, svc, meth) {
				continue
			}
			apiInfo := t.GetHttpInfoCached(file, svc, meth)
			pathItem := swaggerPathItemObject{}
			if originPathItem, ok := swaggerObj.Paths[apiInfo.Path]; ok {
				pathItem = originPathItem
			}

			op := t.getOperationByHTTPMethod(apiInfo.HttpMethod, &pathItem)
			op.Summary = apiInfo.Title
			swaggerObj.Paths[apiInfo.Path] = pathItem
			op.Tags = []string{svc.GetName()}

			// request
			request := t.Reg.MessageDefinition(meth.GetInputType())
			// request cannot represent by simple form
			isComplexRequest := isComplexRequest(request.Descriptor.Field)
			// 解析query参数
			if !isComplexRequest && (apiInfo.HttpMethod == "GET" || apiInfo.HttpMethod == "DELETE") {
				for _, field := range request.Descriptor.Field {
					if !generator.IsScalar(field) {
						continue
					}
					if generator.GetFormOrJSONName(field) == "-" {
						continue
					}
					if isPathField(field) {
						continue
					}
					p := t.getQueryParameter(request, field)
					op.Parameters = append(op.Parameters, p)
				}
			} else {
				if len(request.Descriptor.Field) != 0 {
					p := swaggerParameterObject{}
					p.In = "body"
					p.Required = true
					p.Name = "body"
					p.Schema = &swaggerSchemaObject{}
					p.Schema.Ref = "#/definitions/" + meth.GetInputType()
					op.Parameters = []swaggerParameterObject{p}
				} else {
					op.Parameters = []swaggerParameterObject{}
				}
			}
			// 解析path参数
			isContainPathParameters, pathKeys := isContainPathParameters(apiInfo.Path)
			if isContainPathParameters {
				for _, field := range request.Descriptor.Field {
					if !generator.IsScalar(field) {
						continue
					}
					// :id != uri: "id"  or :id != params: "id"
					if !containsElement(pathKeys, getPathFieldName(field)) {
						continue
					}
					p := t.getPathParameter(request, field)
					op.Parameters = append(op.Parameters, p)
				}
			}
			// response
			resp := swaggerResponseObject{}
			resp.Description = "A successful response."

			// proto 里面的response只定义data里面的
			// 所以需要把code msg data 这一级加上
			resp.Schema.Type = "object"
			resp.Schema.Properties = &swaggerSchemaObjectProperties{}
			p := keyVal{Key: "code", Value: &schemaCore{Type: "string"}}
			*resp.Schema.Properties = append(*resp.Schema.Properties, p)
			p = keyVal{Key: "message", Value: &schemaCore{Type: "string"}}
			*resp.Schema.Properties = append(*resp.Schema.Properties, p)

			if requestID, ok := basicParam.GetParam("requestID"); ok {
				p = keyVal{Key: requestID, Value: &schemaCore{Type: "string"}}
			} else {
				p = keyVal{Key: "requestID", Value: &schemaCore{Type: "string"}}
			}

			*resp.Schema.Properties = append(*resp.Schema.Properties, p)
			p = keyVal{Key: "data", Value: schemaCore{Ref: "#/definitions/" + meth.GetOutputType()}}
			*resp.Schema.Properties = append(*resp.Schema.Properties, p)
			op.Responses = swaggerResponsesObject{"200": resp}
		}
	}

	// walk though definitions
	t.walkThroughFileDefinition(file)
	defs := swaggerDefinitionsObject{}
	swaggerObj.Definitions = defs
	for typ, msg := range t.defsMap {
		def := swaggerSchemaObject{}
		def.Properties = new(swaggerSchemaObjectProperties)
		def.Description = strings.Trim(msg.Comments.Leading, "\n\r ")
		// 生成json或者form参数
		for _, field := range msg.Descriptor.Field {
			p := keyVal{Key: generator.GetFormOrJSONName(field)}
			if p.Key == "-" {
				continue
			}
			if isPathField(field) {
				continue
			}
			if isQueryField(field) {
				continue
			}
			schema := t.schemaForField(file, msg, field)
			if generator.GetFieldRequired(field, t.Reg, msg) {
				def.Required = append(def.Required, p.Key)
			}
			// fix int64 id  =1 [(gogoproto.jsontag) = 'id,string']
			if strings.Contains(getGogoProtoJsonTag(field), ",string") {
				// fix repeated int64 id  =1 [(gogoproto.jsontag) = 'id,string']
				if schema.Type == "array" && schema.Items != nil {
					schema.Items.Type = "string"
				} else {
					schema.Type = "string"
				}
			}
			p.Value = schema
			*def.Properties = append(*def.Properties, p)
		}
		def.Type = "object"
		defs[typ] = def
	}
	b, _ := json.MarshalIndent(swaggerObj, "", "    ")
	str := string(b)
	out.Name = &name
	out.Content = &str
	return out
}

func containsElement(elements []string, element string) bool {
	for _, e := range elements {
		if e == element {
			return true
		}
	}
	return false
}

// getGogoProtoJsonTag
func getGogoProtoJsonTag(field *descriptor.FieldDescriptorProto) string {
	if field == nil {
		return ""
	}
	if field.Options != nil {
		v, err := proto.GetExtension(field.Options, gogoproto.E_Jsontag)
		if err == nil && v.(*string) != nil {
			ret := *(v.(*string))
			return ret
		}
	}
	return field.GetName()
}

// isContainPathParameters eg. /api/:id
func isContainPathParameters(path string) (bool, []string) {
	segments := strings.Split(path, "/")
	pathKeys := []string{}
	for _, v := range segments {
		// eg. :id
		if strings.Contains(v, ":") {
			pathKeys = append(pathKeys, v[1:])
		}

	}
	return len(pathKeys) > 0, pathKeys
}

// isComplexRequest query|path|form｜json
func isComplexRequest(fields []*descriptor.FieldDescriptorProto) bool {
	for _, field := range fields {
		if !generator.IsScalar(field) {
			return true
		}
	}
	return false
}

func (t *swaggerGen) getOperationByHTTPMethod(httpMethod string, pathItem *swaggerPathItemObject) *swaggerOperationObject {
	var op = &swaggerOperationObject{}
	switch httpMethod {
	case http.MethodGet:
		pathItem.Get = op
	case http.MethodPost:
		pathItem.Post = op
	case http.MethodPut:
		pathItem.Put = op
	case http.MethodDelete:
		pathItem.Delete = op
	case http.MethodPatch:
		pathItem.Patch = op
	default:
		pathItem.Get = op
	}
	return op
}

func getQueryFiledName(field *descriptor.FieldDescriptorProto) string {
	name := getTagValue(field, "query")
	if name != "" {
		return name
	}
	return generator.GetFormOrJSONName(field)
}

func (t *swaggerGen) getQueryParameter(input *typemap.MessageDefinition, field *descriptor.FieldDescriptorProto) swaggerParameterObject {
	p := swaggerParameterObject{}
	p.Name = getQueryFiledName(field)
	fComment, _ := t.Reg.FieldComments(input, field)
	cleanComment := tag.GetCommentWithoutTag(fComment.Leading)

	p.Description = strings.Trim(strings.Join(cleanComment, "\n"), "\n\r ")
	p.In = "query"
	p.Required = generator.GetFieldRequired(field, t.Reg, input)
	typ, isArray, format := getFieldSwaggerType(field)
	if isArray {
		p.Items = &swaggerItemsObject{}
		p.Type = "array"
		p.Items.Type = typ
		p.Items.Format = format
	} else {
		p.Type = typ
		p.Format = format
	}
	return p
}

func (t *swaggerGen) getPathParameter(input *typemap.MessageDefinition, field *descriptor.FieldDescriptorProto) swaggerParameterObject {
	p := swaggerParameterObject{}
	p.Name = getPathFieldName(field)
	fComment, _ := t.Reg.FieldComments(input, field)
	cleanComment := tag.GetCommentWithoutTag(fComment.Leading)

	p.Description = strings.Trim(strings.Join(cleanComment, "\n"), "\n\r ")
	p.In = "path"
	p.Required = true // path参数都是必须的
	typ, _, format := getFieldSwaggerType(field)
	p.Type = typ
	p.Format = format
	return p
}

func (t *swaggerGen) schemaForField(file *descriptor.FileDescriptorProto, msg *typemap.MessageDefinition, field *descriptor.FieldDescriptorProto) swaggerSchemaObject {
	schema := swaggerSchemaObject{}
	fComment, err := t.Reg.FieldComments(msg, field)
	if err != nil {
		gen.Error(err, "comment not found err %+v")
	}
	schema.Description = strings.Trim(fComment.Leading, "\n\r ")
	typ, isArray, format := getFieldSwaggerType(field)
	if !generator.IsScalar(field) {
		if generator.IsMap(field, t.Reg) {
			schema.Type = "object"
			mapMsg := t.Reg.MessageDefinition(field.GetTypeName())
			mapValueField := mapMsg.Descriptor.Field[1]
			valSchema := t.schemaForField(file, mapMsg, mapValueField)
			schema.AdditionalProperties = &valSchema
		} else {
			if isArray {
				schema.Items = &swaggerItemsObject{}
				schema.Type = "array"
				schema.Items.Ref = "#/definitions/" + field.GetTypeName()
			} else {
				schema.Ref = "#/definitions/" + field.GetTypeName()
			}
		}
	} else {
		if isArray {
			schema.Items = &swaggerItemsObject{}
			schema.Type = "array"
			schema.Items.Type = typ
			schema.Items.Format = format
		} else {
			schema.Type = typ
			schema.Format = format
		}
	}
	return schema
}

func (t *swaggerGen) walkThroughFileDefinition(file *descriptor.FileDescriptorProto) {
	for _, svc := range file.Service {
		for _, meth := range svc.Method {
			shouldGen := t.ShouldGenForMethod(file, svc, meth)
			if !shouldGen {
				continue
			}
			t.walkThroughMessages(t.Reg.MessageDefinition(meth.GetOutputType()))
			t.walkThroughMessages(t.Reg.MessageDefinition(meth.GetInputType()))
		}
	}
}

func (t *swaggerGen) walkThroughMessages(msg *typemap.MessageDefinition) {
	_, ok := t.defsMap[msg.ProtoName()]
	if ok {
		return
	}
	if !msg.Descriptor.GetOptions().GetMapEntry() {
		t.defsMap[msg.ProtoName()] = msg
	}
	for _, field := range msg.Descriptor.Field {
		if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE {
			t.walkThroughMessages(t.Reg.MessageDefinition(field.GetTypeName()))
		}
	}
}

func getFieldSwaggerType(field *descriptor.FieldDescriptorProto) (typeName string, isArray bool, formatName string) {
	typeName = "unknown"
	switch field.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		typeName = "boolean"
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		typeName = "number"
		formatName = "double"
	case descriptor.FieldDescriptorProto_TYPE_FLOAT:
		typeName = "number"
		formatName = "float"
	case
		descriptor.FieldDescriptorProto_TYPE_INT64,
		descriptor.FieldDescriptorProto_TYPE_UINT64,
		descriptor.FieldDescriptorProto_TYPE_INT32,
		descriptor.FieldDescriptorProto_TYPE_FIXED64,
		descriptor.FieldDescriptorProto_TYPE_FIXED32,
		descriptor.FieldDescriptorProto_TYPE_ENUM,
		descriptor.FieldDescriptorProto_TYPE_UINT32,
		descriptor.FieldDescriptorProto_TYPE_SFIXED32,
		descriptor.FieldDescriptorProto_TYPE_SFIXED64,
		descriptor.FieldDescriptorProto_TYPE_SINT32,
		descriptor.FieldDescriptorProto_TYPE_SINT64:
		typeName = "integer"
	case
		descriptor.FieldDescriptorProto_TYPE_STRING,
		descriptor.FieldDescriptorProto_TYPE_BYTES:
		typeName = "string"
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		typeName = "object"
	}
	if field.Label != nil && *field.Label == descriptor.FieldDescriptorProto_LABEL_REPEATED {
		isArray = true
	}
	return
}
