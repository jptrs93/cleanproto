package gogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jptrs93/cleanproto/internal/generate"
	"github.com/jptrs93/cleanproto/internal/generate/templates"
	"github.com/jptrs93/cleanproto/internal/ir"
)

type Generator struct{}

func (g Generator) Name() string {
	return "go"
}

func (g Generator) Generate(files []ir.File, options generate.Options) ([]generate.OutputFile, error) {
	tmpl, err := template.ParseFS(templates.FS, "go_file.tmpl")
	if err != nil {
		return nil, err
	}

	msgIndex := indexMessages(files)
	var outputs []generate.OutputFile
	var utilPkg string
	var utilDir string
	for _, file := range files {
		goOut := options.GoOut
		if goOut == "" {
			goOut = file.GoOut
		}
		if goOut == "" {
			continue
		}
		pkg := options.GoPackage
		if pkg == "" {
			pkg = file.GoPackage
		}
		if pkg == "" {
			return nil, fmt.Errorf("go package name is required (set -go_pkg or option go_package)")
		}
		if utilPkg == "" {
			utilPkg = pkg
			utilDir = goOut
		}
		data, err := buildGoFileData(file, msgIndex, pkg)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		outPath := filepath.Join(goOut, "model.gen.go")
		outputs = append(outputs, generate.OutputFile{
			Path:    outPath,
			Content: buf.Bytes(),
		})
	}
	if len(outputs) == 0 {
		return nil, nil
	}
	utilContent, err := loadUtilSource(utilPkg)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, generate.OutputFile{
		Path:    filepath.Join(utilDir, "util.go"),
		Content: utilContent,
	})
	return outputs, nil
}

type goFileData struct {
	Package  string
	Imports  []string
	Messages []goMessage
}

type goMessage struct {
	Name          string
	Fields        []goField
	EncodeLines   []string
	DecodeCases   []goDecodeCase
	NeedsMsgBytes bool
	NeedsTmpBytes bool
}

type goField struct {
	Name string
	Type string
}

type goDecodeCase struct {
	Number int
	Lines  []string
}

func buildGoFileData(file ir.File, msgIndex map[string]ir.Message, pkg string) (goFileData, error) {
	data := goFileData{Package: pkg}
	var usesMath bool
	var usesTime bool
	for _, msg := range file.Messages {
		goMsg, mathNeeded, timeNeeded, err := buildGoMessage(msg, msgIndex)
		if err != nil {
			return goFileData{}, err
		}
		if mathNeeded {
			usesMath = true
		}
		if timeNeeded {
			usesTime = true
		}
		data.Messages = append(data.Messages, goMsg)
	}
	imports := []string{
		"google.golang.org/protobuf/encoding/protowire",
	}
	if usesMath {
		imports = append([]string{"math"}, imports...)
	}
	if usesTime {
		imports = append([]string{"time"}, imports...)
	}
	data.Imports = imports
	return data, nil
}

func buildGoMessage(msg ir.Message, msgIndex map[string]ir.Message) (goMessage, bool, bool, error) {
	out := goMessage{Name: msg.Name}
	var usesMath bool
	var usesTime bool
	for _, field := range msg.Fields {
		goType, mathNeeded, err := goFieldType(field, msgIndex)
		if err != nil {
			return goMessage{}, false, false, err
		}
		if mathNeeded {
			usesMath = true
		}
		if field.IsTimestamp {
			usesTime = true
		}
		out.Fields = append(out.Fields, goField{
			Name: ir.GoName(field.Name),
			Type: goType,
		})
	}

	encodeLines, err := buildGoEncodeLines(msg, msgIndex)
	if err != nil {
		return goMessage{}, false, false, err
	}
	out.EncodeLines = encodeLines

	decodeCases, needsMsgBytes, needsTmpBytes, err := buildGoDecodeCases(msg, msgIndex)
	if err != nil {
		return goMessage{}, false, false, err
	}
	out.DecodeCases = decodeCases
	out.NeedsMsgBytes = needsMsgBytes
	out.NeedsTmpBytes = needsTmpBytes

	return out, usesMath, usesTime, nil
}

func goFieldType(field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	if field.IsTimestamp {
		base := "time.Time"
		if field.IsRepeated {
			return "[]" + base, false, nil
		}
		if field.IsOptional {
			return "*" + base, false, nil
		}
		return base, false, nil
	}
	if field.IsMap {
		keyType, err := goMapKeyType(field.MapKeyKind)
		if err != nil {
			return "", false, err
		}
		valueType, mathNeeded, err := goMapValueType(field, msgIndex)
		if err != nil {
			return "", false, err
		}
		return "map[" + keyType + "]" + valueType, mathNeeded, nil
	}
	if field.IsRepeated {
		if field.Kind == ir.KindMessage {
			msg, ok := msgIndex[field.MessageFullName]
			if !ok {
				return "", false, fmt.Errorf("unknown message type: %s", field.MessageFullName)
			}
			return "[]*" + msg.Name, false, nil
		}
		if field.Kind == ir.KindBytes {
			return "[][]byte", false, nil
		}
		t, mathNeeded, err := goScalarType(field.Kind, false)
		if err != nil {
			return "", false, err
		}
		return "[]" + t, mathNeeded, nil
	}

	if field.Kind == ir.KindMessage {
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			return "", false, fmt.Errorf("unknown message type: %s", field.MessageFullName)
		}
		return "*" + msg.Name, false, nil
	}
	if field.Kind == ir.KindBytes {
		if field.IsOptional {
			return "*[]byte", false, nil
		}
		return "[]byte", false, nil
	}
	t, mathNeeded, err := goScalarType(field.Kind, field.IsOptional)
	if err != nil {
		return "", false, err
	}
	return t, mathNeeded, nil
}

