package parser

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jptrs93/cleanproto/internal/ir"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Parser struct {
	ImportPaths []string
}

func (p *Parser) Parse(ctx context.Context, filePaths []string) ([]ir.File, error) {
	resolver := &protocompile.SourceResolver{
		ImportPaths: p.ImportPaths,
		Accessor: func(path string) (io.ReadCloser, error) {
			if path == optionsProtoPath || strings.HasSuffix(path, string(os.PathSeparator)+optionsProtoPath) {
				return io.NopCloser(strings.NewReader(optionsProtoSource)), nil
			}
			return os.Open(path)
		},
	}
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(resolver),
	}
	files, err := compiler.Compile(ctx, filePaths...)
	if err != nil {
		return nil, err
	}

	var result []ir.File
	for _, file := range files {
		irFile, err := fileToIR(file)
		if err != nil {
			return nil, err
		}
		result = append(result, irFile)
	}
	return result, nil
}

func fileToIR(file protoreflect.FileDescriptor) (ir.File, error) {
	if file.Syntax() != protoreflect.Proto3 {
		return ir.File{}, fmt.Errorf("only proto3 is supported: %s", file.Path())
	}
	jsOut, err := jsOutFromOptions(file)
	if err != nil {
		return ir.File{}, err
	}
	goOut, err := goOutFromOptions(file)
	if err != nil {
		return ir.File{}, err
	}
	goPkg := goPackageFromOptions(file)
	if goPkg == "" {
		goPkg = string(file.Package())
	}
	out := ir.File{
		Path:      file.Path(),
		Package:   string(file.Package()),
		GoPackage: goPkg,
		GoOut:     goOut,
		JsOut:     jsOut,
	}
	msgs, err := collectMessages(file.Messages(), nil)
	if err != nil {
		return ir.File{}, err
	}
	enums, err := collectEnums(file.Enums(), nil)
	if err != nil {
		return ir.File{}, err
	}
	nestedEnums, err := collectMessageEnums(file.Messages(), nil)
	if err != nil {
		return ir.File{}, err
	}
	out.Enums = append(out.Enums, enums...)
	out.Enums = append(out.Enums, nestedEnums...)
	out.Messages = msgs
	return out, nil
}

func collectMessages(messages protoreflect.MessageDescriptors, prefix []string) ([]ir.Message, error) {
	var result []ir.Message
	for i := 0; i < messages.Len(); i++ {
		msg := messages.Get(i)
		if msg.IsMapEntry() {
			continue
		}
		nameParts := append(prefix, string(msg.Name()))
		msgName := ir.GoName(joinName(nameParts))
		irMsg := ir.Message{
			Name:     msgName,
			FullName: string(msg.FullName()),
		}
		fields, err := collectFields(msg.Fields())
		if err != nil {
			return nil, err
		}
		irMsg.Fields = fields
		result = append(result, irMsg)

		nested, err := collectMessages(msg.Messages(), nameParts)
		if err != nil {
			return nil, err
		}
		result = append(result, nested...)
	}
	return result, nil
}

func collectEnums(enums protoreflect.EnumDescriptors, prefix []string) ([]ir.Enum, error) {
	var result []ir.Enum
	for i := 0; i < enums.Len(); i++ {
		enum := enums.Get(i)
		nameParts := append(prefix, string(enum.Name()))
		irEnum := ir.Enum{
			Name:     ir.GoName(joinName(nameParts)),
			FullName: string(enum.FullName()),
		}
		for j := 0; j < enum.Values().Len(); j++ {
			value := enum.Values().Get(j)
			irEnum.Values = append(irEnum.Values, ir.EnumValue{
				Name:   string(value.Name()),
				Number: int32(value.Number()),
			})
		}
		result = append(result, irEnum)
	}
	return result, nil
}

func collectMessageEnums(messages protoreflect.MessageDescriptors, prefix []string) ([]ir.Enum, error) {
	var result []ir.Enum
	for i := 0; i < messages.Len(); i++ {
		msg := messages.Get(i)
		if msg.IsMapEntry() {
			continue
		}
		nameParts := append(prefix, string(msg.Name()))
		enums, err := collectEnums(msg.Enums(), nameParts)
		if err != nil {
			return nil, err
		}
		result = append(result, enums...)
		nested, err := collectMessageEnums(msg.Messages(), nameParts)
		if err != nil {
			return nil, err
		}
		result = append(result, nested...)
	}
	return result, nil
}

