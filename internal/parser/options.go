package parser

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

const optionsProtoPath = "cleanproto/options.proto"

const optionsProtoSource = `
syntax = "proto3";

package cleanproto;

import "google/protobuf/descriptor.proto";

extend google.protobuf.FileOptions {
  string go_out = 50000;
  string js_out = 50001;
}

extend google.protobuf.FieldOptions {
  string go_type = 50010;
  string js_type = 50011;
}
`

var E_JsOut = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FileOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50001,
	Name:          "cleanproto.js_out",
	Tag:           "bytes,50001,opt,name=js_out",
	Filename:      optionsProtoPath,
}

var E_GoOut = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FileOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50000,
	Name:          "cleanproto.go_out",
	Tag:           "bytes,50000,opt,name=go_out",
	Filename:      optionsProtoPath,
}

var E_GoType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50010,
	Name:          "cleanproto.go_type",
	Tag:           "bytes,50010,opt,name=go_type",
	Filename:      optionsProtoPath,
}

var E_JsType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50011,
	Name:          "cleanproto.js_type",
	Tag:           "bytes,50011,opt,name=js_type",
	Filename:      optionsProtoPath,
}

func jsOutFromOptions(file protoreflect.FileDescriptor) (string, error) {
	opts, ok := file.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return "", nil
	}
	val := proto.GetExtension(opts, E_JsOut)
	str, ok := val.(string)
	if !ok {
		return "", nil
	}
	return str, nil
}

func goOutFromOptions(file protoreflect.FileDescriptor) (string, error) {
	opts, ok := file.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return "", nil
	}
	val := proto.GetExtension(opts, E_GoOut)
	str, ok := val.(string)
	if !ok {
		return "", nil
	}
	return str, nil
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