func goScalarType(kind ir.Kind, optional bool) (string, bool, error) {
	var t string
	var needsMath bool
	switch kind {
	case ir.KindBool:
		t = "bool"
	case ir.KindInt32:
		t = "int32"
	case ir.KindInt64:
		t = "int64"
	case ir.KindUint32:
		t = "uint32"
	case ir.KindUint64:
		t = "uint64"
	case ir.KindSint32:
		t = "int32"
	case ir.KindSint64:
		t = "int64"
	case ir.KindFixed32:
		t = "uint32"
	case ir.KindFixed64:
		t = "uint64"
	case ir.KindSfixed32:
		t = "int32"
	case ir.KindSfixed64:
		t = "int64"
	case ir.KindFloat:
		t = "float32"
		needsMath = true
	case ir.KindDouble:
		t = "float64"
		needsMath = true
	case ir.KindString:
		t = "string"
	case ir.KindEnum:
		t = "int32"
	default:
		return "", false, fmt.Errorf("unsupported scalar kind: %v", kind)
	}
	if optional {
		return "*" + t, needsMath, nil
	}
	return t, needsMath, nil
}

func buildGoEncodeLines(msg ir.Message, msgIndex map[string]ir.Message) ([]string, error) {
	var lines []string
	for _, field := range msg.Fields {
		fieldName := "m." + ir.GoName(field.Name)
		switch {
		case field.IsTimestamp:
			tsLines, err := goEncodeTimestamp(fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, tsLines...)
		case field.IsMap:
			mapLines, err := goEncodeMap(fieldName, field, msgIndex)
			if err != nil {
				return nil, err
			}
			lines = append(lines, mapLines...)
		case field.IsRepeated && field.Kind == ir.KindMessage:
			lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
			lines = append(lines, "if item == nil {", "continue", "}")
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendBytes(b, item.Encode())"))
			lines = append(lines, "}")
		case field.IsRepeated:
			if field.IsPacked && isGoPackable(field.Kind) {
				packedLines, err := goEncodePacked(fieldName, field)
				if err != nil {
					return nil, err
				}
				lines = append(lines, packedLines...)
			} else {
				lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
				encodeLines, err := goEncodeScalar("item", field)
				if err != nil {
					return nil, err
				}
				lines = append(lines, encodeLines...)
				lines = append(lines, "}")
			}
		case field.Kind == ir.KindMessage:
			lines = append(lines, fmt.Sprintf("if %s != nil {", fieldName))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendBytes(b, %s.Encode())", fieldName))
			lines = append(lines, "}")
		case field.IsOptional:
			lines = append(lines, fmt.Sprintf("if %s != nil {", fieldName))
			encodeLines, err := goEncodeScalar("*"+fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, encodeLines...)
			lines = append(lines, "}")
		default:
			cond := goDefaultCheck(fieldName, field)
			if cond != "" {
				lines = append(lines, fmt.Sprintf("if %s {", cond))
			}
			encodeLines, err := goEncodeScalar(fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, encodeLines...)
			if cond != "" {
				lines = append(lines, "}")
			}
		}
	}
	return lines, nil
}

func goDefaultCheck(name string, field ir.Field) string {
	switch field.Kind {
	case ir.KindString:
		return name + " != \"\""
	case ir.KindBytes:
		return "len(" + name + ") > 0"
	case ir.KindBool:
		return name
	default:
		return name + " != 0"
	}
}

func goEncodeScalar(name string, field ir.Field) ([]string, error) {
	switch field.Kind {
	case ir.KindString:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number),
			fmt.Sprintf("b = protowire.AppendBytes(b, []byte(%s))", name),
		}, nil
	case ir.KindBytes:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number),
			fmt.Sprintf("b = protowire.AppendBytes(b, %s)", name),
		}, nil
	case ir.KindBool:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("if %s {", name),
			"b = protowire.AppendVarint(b, 1)",
			"} else {",
			"b = protowire.AppendVarint(b, 0)",
			"}",
		}, nil
	case ir.KindFloat:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.Fixed32Type)", field.Number),
			fmt.Sprintf("b = protowire.AppendFixed32(b, math.Float32bits(%s))", name),
		}, nil
	case ir.KindDouble:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.Fixed64Type)", field.Number),
			fmt.Sprintf("b = protowire.AppendFixed64(b, math.Float64bits(%s))", name),
		}, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, uint64(uint32(%s)))", name),
		}, nil
	case ir.KindUint32:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, uint64(%s))", name),
		}, nil
	case ir.KindSint32:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, protowire.EncodeZigZag(int64(%s)))", name),
		}, nil
	case ir.KindInt64:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, uint64(%s))", name),
		}, nil
	case ir.KindUint64:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, uint64(%s))", name),
		}, nil
	case ir.KindSint64:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number),
			fmt.Sprintf("b = protowire.AppendVarint(b, protowire.EncodeZigZag(%s))", name),
		}, nil
	case ir.KindFixed32, ir.KindSfixed32:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.Fixed32Type)", field.Number),
			fmt.Sprintf("b = protowire.AppendFixed32(b, uint32(%s))", name),
		}, nil
	case ir.KindFixed64, ir.KindSfixed64:
		return []string{
			fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.Fixed64Type)", field.Number),
			fmt.Sprintf("b = protowire.AppendFixed64(b, uint64(%s))", name),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported encode kind: %v", field.Kind)
	}
}

