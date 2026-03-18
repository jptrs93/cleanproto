package parser

import (
	"strings"

	"github.com/jptrs93/cleanproto"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const optionsProtoPath = cp.OptionsProtoPath

var optionsProtoSource = cp.OptionsProtoSource

var E_GoType = cp.E_GoType
var E_JsType = cp.E_JsType
var E_GoEncode = cp.E_GoEncode
var E_JsEncode = cp.E_JsEncode
var E_GoIgnore = cp.E_GoIgnore
var E_JsIgnore = cp.E_JsIgnore
var E_TsType = cp.E_TsType
var E_TsEncode = cp.E_TsEncode
var E_TsIgnore = cp.E_TsIgnore
var E_GoCustom = cp.E_GoCustom
var E_AuditId = cp.E_AuditId

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

func auditIDFromMethodOptions(method protoreflect.MethodDescriptor) (string, error) {
	opts, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return "", nil
	}
	if !proto.HasExtension(opts, E_AuditId) {
		return "", nil
	}
	val := proto.GetExtension(opts, E_AuditId)
	str, ok := val.(string)
	if !ok || str == "" {
		return "", nil
	}
	return str, nil
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
