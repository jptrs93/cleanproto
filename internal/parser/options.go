package parser

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

const optionsProtoPath = "cleanproto/options.proto"
const goTypeOptionsProtoPath = "cp/go/options.proto"
const jsTypeOptionsProtoPath = "cp/js/options.proto"

const optionsProtoSource = `
syntax = "proto3";

package cleanproto;

import public "cp/go/options.proto";
import public "cp/js/options.proto";
`

const goTypeOptionsProtoSource = `
syntax = "proto3";

package cp.go;

import "google/protobuf/descriptor.proto";

extend google.protobuf.FieldOptions {
  string type = 50010;
}
`

const jsTypeOptionsProtoSource = `
syntax = "proto3";

package cp.js;

import "google/protobuf/descriptor.proto";

extend google.protobuf.FieldOptions {
  string type = 50011;
}
`

var E_GoType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50010,
	Name:          "cp.go.type",
	Tag:           "bytes,50010,opt,name=type",
	Filename:      goTypeOptionsProtoPath,
}

var E_JsType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50011,
	Name:          "cp.js.type",
	Tag:           "bytes,50011,opt,name=type",
	Filename:      jsTypeOptionsProtoPath,
}

func goTypeFromFieldOptions(field protoreflect.FieldDescriptor) (string, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return "", nil
	}
	val := proto.GetExtension(opts, E_GoType)
	str, ok := val.(string)
	if !ok || str == "" {
		return "", nil
	}
	return str, nil
}

func jsTypeFromFieldOptions(field protoreflect.FieldDescriptor) (string, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return "", nil
	}
	val := proto.GetExtension(opts, E_JsType)
	str, ok := val.(string)
	if !ok || str == "" {
		return "", nil
	}
	return str, nil
}

func goPackageFromOptions(file protoreflect.FileDescriptor) string {
	opts, ok := file.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return ""
	}
	goPkg := opts.GetGoPackage()
	if goPkg == "" {
		return ""
	}
	if strings.Contains(goPkg, ";") {
		parts := strings.Split(goPkg, ";")
		return parts[len(parts)-1]
	}
	goPkg = strings.TrimSuffix(goPkg, "/")
	if idx := strings.LastIndex(goPkg, "/"); idx != -1 {
		return goPkg[idx+1:]
	}
	return goPkg
}