func goEncodeTimestamp(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	if field.IsRepeated {
		lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
		lines = append(lines, "if item.IsZero() {", "continue", "}")
		if field.TimestampUnit == "wkt" {
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
			lines = append(lines, "b = protowire.AppendBytes(b, EncodeTimestamp(item))")
		} else {
			valueExpr := goTimestampValue("item", field.TimestampUnit, field.Kind)
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendVarint(b, %s)", valueExpr))
		}
		lines = append(lines, "}")
		return lines, nil
	}

	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("if %s != nil && !%s.IsZero() {", fieldName, fieldName))
		if field.TimestampUnit == "wkt" {
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendBytes(b, EncodeTimestamp(*%s))", fieldName))
		} else {
			valueExpr := goTimestampValue("*"+fieldName, field.TimestampUnit, field.Kind)
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendVarint(b, %s)", valueExpr))
		}
		lines = append(lines, "}")
		return lines, nil
	}

	lines = append(lines, fmt.Sprintf("if !%s.IsZero() {", fieldName))
	if field.TimestampUnit == "wkt" {
		lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
		lines = append(lines, fmt.Sprintf("b = protowire.AppendBytes(b, EncodeTimestamp(%s))", fieldName))
	} else {
		valueExpr := goTimestampValue(fieldName, field.TimestampUnit, field.Kind)
		lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.VarintType)", field.Number))
		lines = append(lines, fmt.Sprintf("b = protowire.AppendVarint(b, %s)", valueExpr))
	}
	lines = append(lines, "}")
	return lines, nil
}

func goTimestampValue(name string, unit string, kind ir.Kind) string {
	var value string
	if unit == "milliseconds" {
		value = name + ".UnixMilli()"
	} else {
		value = name + ".Unix()"
	}
	if kind == ir.KindInt32 {
		return "uint64(uint32(" + value + "))"
	}
	return "uint64(" + value + ")"
}

func goMapKeyType(kind ir.Kind) (string, error) {
	switch kind {
	case ir.KindBool:
		return "bool", nil
	case ir.KindString:
		return "string", nil
	case ir.KindInt32, ir.KindSint32, ir.KindSfixed32:
		return "int32", nil
	case ir.KindInt64, ir.KindSint64, ir.KindSfixed64:
		return "int64", nil
	case ir.KindUint32, ir.KindFixed32:
		return "uint32", nil
	case ir.KindUint64, ir.KindFixed64:
		return "uint64", nil
	default:
		return "", fmt.Errorf("unsupported map key type: %v", kind)
	}
}

func goMapValueType(field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	switch field.MapValueKind {
	case ir.KindMessage:
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", false, fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		return "*" + msg.Name, false, nil
	case ir.KindBytes:
		return "[]byte", false, nil
	default:
		return goScalarType(field.MapValueKind, false)
	}
}

func goEncodeMap(fieldName string, field ir.Field, msgIndex map[string]ir.Message) ([]string, error) {
	var lines []string
	lines = append(lines, fmt.Sprintf("for key, value := range %s {", fieldName))
	lines = append(lines, "var entry []byte")
	keyField := ir.Field{Number: 1, Kind: field.MapKeyKind}
	valField := ir.Field{Number: 2, Kind: field.MapValueKind}
	keyLines, err := goEncodeScalar("key", keyField)
	if err != nil {
		return nil, err
	}
	lines = append(lines, prefixLines(keyLines, "entry = ")...)
	if field.MapValueKind == ir.KindMessage {
		lines = append(lines, "if value != nil {")
		lines = append(lines, fmt.Sprintf("entry = protowire.AppendTag(entry, 2, protowire.BytesType)"))
		lines = append(lines, fmt.Sprintf("entry = protowire.AppendBytes(entry, value.Encode())"))
		lines = append(lines, "}")
	} else if field.MapValueKind == ir.KindBytes {
		lines = append(lines, fmt.Sprintf("if len(value) > 0 {"))
		lines = append(lines, fmt.Sprintf("entry = protowire.AppendTag(entry, 2, protowire.BytesType)"))
		lines = append(lines, fmt.Sprintf("entry = protowire.AppendBytes(entry, value)"))
		lines = append(lines, "}")
	} else {
		valLines, err := goEncodeScalar("value", valField)
		if err != nil {
			return nil, err
		}
		cond := goDefaultCheck("value", ir.Field{Kind: field.MapValueKind})
		if cond != "" {
			lines = append(lines, fmt.Sprintf("if %s {", cond))
			lines = append(lines, prefixLines(valLines, "entry = ")...)
			lines = append(lines, "}")
		} else {
			lines = append(lines, prefixLines(valLines, "entry = ")...)
		}
	}
	lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
	lines = append(lines, "b = protowire.AppendBytes(b, entry)")
	lines = append(lines, "}")
	return lines, nil
}

