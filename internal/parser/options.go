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
  bool encode = 50012;
  bool ignore = 50014;
}
`

const jsTypeOptionsProtoSource = `
syntax = "proto3";

package cp.js;

import "google/protobuf/descriptor.proto";

extend google.protobuf.FieldOptions {
  string type = 50011;
  bool encode = 50013;
  bool ignore = 50015;
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

var E_GoEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50012,
	Name:          "cp.go.encode",
	Tag:           "varint,50012,opt,name=encode",
	Filename:      goTypeOptionsProtoPath,
}

var E_JsEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50013,
	Name:          "cp.js.encode",
	Tag:           "varint,50013,opt,name=encode",
	Filename:      jsTypeOptionsProtoPath,
}

var E_GoIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50014,
	Name:          "cp.go.ignore",
	Tag:           "varint,50014,opt,name=ignore",
	Filename:      goTypeOptionsProtoPath,
}

var E_JsIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50015,
	Name:          "cp.js.ignore",
	Tag:           "varint,50015,opt,name=ignore",
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

func goEncodeFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return true, nil
	}
	if !proto.HasExtension(opts, E_GoEncode) {
		return true, nil
	}
	val := proto.GetExtension(opts, E_GoEncode)
	b, ok := val.(bool)
	if !ok {
		return true, nil
	}
	return b, nil
}

func jsEncodeFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return true, nil
	}
	if !proto.HasExtension(opts, E_JsEncode) {
		return true, nil
	}
	val := proto.GetExtension(opts, E_JsEncode)
	b, ok := val.(bool)
	if !ok {
		return true, nil
	}
	return b, nil
}

func goIgnoreFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return false, nil
	}
	val := proto.GetExtension(opts, E_GoIgnore)
	b, ok := val.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
}

func jsIgnoreFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return false, nil
	}
	val := proto.GetExtension(opts, E_JsIgnore)
	b, ok := val.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
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
