package cp

import (
	_ "embed"

	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

const OptionsProtoPath = "options.proto"

//go:embed options.proto
var OptionsProtoSource string

var E_GoType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50010,
	Name:          "cp.go_type",
	Tag:           "bytes,50010,opt,name=go_type",
	Filename:      OptionsProtoPath,
}

var E_JsType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50011,
	Name:          "cp.js_type",
	Tag:           "bytes,50011,opt,name=js_type",
	Filename:      OptionsProtoPath,
}

var E_GoEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50012,
	Name:          "cp.go_encode",
	Tag:           "varint,50012,opt,name=go_encode",
	Filename:      OptionsProtoPath,
}

var E_JsEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50013,
	Name:          "cp.js_encode",
	Tag:           "varint,50013,opt,name=js_encode",
	Filename:      OptionsProtoPath,
}

var E_GoIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50014,
	Name:          "cp.go_ignore",
	Tag:           "varint,50014,opt,name=go_ignore",
	Filename:      OptionsProtoPath,
}

var E_JsIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50015,
	Name:          "cp.js_ignore",
	Tag:           "varint,50015,opt,name=js_ignore",
	Filename:      OptionsProtoPath,
}

var E_TsType = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*string)(nil),
	Field:         50016,
	Name:          "cp.ts_type",
	Tag:           "bytes,50016,opt,name=ts_type",
	Filename:      OptionsProtoPath,
}

var E_TsEncode = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50017,
	Name:          "cp.ts_encode",
	Tag:           "varint,50017,opt,name=ts_encode",
	Filename:      OptionsProtoPath,
}

var E_TsIgnore = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.FieldOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50018,
	Name:          "cp.ts_ignore",
	Tag:           "varint,50018,opt,name=ts_ignore",
	Filename:      OptionsProtoPath,
}

var E_GoCustom = &protoimpl.ExtensionInfo{
	ExtendedType:  (*descriptorpb.MethodOptions)(nil),
	ExtensionType: (*bool)(nil),
	Field:         50013,
	Name:          "cp.go_custom",
	Tag:           "varint,50013,opt,name=go_custom",
	Filename:      OptionsProtoPath,
}
