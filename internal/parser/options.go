package parser

import (
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

const optionsProtoPath = "cleanproto/options.proto"
const goTypeOptionsProtoPath = "cp/go/options.proto"
const jsTypeOptionsProtoPath = "cp/js/options.proto"
const tsTypeOptionsProtoPath = "cp/ts/options.proto"

const optionsProtoSource = `
syntax = "proto3";

package cp;

import "google/protobuf/descriptor.proto";
import public "cp/go/options.proto";
import public "cp/js/options.proto";
import public "cp/ts/options.proto";

enum AccessPolicyType {
  ACCESS_POLICY_TYPE_UNSPECIFIED = 0;
  NO_AUTH = 1;
  OPTIONAL_AUTH = 2;
  ANY_OF = 3;
}

message AccessPolicy {
  AccessPolicyType policy_type = 1;
  repeated string scopes = 2;
}

extend google.protobuf.MethodOptions {
  AccessPolicy policy = 50030;
}
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

extend google.protobuf.MethodOptions {
  bool custom = 50013;
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

const tsTypeOptionsProtoSource = `
syntax = "proto3";

package cp.ts;

import "google/protobuf/descriptor.proto";

extend google.protobuf.FieldOptions {
  string type = 50016;
  bool encode = 50017;
  bool ignore = 50018;
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

var E_TsType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50016,
	Name:          "cp.ts.type",
	Tag:           "bytes,50016,opt,name=type",
	Filename:      tsTypeOptionsProtoPath,
}

var E_TsEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50017,
	Name:          "cp.ts.encode",
	Tag:           "varint,50017,opt,name=encode",
	Filename:      tsTypeOptionsProtoPath,
}

var E_TsIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50018,
	Name:          "cp.ts.ignore",
	Tag:           "varint,50018,opt,name=ignore",
	Filename:      tsTypeOptionsProtoPath,
}

var E_GoCustom = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.MethodOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50013,
	Name:          "cp.go.custom",
	Tag:           "varint,50013,opt,name=custom",
	Filename:      goTypeOptionsProtoPath,
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

func tsTypeFromFieldOptions(field protoreflect.FieldDescriptor) (string, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return "", nil
	}
	val := proto.GetExtension(opts, E_TsType)
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

func tsEncodeFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return true, nil
	}
	if !proto.HasExtension(opts, E_TsEncode) {
		return true, nil
	}
	val := proto.GetExtension(opts, E_TsEncode)
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

func tsIgnoreFromFieldOptions(field protoreflect.FieldDescriptor) (bool, error) {
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return false, nil
	}
	val := proto.GetExtension(opts, E_TsIgnore)
	b, ok := val.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
}

func goCustomFromMethodOptions(method protoreflect.MethodDescriptor) (bool, error) {
	opts, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return false, nil
	}
	if !proto.HasExtension(opts, E_GoCustom) {
		return false, nil
	}
	val := proto.GetExtension(opts, E_GoCustom)
	b, ok := val.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
}

func policyFromMethodOptions(method protoreflect.MethodDescriptor) (int32, []string, error) {
	opts, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return 0, nil, nil
	}
	var policyType int32
	var scopes []string
	found := false
	opts.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !fd.IsExtension() || fd.Number() != 50030 {
			return true
		}
		found = true
		msg := v.Message()
		md := msg.Descriptor()
		if pf := md.Fields().ByName("policy_type"); pf != nil && msg.Has(pf) {
			policyType = int32(msg.Get(pf).Enum())
		}
		if sf := md.Fields().ByName("scopes"); sf != nil && msg.Has(sf) {
			list := msg.Get(sf).List()
			for i := 0; i < list.Len(); i++ {
				scopes = append(scopes, list.Get(i).String())
			}
		}
		return false
	})
	if found {
		return policyType, scopes, nil
	}
	unknown := opts.ProtoReflect().GetUnknown()
	for len(unknown) > 0 {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return 0, nil, protowire.ParseError(n)
		}
		unknown = unknown[n:]
		if num != 50030 {
			m := protowire.ConsumeFieldValue(num, typ, unknown)
			if m < 0 {
				return 0, nil, protowire.ParseError(m)
			}
			unknown = unknown[m:]
			continue
		}
		if typ != protowire.BytesType {
			m := protowire.ConsumeFieldValue(num, typ, unknown)
			if m < 0 {
				return 0, nil, protowire.ParseError(m)
			}
			unknown = unknown[m:]
			continue
		}
		value, m := protowire.ConsumeBytes(unknown)
		if m < 0 {
			return 0, nil, protowire.ParseError(m)
		}
		return parsePolicyBytes(value)
	}
	return 0, nil, nil
}

func parsePolicyBytes(value []byte) (int32, []string, error) {
	policyType := int32(0)
	var scopes []string
	for len(value) > 0 {
		fnum, ftyp, fn := protowire.ConsumeTag(value)
		if fn < 0 {
			return 0, nil, protowire.ParseError(fn)
		}
		value = value[fn:]
		switch {
		case fnum == 1 && ftyp == protowire.VarintType:
			v, vn := protowire.ConsumeVarint(value)
			if vn < 0 {
				return 0, nil, protowire.ParseError(vn)
			}
			policyType = int32(v)
			value = value[vn:]
		case fnum == 2 && ftyp == protowire.BytesType:
			s, sn := protowire.ConsumeString(value)
			if sn < 0 {
				return 0, nil, protowire.ParseError(sn)
			}
			scopes = append(scopes, s)
			value = value[sn:]
		default:
			sn := protowire.ConsumeFieldValue(fnum, ftyp, value)
			if sn < 0 {
				return 0, nil, protowire.ParseError(sn)
			}
			value = value[sn:]
		}
	}
	return policyType, scopes, nil
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