func goDecodeMap(fieldName string, field ir.Field, msgIndex map[string]ir.Message) ([]string, bool, error) {
	var lines []string
	keyConsume, err := goConsumeFunc(ir.Field{Kind: field.MapKeyKind})
	if err != nil {
		return nil, false, err
	}
	valConsume, err := goConsumeMapValueFunc(field, msgIndex)
	if err != nil {
		return nil, false, err
	}
	lines = append(lines, fmt.Sprintf("var mapKey %s", mustGoMapKeyType(field.MapKeyKind)))
	lines = append(lines, fmt.Sprintf("var mapValue %s", mustGoMapValueType(field, msgIndex)))
	lines = append(lines, fmt.Sprintf("b, mapKey, mapValue, err = ConsumeMapEntry(b, typ, %s, %s)", keyConsume, valConsume))
	lines = append(lines, "if err != nil {", "return nil, err", "}")
	lines = append(lines, fmt.Sprintf("if %s == nil {", fieldName))
	lines = append(lines, fmt.Sprintf("%s = make(map[%s]%s)", fieldName, mustGoMapKeyType(field.MapKeyKind), mustGoMapValueType(field, msgIndex)))
	lines = append(lines, "}")
	lines = append(lines, fmt.Sprintf("%s[mapKey] = mapValue", fieldName))
	return lines, false, nil
}

func goDecodeMapScalar(kind ir.Kind, target string, bufName string) ([]string, error) {
	var lines []string
	switch kind {
	case ir.KindString:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeString(%s, typ2)", bufName, target, bufName))
	case ir.KindBool:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeBool(%s, typ2)", bufName, target, bufName))
	case ir.KindInt32, ir.KindEnum:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeVarInt32(%s, typ2)", bufName, target, bufName))
	case ir.KindSint32:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeSint32(%s, typ2)", bufName, target, bufName))
	case ir.KindUint32:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeVarUint32(%s, typ2)", bufName, target, bufName))
	case ir.KindInt64:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeVarInt64(%s, typ2)", bufName, target, bufName))
	case ir.KindSint64:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeSint64(%s, typ2)", bufName, target, bufName))
	case ir.KindUint64:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeVarUint64(%s, typ2)", bufName, target, bufName))
	case ir.KindFixed32:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeFixedUint32(%s, typ2)", bufName, target, bufName))
	case ir.KindFixed64:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeFixedUint64(%s, typ2)", bufName, target, bufName))
	case ir.KindSfixed32:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeSfixed32(%s, typ2)", bufName, target, bufName))
	case ir.KindSfixed64:
		lines = append(lines, fmt.Sprintf("%s, %s, err2 = ConsumeSfixed64(%s, typ2)", bufName, target, bufName))
	default:
		return nil, fmt.Errorf("unsupported map scalar: %v", kind)
	}
	lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
	return lines, nil
}

func mustGoMapKeyType(kind ir.Kind) string {
	key, err := goMapKeyType(kind)
	if err != nil {
		panic(err)
	}
	return key
}

func mustGoMapValueType(field ir.Field, msgIndex map[string]ir.Message) string {
	val, _, err := goMapValueType(field, msgIndex)
	if err != nil {
		panic(err)
	}
	return val
}

func isGoPackable(kind ir.Kind) bool {
	switch kind {
	case ir.KindString, ir.KindBytes, ir.KindMessage:
		return false
	default:
		return true
	}
}

func goEncodePacked(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	lines = append(lines, "var packed []byte")
	lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
	packedLines, err := goEncodePackedItem("item", field)
	if err != nil {
		return nil, err
	}
	lines = append(lines, packedLines...)
	lines = append(lines, "}")
	lines = append(lines, "if len(packed) > 0 {")
	lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
	lines = append(lines, "b = protowire.AppendBytes(b, packed)")
	lines = append(lines, "}")
	return lines, nil
}

func goEncodePackedItem(name string, field ir.Field) ([]string, error) {
	switch field.Kind {
	case ir.KindBool:
		return []string{
			fmt.Sprintf("if %s {", name),
			"packed = protowire.AppendVarint(packed, 1)",
			"} else {",
			"packed = protowire.AppendVarint(packed, 0)",
			"}",
		}, nil
	case ir.KindFloat:
		return []string{fmt.Sprintf("packed = protowire.AppendFixed32(packed, math.Float32bits(%s))", name)}, nil
	case ir.KindDouble:
		return []string{fmt.Sprintf("packed = protowire.AppendFixed64(packed, math.Float64bits(%s))", name)}, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, uint64(uint32(%s)))", name)}, nil
	case ir.KindUint32:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, uint64(%s))", name)}, nil
	case ir.KindSint32:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, protowire.EncodeZigZag(int64(%s)))", name)}, nil
	case ir.KindInt64:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, uint64(%s))", name)}, nil
	case ir.KindUint64:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, uint64(%s))", name)}, nil
	case ir.KindSint64:
		return []string{fmt.Sprintf("packed = protowire.AppendVarint(packed, protowire.EncodeZigZag(%s))", name)}, nil
	case ir.KindFixed32, ir.KindSfixed32:
		return []string{fmt.Sprintf("packed = protowire.AppendFixed32(packed, uint32(%s))", name)}, nil
	case ir.KindFixed64, ir.KindSfixed64:
		return []string{fmt.Sprintf("packed = protowire.AppendFixed64(packed, uint64(%s))", name)}, nil
	default:
		return nil, fmt.Errorf("unsupported packed kind: %v", field.Kind)
	}
}