func collectFields(fields protoreflect.FieldDescriptors) ([]ir.Field, error) {
	var result []ir.Field
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if oneof := field.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			return nil, fmt.Errorf("oneof is not supported: %s", field.FullName())
		}
		kind, err := kindFromField(field)
		if err != nil {
			return nil, err
		}
		var msgName string
		var enumName string
		var isMap bool
		var mapKeyKind ir.Kind
		var mapValueKind ir.Kind
		var mapValueMessage string
		var mapValueEnum string
		var isTimestamp bool
		var isDuration bool
		var goType string
		var jsType string
		if field.IsMap() {
			isMap = true
			keyKind, err := kindFromField(field.MapKey())
			if err != nil {
				return nil, err
			}
			valKind, err := kindFromField(field.MapValue())
			if err != nil {
				return nil, err
			}
			mapKeyKind = keyKind
			mapValueKind = valKind
			if valKind == ir.KindMessage {
				mapValueMessage = string(field.MapValue().Message().FullName())
			}
			if valKind == ir.KindEnum {
				mapValueEnum = string(field.MapValue().Enum().FullName())
			}
		} else if kind == ir.KindMessage {
			msgName = string(field.Message().FullName())
			if msgName == "google.protobuf.Timestamp" {
				isTimestamp = true
			}
			if msgName == "google.protobuf.Duration" {
				isDuration = true
			}
		} else if kind == ir.KindEnum {
			enumName = string(field.Enum().FullName())
		}
		goType, err = goTypeFromFieldOptions(field)
		if err != nil {
			return nil, err
		}
		jsType, err = jsTypeFromFieldOptions(field)
		if err != nil {
			return nil, err
		}
		if err := validateNativeTypes(field.FullName(), kind, msgName, goType, jsType, field.IsMap()); err != nil {
			return nil, err
		}
		isOptional := field.HasPresence() && !field.IsList() && !field.IsMap() && field.Kind() != protoreflect.MessageKind
		result = append(result, ir.Field{
			Name:            ir.JsName(string(field.Name())),
			Number:          int(field.Number()),
			Kind:            kind,
			IsRepeated:      field.IsList(),
			IsOptional:      isOptional,
			IsPacked:        field.IsPacked(),
			IsMap:           isMap,
			IsTimestamp:     isTimestamp,
			IsDuration:      isDuration,
			GoType:          goType,
			JSType:          jsType,
			MapKeyKind:      mapKeyKind,
			MapValueKind:    mapValueKind,
			MapValueMessage: mapValueMessage,
			MapValueEnum:    mapValueEnum,
			MessageFullName: msgName,
			EnumFullName:    enumName,
		})
	}
	return result, nil
}

func validateNativeTypes(fullName protoreflect.FullName, kind ir.Kind, msgName string, goType string, jsType string, isMap bool) error {
	if isMap && (goType != "" || jsType != "") {
		return fmt.Errorf("cleanproto.go_type/js_type not supported on map fields: %s", fullName)
	}
	if goType != "" {
		if !isSupportedGoType(kind, msgName, goType) {
			return fmt.Errorf("unsupported cleanproto.go_type %q for %s", goType, fullName)
		}
	}
	if jsType != "" {
		if !isSupportedJSType(kind, msgName, jsType) {
			return fmt.Errorf("unsupported cleanproto.js_type %q for %s", jsType, fullName)
		}
	}
	return nil
}

func isSupportedGoType(kind ir.Kind, msgName string, goType string) bool {
	switch goType {
	case "time.Time":
		return (kind == ir.KindMessage && msgName == "google.protobuf.Timestamp") || kind == ir.KindInt32 || kind == ir.KindInt64
	case "time.Duration":
		return (kind == ir.KindMessage && msgName == "google.protobuf.Duration") || kind == ir.KindInt32 || kind == ir.KindInt64
	case "github.com/google/uuid.UUID":
		return kind == ir.KindBytes
	default:
		return false
	}
}

func isSupportedJSType(kind ir.Kind, msgName string, jsType string) bool {
	if jsType != "number" && jsType != "bigint" {
		return false
	}
	if kind == ir.KindInt32 || kind == ir.KindInt64 {
		return true
	}
	if kind == ir.KindMessage && (msgName == "google.protobuf.Timestamp" || msgName == "google.protobuf.Duration") {
		return true
	}
	return false
}

func kindFromField(field protoreflect.FieldDescriptor) (ir.Kind, error) {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return ir.KindBool, nil
	case protoreflect.Int32Kind:
		return ir.KindInt32, nil
	case protoreflect.Int64Kind:
		return ir.KindInt64, nil
	case protoreflect.Uint32Kind:
		return ir.KindUint32, nil
	case protoreflect.Uint64Kind:
		return ir.KindUint64, nil
	case protoreflect.Sint32Kind:
		return ir.KindSint32, nil
	case protoreflect.Sint64Kind:
		return ir.KindSint64, nil
	case protoreflect.Fixed32Kind:
		return ir.KindFixed32, nil
	case protoreflect.Fixed64Kind:
		return ir.KindFixed64, nil
	case protoreflect.Sfixed32Kind:
		return ir.KindSfixed32, nil
	case protoreflect.Sfixed64Kind:
		return ir.KindSfixed64, nil
	case protoreflect.FloatKind:
		return ir.KindFloat, nil
	case protoreflect.DoubleKind:
		return ir.KindDouble, nil
	case protoreflect.StringKind:
		return ir.KindString, nil
	case protoreflect.BytesKind:
		return ir.KindBytes, nil
	case protoreflect.MessageKind:
		return ir.KindMessage, nil
	case protoreflect.EnumKind:
		return ir.KindEnum, nil
	default:
		return 0, fmt.Errorf("unsupported field kind: %s", field.Kind())
	}
}

func joinName(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "_")
}