func goDecodePacked(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	lines = append(lines, "if typ == protowire.BytesType {")
	lines = append(lines, "var packed []byte")
	lines = append(lines, "b, packed, err = ConsumeBytes(b, typ)")
	lines = append(lines, "if err != nil {", "return nil, err", "}")
	lines = append(lines, "for len(packed) > 0 {")
	itemLines, err := goDecodePackedItem("packed", field)
	if err != nil {
		return nil, err
	}
	lines = append(lines, itemLines...)
	lines = append(lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
	lines = append(lines, "}")
	lines = append(lines, "} else {")
	decodeLines, _, err := goDecodeScalar(field, "item")
	if err != nil {
		return nil, err
	}
	lines = append(lines, decodeLines...)
	lines = append(lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
	lines = append(lines, "}")
	return lines, nil
}

func goDecodePackedItem(bufName string, field ir.Field) ([]string, error) {
	var lines []string
	switch field.Kind {
	case ir.KindBool:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := v != 0")
	case ir.KindFloat:
		lines = append(lines, "var v uint32")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeFixed32(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := math.Float32frombits(v)")
	case ir.KindDouble:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeFixed64(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := math.Float64frombits(v)")
	case ir.KindInt32, ir.KindEnum:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := int32(v)")
	case ir.KindUint32:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := uint32(v)")
	case ir.KindSint32:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := int32(protowire.DecodeZigZag(v))")
	case ir.KindInt64:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := int64(v)")
	case ir.KindUint64:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := v")
	case ir.KindSint64:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeVarint(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := int64(protowire.DecodeZigZag(v))")
	case ir.KindFixed32, ir.KindSfixed32:
		lines = append(lines, "var v uint32")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeFixed32(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := v")
	case ir.KindFixed64, ir.KindSfixed64:
		lines = append(lines, "var v uint64")
		lines = append(lines, "var n int")
		lines = append(lines, fmt.Sprintf("v, n = protowire.ConsumeFixed64(%s)", bufName))
		lines = append(lines, "if err := protowire.ParseError(n); err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = %s[n:]", bufName, bufName))
		lines = append(lines, "item := v")
	default:
		return nil, fmt.Errorf("unsupported packed decode kind: %v", field.Kind)
	}
	return lines, nil
}

func prefixLines(lines []string, prefix string) []string {
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "b =") && prefix != "" {
			out = append(out, strings.Replace(line, "b =", prefix, 1))
		} else if strings.HasPrefix(line, "packed =") && prefix != "" {
			out = append(out, strings.Replace(line, "packed =", prefix, 1))
		} else {
			out = append(out, line)
		}
	}
	return out
}

func buildGoDecodeCases(msg ir.Message, msgIndex map[string]ir.Message) ([]goDecodeCase, bool, bool, error) {
	var cases []goDecodeCase
	needsMsgBytes := false
	needsTmpBytes := false
	for _, field := range msg.Fields {
		c := goDecodeCase{Number: field.Number}
		fieldName := "m." + ir.GoName(field.Name)
		switch {
		case field.IsTimestamp:
			lines, needsMsg, err := goDecodeTimestamp(fieldName, field)
			if err != nil {
				return nil, false, false, err
			}
			if needsMsg {
				needsMsgBytes = true
			}
			c.Lines = append(c.Lines, lines...)
		case field.IsMap:
			lines, msgBytesNeeded, err := goDecodeMap(fieldName, field, msgIndex)
			if err != nil {
				return nil, false, false, err
			}
			if msgBytesNeeded {
				needsMsgBytes = true
			}
			c.Lines = append(c.Lines, lines...)
		case field.IsRepeated && field.Kind == ir.KindMessage:
			needsMsgBytes = true
			msgType := msgIndex[field.MessageFullName].Name
			c.Lines = append(c.Lines, "var err2 error")
			c.Lines = append(c.Lines, "b, msgBytes, err = ConsumeMessage(b, typ)")
			c.Lines = append(c.Lines, "if err != nil {", "return nil, err", "}")
			c.Lines = append(c.Lines, fmt.Sprintf("var item *%s", msgType))
			c.Lines = append(c.Lines, fmt.Sprintf("item, err2 = Decode%s(msgBytes)", msgType))
			c.Lines = append(c.Lines, "if err2 != nil {", "return nil, err2", "}")
			c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
		case field.IsRepeated:
			if field.Kind == ir.KindMessage {
				decodeLines, tmpBytes, err := goDecodeScalar(field, "item")
				if err != nil {
					return nil, false, false, err
				}
				if tmpBytes {
					needsTmpBytes = true
				}
				c.Lines = append(c.Lines, decodeLines...)
				c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
			} else {
				consumeCall, err := goConsumeFunc(field)
				if err != nil {
					return nil, false, false, err
				}
				if field.IsPacked && isGoPackable(field.Kind) {
					elemTyp := goWireType(field.Kind)
					c.Lines = append(c.Lines, fmt.Sprintf("b, %s, err = ConsumeRepeatedCompact(b, typ, %s, %s)", fieldName, elemTyp, consumeCall))
					c.Lines = append(c.Lines, "if err != nil {", "return nil, err", "}")
				} else {
					c.Lines = append(c.Lines, fmt.Sprintf("var item %s", mustGoSliceElemType(field, msgIndex)))
					c.Lines = append(c.Lines, fmt.Sprintf("b, item, err = ConsumeRepeatedElement(b, typ, %s)", consumeCall))
					c.Lines = append(c.Lines, "if err != nil {", "return nil, err", "}")
					c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
				}
			}
		case field.Kind == ir.KindMessage:
			needsMsgBytes = true
			msgType := msgIndex[field.MessageFullName].Name
			c.Lines = append(c.Lines, "b, msgBytes, err = ConsumeMessage(b, typ)")
			c.Lines = append(c.Lines, "if err != nil {", "return nil, err", "}")
			c.Lines = append(c.Lines, fmt.Sprintf("var item *%s", msgType))
			c.Lines = append(c.Lines, fmt.Sprintf("item, err = Decode%s(msgBytes)", msgType))
			c.Lines = append(c.Lines, "if err != nil {", "return nil, err", "}")
			c.Lines = append(c.Lines, fmt.Sprintf("%s = item", fieldName))
		case field.IsOptional:
			decodeLines, err := goDecodeOptionalScalar(field, fieldName)
			if err != nil {
				return nil, false, false, err
			}
			c.Lines = append(c.Lines, decodeLines...)
		default:
			decodeLines, tmpBytes, err := goDecodeScalar(field, fieldName)
			if err != nil {
				return nil, false, false, err
			}
			if tmpBytes {
				needsTmpBytes = true
			}
			c.Lines = append(c.Lines, decodeLines...)
		}
		cases = append(cases, c)
	}
	return cases, needsMsgBytes, needsTmpBytes, nil
}

func goOptionalVarType(field ir.Field) (string, error) {
	switch field.Kind {
	case ir.KindBytes:
		return "[]byte", nil
	case ir.KindString:
		return "string", nil
	case ir.KindBool:
		return "bool", nil
	default:
		t, _, err := goScalarType(field.Kind, false)
		return t, err
	}
}

func goDecodeScalar(field ir.Field, name string) ([]string, bool, error) {
	switch field.Kind {
	case ir.KindString:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeString(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindBytes:
		lines := []string{
			fmt.Sprintf("b, tmpBytes, err = ConsumeBytes(b, typ)"),
			"if err != nil {", "return nil, err", "}",
			fmt.Sprintf("%s = append([]byte(nil), tmpBytes...)", name),
		}
		return lines, true, nil
	case ir.KindBool:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBool(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindFloat:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindDouble:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindSint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindUint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindInt64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindSint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindUint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindFixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindFixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindSfixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed32(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	case ir.KindSfixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed64(b, typ)", name),
			"if err != nil {", "return nil, err", "}",
		}, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported decode kind: %v", field.Kind)
	}
}

func goDecodeTimestamp(fieldName string, field ir.Field) ([]string, bool, error) {
	var lines []string
	if field.IsRepeated {
		if field.TimestampUnit == "wkt" {
			lines = append(lines, "var item time.Time")
			lines = append(lines, "b, item, err = ConsumeTimestamp(b, typ)")
			lines = append(lines, "if err != nil {", "return nil, err", "}")
			lines = append(lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
			return lines, true, nil
		}
		consumeCall, err := goConsumeFunc(ir.Field{Kind: field.Kind})
		if err != nil {
			return nil, false, err
		}
		if field.IsPacked && isGoPackable(field.Kind) {
			lines = append(lines, fmt.Sprintf("var raw []%s", goTimestampRawType(field.Kind)))
			lines = append(lines, fmt.Sprintf("b, raw, err = ConsumeRepeatedCompact(b, typ, %s, %s)", goWireType(field.Kind), consumeCall))
			lines = append(lines, "if err != nil {", "return nil, err", "}")
			lines = append(lines, "for _, v := range raw {")
			lines = append(lines, fmt.Sprintf("%s = append(%s, %s)", fieldName, fieldName, goTimestampFromValue("v", field.TimestampUnit)))
			lines = append(lines, "}")
			return lines, false, nil
		}
		lines = append(lines, fmt.Sprintf("var raw %s", goTimestampRawType(field.Kind)))
		lines = append(lines, fmt.Sprintf("b, raw, err = ConsumeRepeatedElement(b, typ, %s)", consumeCall))
		lines = append(lines, "if err != nil {", "return nil, err", "}")
		lines = append(lines, fmt.Sprintf("%s = append(%s, %s)", fieldName, fieldName, goTimestampFromValue("raw", field.TimestampUnit)))
		return lines, false, nil
	}

	if field.TimestampUnit == "wkt" {
		lines = append(lines, "var item time.Time")
		lines = append(lines, "b, item, err = ConsumeTimestamp(b, typ)")
		lines = append(lines, "if err != nil {", "return nil, err", "}")
		if field.IsOptional {
			lines = append(lines, fmt.Sprintf("%s = &item", fieldName))
		} else {
			lines = append(lines, fmt.Sprintf("%s = item", fieldName))
		}
		return lines, true, nil
	}

	consumeCall, err := goConsumeFunc(ir.Field{Kind: field.Kind})
	if err != nil {
		return nil, false, err
	}
	lines = append(lines, fmt.Sprintf("var raw %s", goTimestampRawType(field.Kind)))
	lines = append(lines, fmt.Sprintf("b, raw, err = %s(b, typ)", consumeCall))
	lines = append(lines, "if err != nil {", "return nil, err", "}")
	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("tmp := %s", goTimestampFromValue("raw", field.TimestampUnit)))
		lines = append(lines, fmt.Sprintf("%s = &tmp", fieldName))
	} else {
		lines = append(lines, fmt.Sprintf("%s = %s", fieldName, goTimestampFromValue("raw", field.TimestampUnit)))
	}
	return lines, false, nil
}

func goTimestampRawType(kind ir.Kind) string {
	if kind == ir.KindInt32 {
		return "int32"
	}
	return "int64"
}

func goTimestampFromValue(name string, unit string) string {
	if unit == "milliseconds" {
		return "time.UnixMilli(int64(" + name + "))"
	}
	return "time.Unix(int64(" + name + "), 0)"
}

func goDecodeOptionalScalar(field ir.Field, fieldName string) ([]string, error) {
	switch field.Kind {
	case ir.KindString:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeStringOpt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindBytes:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBytesOpt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindBool:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBoolOpt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindFloat:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindDouble:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindSint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindUint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindInt64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindSint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindUint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindFixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindFixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindSfixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed32Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	case ir.KindSfixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed64Opt(b, typ)", fieldName),
			"if err != nil {", "return nil, err", "}",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported optional decode kind: %v", field.Kind)
	}
}

func goConsumeFunc(field ir.Field) (string, error) {
	switch field.Kind {
	case ir.KindString:
		return "ConsumeString", nil
	case ir.KindBytes:
		return "ConsumeBytesCopy", nil
	case ir.KindBool:
		return "ConsumeBool", nil
	case ir.KindFloat:
		return "ConsumeFloat32", nil
	case ir.KindDouble:
		return "ConsumeFloat64", nil
	case ir.KindInt32, ir.KindEnum:
		return "ConsumeVarInt32", nil
	case ir.KindSint32:
		return "ConsumeSint32", nil
	case ir.KindUint32:
		return "ConsumeVarUint32", nil
	case ir.KindInt64:
		return "ConsumeVarInt64", nil
	case ir.KindSint64:
		return "ConsumeSint64", nil
	case ir.KindUint64:
		return "ConsumeVarUint64", nil
	case ir.KindFixed32:
		return "ConsumeFixedUint32", nil
	case ir.KindFixed64:
		return "ConsumeFixedUint64", nil
	case ir.KindSfixed32:
		return "ConsumeSfixed32", nil
	case ir.KindSfixed64:
		return "ConsumeSfixed64", nil
	default:
		return "", fmt.Errorf("unsupported consume kind: %v", field.Kind)
	}
}

func goConsumeMapValueFunc(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	if field.IsTimestamp {
		return "", fmt.Errorf("timestamp not supported in map values")
	}
	switch field.MapValueKind {
	case ir.KindMessage:
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		return "ConsumeMessageDecorator(Decode" + msg.Name + ")", nil
	case ir.KindBytes:
		return "ConsumeBytes", nil
	default:
		return goConsumeFunc(ir.Field{Kind: field.MapValueKind})
	}
}

func goWireType(kind ir.Kind) string {
	switch kind {
	case ir.KindString, ir.KindBytes, ir.KindMessage:
		return "protowire.BytesType"
	case ir.KindFixed32, ir.KindSfixed32, ir.KindFloat:
		return "protowire.Fixed32Type"
	case ir.KindFixed64, ir.KindSfixed64, ir.KindDouble:
		return "protowire.Fixed64Type"
	default:
		return "protowire.VarintType"
	}
}

func mustGoSliceElemType(field ir.Field, msgIndex map[string]ir.Message) string {
	if field.Kind == ir.KindMessage {
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			panic(fmt.Errorf("unknown message type: %s", field.MessageFullName))
		}
		return "*" + msg.Name
	}
	if field.Kind == ir.KindBytes {
		return "[]byte"
	}
	name, _, err := goScalarType(field.Kind, false)
	if err != nil {
		panic(err)
	}
	return name
}

func indexMessages(files []ir.File) map[string]ir.Message {
	index := make(map[string]ir.Message)
	for _, file := range files {
		for _, msg := range file.Messages {
			index[string(msg.FullName)] = msg
		}
	}
	return index
}

func loadUtilSource(pkg string) ([]byte, error) {
	srcPath := filepath.Clean("../jnotes/app/protowireu/protowireu.go")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read protowireu source: %w", err)
	}
	updated := strings.Replace(string(content), "package protowireu", "package "+pkg, 1)
	trimmed := strings.TrimSpace(updated)
	if !strings.HasPrefix(trimmed, "package ") {
		updated = "package " + pkg + "\n\n" + updated
	}
	if strings.Contains(updated, "import (") && !strings.Contains(updated, "\"time\"") {
		updated = strings.Replace(updated, "import (\n", "import (\n\t\"time\"\n", 1)
	}
	updated += "\n\n" + utilExtra
	return []byte(updated), nil
}

const utilExtra = `
func EncodeTimestamp(t time.Time) []byte {
	if t.IsZero() {
		return nil
	}
	var b []byte
	seconds := t.Unix()
	nanos := int32(t.Nanosecond())
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(seconds))
	if nanos != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(nanos)))
	}
	return b
}

func DecodeTimestamp(b []byte) (time.Time, error) {
	var seconds int64
	var nanos int32
	for len(b) > 0 {
		var num protowire.Number
		var typ protowire.Type
		var err error
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return time.Time{}, err
		}
		switch num {
		case 1:
			b, seconds, err = ConsumeVarInt64(b, typ)
			if err != nil {
				return time.Time{}, err
			}
		case 2:
			var n int32
			b, n, err = ConsumeVarInt32(b, typ)
			if err != nil {
				return time.Time{}, err
			}
			nanos = n
		default:
			b, err = SkipFieldValue(b, num, typ)
			if err != nil {
				return time.Time{}, err
			}
		}
	}
	return time.Unix(seconds, int64(nanos)), nil
}

func ConsumeTimestamp(b []byte, typ protowire.Type) ([]byte, time.Time, error) {
	var msgBytes []byte
	var err error
	b, msgBytes, err = ConsumeMessage(b, typ)
	if err != nil {
		return nil, time.Time{}, err
	}
	msg, err := DecodeTimestamp(msgBytes)
	if err != nil {
		return nil, time.Time{}, err
	}
	return b, msg, nil
}

func ConsumeMapEntry[K comparable, V any](b []byte, typ protowire.Type, consumeK func([]byte, protowire.Type) ([]byte, K, error), consumeV func([]byte, protowire.Type) ([]byte, V, error)) ([]byte, K, V, error) {
	var key K
	var value V
	if typ != protowire.BytesType {
		return nil, key, value, errInvalidWireType
	}
	var entryBytes []byte
	var err error
	b, entryBytes, err = ConsumeMessage(b, typ)
	if err != nil {
		return nil, key, value, err
	}
	for len(entryBytes) > 0 {
		var num protowire.Number
		var t protowire.Type
		var err2 error
		entryBytes, num, t, err2 = ConsumeTag(entryBytes)
		if err2 != nil {
			return nil, key, value, err2
		}
		switch num {
		case 1:
			entryBytes, key, err2 = consumeK(entryBytes, t)
			if err2 != nil {
				return nil, key, value, err2
			}
		case 2:
			entryBytes, value, err2 = consumeV(entryBytes, t)
			if err2 != nil {
				return nil, key, value, err2
			}
		default:
			entryBytes, err2 = SkipFieldValue(entryBytes, num, t)
			if err2 != nil {
				return nil, key, value, err2
			}
		}
	}
	return b, key, value, nil
}

func ConsumeMessageDecorator[T any](decodeFunc func([]byte) (T, error)) func(b []byte, typ protowire.Type) ([]byte, T, error) {
	return func(b []byte, typ protowire.Type) ([]byte, T, error) {
		var zeroV T
		var msgBytes []byte
		var err error
		b, msgBytes, err = ConsumeMessage(b, typ)
		if err != nil {
			return nil, zeroV, err
		}
		msg, err := decodeFunc(msgBytes)
		if err != nil {
			return nil, zeroV, err
		}
		return b, msg, nil
	}
}

func ConsumeRepeatedElement[T any](b []byte, typ protowire.Type, consume func([]byte, protowire.Type) ([]byte, T, error)) ([]byte, T, error) {
	var item T
	var err error
	b, item, err = consume(b, typ)
	if err != nil {
		return nil, item, err
	}
	return b, item, nil
}

func ConsumeRepeatedCompact[T any](b []byte, typ protowire.Type, elemTyp protowire.Type, consume func([]byte, protowire.Type) ([]byte, T, error)) ([]byte, []T, error) {
	if typ != protowire.BytesType || elemTyp == protowire.BytesType {
		return nil, nil, errInvalidWireType
	}
	var packed []byte
	var err error
	b, packed, err = ConsumeBytes(b, typ)
	if err != nil {
		return nil, nil, err
	}
	var items []T
	for len(packed) > 0 {
		var v T
		packed, v, err = consume(packed, elemTyp)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, v)
	}
	return b, items, nil
}

func ConsumeVarInt32Opt(b []byte, typ protowire.Type) ([]byte, *int32, error) {
	var v int32
	var err error
	b, v, err = ConsumeVarInt32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeVarInt64Opt(b []byte, typ protowire.Type) ([]byte, *int64, error) {
	var v int64
	var err error
	b, v, err = ConsumeVarInt64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeVarUint32Opt(b []byte, typ protowire.Type) ([]byte, *uint32, error) {
	var v uint32
	var err error
	b, v, err = ConsumeVarUint32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeVarUint64Opt(b []byte, typ protowire.Type) ([]byte, *uint64, error) {
	var v uint64
	var err error
	b, v, err = ConsumeVarUint64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeSint32Opt(b []byte, typ protowire.Type) ([]byte, *int32, error) {
	var v int32
	var err error
	b, v, err = ConsumeSint32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeSint64Opt(b []byte, typ protowire.Type) ([]byte, *int64, error) {
	var v int64
	var err error
	b, v, err = ConsumeSint64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeFixedUint32Opt(b []byte, typ protowire.Type) ([]byte, *uint32, error) {
	var v uint32
	var err error
	b, v, err = ConsumeFixedUint32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeFixedUint64Opt(b []byte, typ protowire.Type) ([]byte, *uint64, error) {
	var v uint64
	var err error
	b, v, err = ConsumeFixedUint64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeSfixed32Opt(b []byte, typ protowire.Type) ([]byte, *int32, error) {
	var v int32
	var err error
	b, v, err = ConsumeSfixed32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeSfixed64Opt(b []byte, typ protowire.Type) ([]byte, *int64, error) {
	var v int64
	var err error
	b, v, err = ConsumeSfixed64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeFloat32Opt(b []byte, typ protowire.Type) ([]byte, *float32, error) {
	var v float32
	var err error
	b, v, err = ConsumeFloat32(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeFloat64Opt(b []byte, typ protowire.Type) ([]byte, *float64, error) {
	var v float64
	var err error
	b, v, err = ConsumeFloat64(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeBoolOpt(b []byte, typ protowire.Type) ([]byte, *bool, error) {
	var v bool
	var err error
	b, v, err = ConsumeBool(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeStringOpt(b []byte, typ protowire.Type) ([]byte, *string, error) {
	var v string
	var err error
	b, v, err = ConsumeString(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, &v, nil
}

func ConsumeBytesOpt(b []byte, typ protowire.Type) ([]byte, *[]byte, error) {
	var v []byte
	var err error
	b, v, err = ConsumeBytes(b, typ)
	if err != nil {
		return nil, nil, err
	}
	copyBytes := append([]byte(nil), v...)
	return b, &copyBytes, nil
}

func ConsumeBytesCopy(b []byte, typ protowire.Type) ([]byte, []byte, error) {
	var v []byte
	var err error
	b, v, err = ConsumeBytes(b, typ)
	if err != nil {
		return nil, nil, err
	}
	return b, append([]byte(nil), v...), nil
}
`
