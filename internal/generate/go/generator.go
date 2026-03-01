package gogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

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
	Name    string
	Type    string
	JSONTag string
}

type goDecodeCase struct {
	Number int
	Lines  []string
}

func buildGoFileData(file ir.File, msgIndex map[string]ir.Message, pkg string) (goFileData, error) {
	data := goFileData{Package: pkg}
	var usesTime bool
	for _, msg := range file.Messages {
		goMsg, _, timeNeeded, err := buildGoMessage(msg, msgIndex)
		if err != nil {
			return goFileData{}, err
		}
		if timeNeeded {
			usesTime = true
		}
		data.Messages = append(data.Messages, goMsg)
	}
	imports := []string{
		"google.golang.org/protobuf/encoding/protowire",
	}
	if usesTime {
		imports = append([]string{"time"}, imports...)
	}
	data.Imports = imports
	return data, nil
}

func buildGoMessage(msg ir.Message, msgIndex map[string]ir.Message) (goMessage, bool, bool, error) {
	out := goMessage{Name: msg.Name}
	var usesTime bool
	for _, field := range msg.Fields {
		goType, _, err := goFieldType(field, msgIndex)
		if err != nil {
			return goMessage{}, false, false, err
		}
		if field.IsTimestamp {
			usesTime = true
		}
		if field.IsDuration {
			usesTime = true
		}
		out.Fields = append(out.Fields, goField{
			Name:    ir.GoName(field.Name),
			Type:    goType,
			JSONTag: toSnakeCase(field.Name),
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

	return out, false, usesTime, nil
}

func toSnakeCase(name string) string {
	if name == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(name) + 4)
	for i, r := range name {
		if r == '-' {
			out.WriteByte('_')
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
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
	if field.IsDuration {
		base := "time.Duration"
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
		case field.IsDuration:
			durLines, err := goEncodeDuration(fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, durLines...)
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
				repeatedLines, err := goEncodeRepeated(fieldName, field)
				if err != nil {
					return nil, err
				}
				lines = append(lines, repeatedLines...)
			}
		case field.Kind == ir.KindMessage:
			lines = append(lines, fmt.Sprintf("if %s != nil {", fieldName))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendTag(b, %d, protowire.BytesType)", field.Number))
			lines = append(lines, fmt.Sprintf("b = protowire.AppendBytes(b, %s.Encode())", fieldName))
			lines = append(lines, "}")
		case field.IsOptional:
			encodeLines, err := goEncodeOptionalField(fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, encodeLines...)
		default:
			encodeLines, err := goEncodeField(fieldName, field)
			if err != nil {
				return nil, err
			}
			lines = append(lines, encodeLines...)
		}
	}
	return lines, nil
}

func goEncodeField(name string, field ir.Field) ([]string, error) {
	helper, err := goAppendHelperName(field.Kind, false)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("b = %s(b, %s, %d)", helper, name, field.Number)}, nil
}

func goEncodeRepeated(fieldName string, field ir.Field) ([]string, error) {
	helper, err := goAppendHelperName(field.Kind, false)
	if err != nil {
		return nil, err
	}
	lines := []string{fmt.Sprintf("b = AppendRepeated(b, %s, AppendFieldDecorator(%s, %d))", fieldName, helper, field.Number)}
	return lines, nil
}

func goEncodeOptionalField(name string, field ir.Field) ([]string, error) {
	if field.Kind == ir.KindBytes {
		return []string{
			fmt.Sprintf("if %s != nil {", name),
			fmt.Sprintf("b = AppendBytesField(b, *%s, %d)", name, field.Number),
			"}",
		}, nil
	}
	helper, err := goAppendHelperName(field.Kind, true)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("b = %s(b, %s, %d)", helper, name, field.Number)}, nil
}

func goAppendHelperName(kind ir.Kind, optional bool) (string, error) {
	var base string
	switch kind {
	case ir.KindString:
		base = "AppendStringField"
	case ir.KindBytes:
		base = "AppendBytesField"
	case ir.KindBool:
		base = "AppendBoolField"
	case ir.KindFloat:
		base = "AppendFloat32Field"
	case ir.KindDouble:
		base = "AppendFloat64Field"
	case ir.KindInt32, ir.KindEnum:
		base = "AppendInt32Field"
	case ir.KindSint32:
		base = "AppendSint32Field"
	case ir.KindUint32:
		base = "AppendUint32Field"
	case ir.KindInt64:
		base = "AppendInt64Field"
	case ir.KindSint64:
		base = "AppendSint64Field"
	case ir.KindUint64:
		base = "AppendUint64Field"
	case ir.KindFixed32:
		base = "AppendFixed32Field"
	case ir.KindFixed64:
		base = "AppendFixed64Field"
	case ir.KindSfixed32:
		base = "AppendSfixed32Field"
	case ir.KindSfixed64:
		base = "AppendSfixed64Field"
	default:
		return "", fmt.Errorf("unsupported append kind: %v", kind)
	}
	if optional {
		if base == "AppendBytesField" {
			return "", fmt.Errorf("optional bytes append helper not supported")
		}
		return base + "Opt", nil
	}
	return base, nil
}

func goEncodeTimestamp(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	if field.IsRepeated {
		lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
		lines = append(lines, "if item.IsZero() {", "continue", "}")
		if field.TimestampUnit == "wkt" {
			lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeTimestamp(item), %d)", field.Number))
		} else {
			valueExpr := goTimestampValue("item", field.TimestampUnit, field.Kind)
			lines = append(lines, fmt.Sprintf("b = AppendVarIntField(b, %s, %d)", valueExpr, field.Number))
		}
		lines = append(lines, "}")
		return lines, nil
	}

	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("if %s != nil && !%s.IsZero() {", fieldName, fieldName))
		if field.TimestampUnit == "wkt" {
			lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeTimestamp(*%s), %d)", fieldName, field.Number))
		} else {
			valueExpr := goTimestampValue("*"+fieldName, field.TimestampUnit, field.Kind)
			lines = append(lines, fmt.Sprintf("b = AppendVarIntField(b, %s, %d)", valueExpr, field.Number))
		}
		lines = append(lines, "}")
		return lines, nil
	}

	lines = append(lines, fmt.Sprintf("if !%s.IsZero() {", fieldName))
	if field.TimestampUnit == "wkt" {
		lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeTimestamp(%s), %d)", fieldName, field.Number))
	} else {
		valueExpr := goTimestampValue(fieldName, field.TimestampUnit, field.Kind)
		lines = append(lines, fmt.Sprintf("b = AppendVarIntField(b, %s, %d)", valueExpr, field.Number))
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

func goEncodeDuration(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	if field.IsRepeated {
		lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
		lines = append(lines, "if item == 0 {", "continue", "}")
		lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeDuration(item), %d)", field.Number))
		lines = append(lines, "}")
		return lines, nil
	}

	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("if %s != nil && *%s != 0 {", fieldName, fieldName))
		lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeDuration(*%s), %d)", fieldName, field.Number))
		lines = append(lines, "}")
		return lines, nil
	}

	lines = append(lines, fmt.Sprintf("if %s != 0 {", fieldName))
	lines = append(lines, fmt.Sprintf("b = AppendBytesField(b, EncodeDuration(%s), %d)", fieldName, field.Number))
	lines = append(lines, "}")
	return lines, nil
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
	mapValueType := mustGoMapValueType(field, msgIndex)
	keyHelper, err := goAppendHelperName(field.MapKeyKind, false)
	if err != nil {
		return nil, err
	}
	keyExpr := fmt.Sprintf("AppendFieldDecorator(%s, 1)", keyHelper)
	var valueExpr string
	if field.MapValueKind == ir.KindMessage {
		valueExpr = fmt.Sprintf("AppendMessageFieldDecorator[%s](2)", mapValueType)
	} else {
		valHelper, err := goAppendHelperName(field.MapValueKind, false)
		if err != nil {
			return nil, err
		}
		valueExpr = fmt.Sprintf("AppendFieldDecorator(%s, 2)", valHelper)
	}
	lines = append(lines, fmt.Sprintf("b = AppendMap(b, %s, %d, %s, %s)", fieldName, field.Number, keyExpr, valueExpr))
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
	lines = append(lines, fmt.Sprintf("if %s == nil {", fieldName))
	lines = append(lines, fmt.Sprintf("%s = make(map[%s]%s)", fieldName, mustGoMapKeyType(field.MapKeyKind), mustGoMapValueType(field, msgIndex)))
	lines = append(lines, "}")
	lines = append(lines, fmt.Sprintf("b, err = ConsumeMapEntry(b, typ, %s, %s, %s)", fieldName, keyConsume, valConsume))
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
	compactHelper, err := goAppendCompactHelperName(field.Kind)
	if err != nil {
		return nil, err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("b = AppendRepeatedCompact(b, %s, %d, AppendCompactDecorator(%s))", fieldName, field.Number, compactHelper))
	return lines, nil
}

func goAppendCompactHelperName(kind ir.Kind) (string, error) {
	switch kind {
	case ir.KindBool:
		return "AppendBoolCompact", nil
	case ir.KindFloat:
		return "AppendFloat32Compact", nil
	case ir.KindDouble:
		return "AppendFloat64Compact", nil
	case ir.KindInt32, ir.KindEnum:
		return "AppendInt32Compact", nil
	case ir.KindUint32:
		return "AppendUint32Compact", nil
	case ir.KindSint32:
		return "AppendSint32Compact", nil
	case ir.KindInt64:
		return "AppendInt64Compact", nil
	case ir.KindUint64:
		return "AppendUint64Compact", nil
	case ir.KindSint64:
		return "AppendSint64Compact", nil
	case ir.KindFixed32, ir.KindSfixed32:
		if kind == ir.KindSfixed32 {
			return "AppendSfixed32Compact", nil
		}
		return "AppendFixed32Compact", nil
	case ir.KindFixed64, ir.KindSfixed64:
		if kind == ir.KindSfixed64 {
			return "AppendSfixed64Compact", nil
		}
		return "AppendFixed64Compact", nil
	default:
		return "", fmt.Errorf("unsupported packed append kind: %v", kind)
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
		case field.IsDuration:
			lines, err := goDecodeDuration(fieldName, field)
			if err != nil {
				return nil, false, false, err
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
			c.Lines = append(c.Lines, "b, msgBytes, err = ConsumeMessage(b, typ)")
			c.Lines = append(c.Lines, "if err == nil {")
			c.Lines = append(c.Lines, fmt.Sprintf("var item *%s", msgType))
			c.Lines = append(c.Lines, fmt.Sprintf("item, err = Decode%s(msgBytes)", msgType))
			c.Lines = append(c.Lines, "if err == nil {")
			c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
			c.Lines = append(c.Lines, "}")
			c.Lines = append(c.Lines, "}")
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
				} else {
					c.Lines = append(c.Lines, fmt.Sprintf("var item %s", mustGoSliceElemType(field, msgIndex)))
					c.Lines = append(c.Lines, fmt.Sprintf("b, item, err = ConsumeRepeatedElement(b, typ, %s)", consumeCall))
					c.Lines = append(c.Lines, "if err == nil {")
					c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
					c.Lines = append(c.Lines, "}")
				}
			}
		case field.Kind == ir.KindMessage:
			needsMsgBytes = true
			msgType := msgIndex[field.MessageFullName].Name
			c.Lines = append(c.Lines, "b, msgBytes, err = ConsumeMessage(b, typ)")
			c.Lines = append(c.Lines, "if err == nil {")
			c.Lines = append(c.Lines, fmt.Sprintf("var item *%s", msgType))
			c.Lines = append(c.Lines, fmt.Sprintf("item, err = Decode%s(msgBytes)", msgType))
			c.Lines = append(c.Lines, "if err == nil {")
			c.Lines = append(c.Lines, fmt.Sprintf("%s = item", fieldName))
			c.Lines = append(c.Lines, "}")
			c.Lines = append(c.Lines, "}")
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
		}, false, nil
	case ir.KindBytes:
		lines := []string{
			fmt.Sprintf("b, %s, err = ConsumeBytesCopy(b, typ)", name),
		}
		return lines, false, nil
	case ir.KindBool:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBool(b, typ)", name),
		}, false, nil
	case ir.KindFloat:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat32(b, typ)", name),
		}, false, nil
	case ir.KindDouble:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat64(b, typ)", name),
		}, false, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt32(b, typ)", name),
		}, false, nil
	case ir.KindSint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint32(b, typ)", name),
		}, false, nil
	case ir.KindUint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint32(b, typ)", name),
		}, false, nil
	case ir.KindInt64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt64(b, typ)", name),
		}, false, nil
	case ir.KindSint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint64(b, typ)", name),
		}, false, nil
	case ir.KindUint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint64(b, typ)", name),
		}, false, nil
	case ir.KindFixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint32(b, typ)", name),
		}, false, nil
	case ir.KindFixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint64(b, typ)", name),
		}, false, nil
	case ir.KindSfixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed32(b, typ)", name),
		}, false, nil
	case ir.KindSfixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed64(b, typ)", name),
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
			lines = append(lines, "if err == nil {")
			lines = append(lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
			lines = append(lines, "}")
			return lines, true, nil
		}
		consumeCall, err := goConsumeFunc(ir.Field{Kind: field.Kind})
		if err != nil {
			return nil, false, err
		}
		if field.IsPacked && isGoPackable(field.Kind) {
			lines = append(lines, fmt.Sprintf("var raw []%s", goTimestampRawType(field.Kind)))
			lines = append(lines, fmt.Sprintf("b, raw, err = ConsumeRepeatedCompact(b, typ, %s, %s)", goWireType(field.Kind), consumeCall))
			lines = append(lines, "if err == nil {")
			lines = append(lines, "for _, v := range raw {")
			lines = append(lines, fmt.Sprintf("%s = append(%s, %s)", fieldName, fieldName, goTimestampFromValue("v", field.TimestampUnit)))
			lines = append(lines, "}")
			lines = append(lines, "}")
			return lines, false, nil
		}
		lines = append(lines, fmt.Sprintf("var raw %s", goTimestampRawType(field.Kind)))
		lines = append(lines, fmt.Sprintf("b, raw, err = ConsumeRepeatedElement(b, typ, %s)", consumeCall))
		lines = append(lines, "if err == nil {")
		lines = append(lines, fmt.Sprintf("%s = append(%s, %s)", fieldName, fieldName, goTimestampFromValue("raw", field.TimestampUnit)))
		lines = append(lines, "}")
		return lines, false, nil
	}

	if field.TimestampUnit == "wkt" {
		lines = append(lines, "var item time.Time")
		lines = append(lines, "b, item, err = ConsumeTimestamp(b, typ)")
		lines = append(lines, "if err == nil {")
		if field.IsOptional {
			lines = append(lines, fmt.Sprintf("%s = &item", fieldName))
		} else {
			lines = append(lines, fmt.Sprintf("%s = item", fieldName))
		}
		lines = append(lines, "}")
		return lines, true, nil
	}

	consumeCall, err := goConsumeFunc(ir.Field{Kind: field.Kind})
	if err != nil {
		return nil, false, err
	}
	lines = append(lines, fmt.Sprintf("var raw %s", goTimestampRawType(field.Kind)))
	lines = append(lines, fmt.Sprintf("b, raw, err = %s(b, typ)", consumeCall))
	lines = append(lines, "if err == nil {")
	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("tmp := %s", goTimestampFromValue("raw", field.TimestampUnit)))
		lines = append(lines, fmt.Sprintf("%s = &tmp", fieldName))
	} else {
		lines = append(lines, fmt.Sprintf("%s = %s", fieldName, goTimestampFromValue("raw", field.TimestampUnit)))
	}
	lines = append(lines, "}")
	return lines, false, nil
}

func goDecodeDuration(fieldName string, field ir.Field) ([]string, error) {
	var lines []string
	if field.IsRepeated {
		lines = append(lines, "var item time.Duration")
		lines = append(lines, "b, item, err = ConsumeDuration(b, typ)")
		lines = append(lines, "if err == nil {")
		lines = append(lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
		lines = append(lines, "}")
		return lines, nil
	}

	lines = append(lines, "var item time.Duration")
	lines = append(lines, "b, item, err = ConsumeDuration(b, typ)")
	lines = append(lines, "if err == nil {")
	if field.IsOptional {
		lines = append(lines, fmt.Sprintf("%s = &item", fieldName))
	} else {
		lines = append(lines, fmt.Sprintf("%s = item", fieldName))
	}
	lines = append(lines, "}")
	return lines, nil
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
		}, nil
	case ir.KindBytes:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBytesOpt(b, typ)", fieldName),
		}, nil
	case ir.KindBool:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeBoolOpt(b, typ)", fieldName),
		}, nil
	case ir.KindFloat:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindDouble:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFloat64Opt(b, typ)", fieldName),
		}, nil
	case ir.KindInt32, ir.KindEnum:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindSint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindUint32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindInt64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarInt64Opt(b, typ)", fieldName),
		}, nil
	case ir.KindSint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSint64Opt(b, typ)", fieldName),
		}, nil
	case ir.KindUint64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeVarUint64Opt(b, typ)", fieldName),
		}, nil
	case ir.KindFixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindFixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeFixedUint64Opt(b, typ)", fieldName),
		}, nil
	case ir.KindSfixed32:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed32Opt(b, typ)", fieldName),
		}, nil
	case ir.KindSfixed64:
		return []string{
			fmt.Sprintf("b, %s, err = ConsumeSfixed64Opt(b, typ)", fieldName),
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
	if field.IsDuration {
		return "time.Duration"
	}
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

func EncodeDuration(d time.Duration) []byte {
	if d == 0 {
		return nil
	}
	var b []byte
	seconds := int64(d / time.Second)
	nanos := int32(d % time.Second)
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(seconds))
	if nanos != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(nanos)))
	}
	return b
}

func DecodeDuration(b []byte) (time.Duration, error) {
	var seconds int64
	var nanos int32
	for len(b) > 0 {
		var num protowire.Number
		var typ protowire.Type
		var err error
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return 0, err
		}
		switch num {
		case 1:
			b, seconds, err = ConsumeVarInt64(b, typ)
			if err != nil {
				return 0, err
			}
		case 2:
			var n int32
			b, n, err = ConsumeVarInt32(b, typ)
			if err != nil {
				return 0, err
			}
			nanos = n
		default:
			b, err = SkipFieldValue(b, num, typ)
			if err != nil {
				return 0, err
			}
		}
	}
	return time.Duration(seconds)*time.Second + time.Duration(nanos), nil
}

func ConsumeDuration(b []byte, typ protowire.Type) ([]byte, time.Duration, error) {
	var msgBytes []byte
	var err error
	b, msgBytes, err = ConsumeMessage(b, typ)
	if err != nil {
		return nil, 0, err
	}
	msg, err := DecodeDuration(msgBytes)
	if err != nil {
		return nil, 0, err
	}
	return b, msg, nil
}

func ConsumeMapEntry[K comparable, V any](b []byte, typ protowire.Type, m map[K]V, consumeK func([]byte, protowire.Type) ([]byte, K, error), consumeV func([]byte, protowire.Type) ([]byte, V, error)) ([]byte, error) {
	var key K
	var value V
	if typ != protowire.BytesType {
		return nil, errInvalidWireType
	}
	var entryBytes []byte
	var err error
	b, entryBytes, err = ConsumeMessage(b, typ)
	if err != nil {
		return nil, err
	}
	for len(entryBytes) > 0 {
		var num protowire.Number
		var t protowire.Type
		var err2 error
		entryBytes, num, t, err2 = ConsumeTag(entryBytes)
		if err2 != nil {
			return nil, err2
		}
		switch num {
		case 1:
			entryBytes, key, err2 = consumeK(entryBytes, t)
			if err2 != nil {
				return nil, err2
			}
		case 2:
			entryBytes, value, err2 = consumeV(entryBytes, t)
			if err2 != nil {
				return nil, err2
			}
		default:
			entryBytes, err2 = SkipFieldValue(entryBytes, num, t)
			if err2 != nil {
				return nil, err2
			}
		}
	}
	m[key] = value
	return b, nil
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

func AppendVarIntField(b []byte, v uint64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func AppendVarIntFieldOpt(b []byte, v *uint64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, *v)
}

func AppendStringField(b []byte, v string, num protowire.Number) []byte {
	if v == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, []byte(v))
}

func AppendStringFieldOpt(b []byte, v *string, num protowire.Number) []byte {
	if v == nil || *v == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, []byte(*v))
}

func AppendBytesField(b []byte, v []byte, num protowire.Number) []byte {
	if len(v) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

func AppendBoolField(b []byte, v bool, num protowire.Number) []byte {
	if !v {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, 1)
}

func AppendBoolFieldOpt(b []byte, v *bool, num protowire.Number) []byte {
	if v == nil || !*v {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, 1)
}

func AppendFloat32Field(b []byte, v float32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, math.Float32bits(v))
}

func AppendFloat32FieldOpt(b []byte, v *float32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, math.Float32bits(*v))
}

func AppendFloat64Field(b []byte, v float64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, math.Float64bits(v))
}

func AppendFloat64FieldOpt(b []byte, v *float64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, math.Float64bits(*v))
}

func AppendInt32Field(b []byte, v int32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(uint32(v)))
}

func AppendInt32FieldOpt(b []byte, v *int32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(uint32(*v)))
}

func AppendUint32Field(b []byte, v uint32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(v))
}

func AppendUint32FieldOpt(b []byte, v *uint32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(*v))
}

func AppendSint32Field(b []byte, v int32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, protowire.EncodeZigZag(int64(v)))
}

func AppendSint32FieldOpt(b []byte, v *int32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, protowire.EncodeZigZag(int64(*v)))
}

func AppendInt64Field(b []byte, v int64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(v))
}

func AppendInt64FieldOpt(b []byte, v *int64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(*v))
}

func AppendUint64Field(b []byte, v uint64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func AppendUint64FieldOpt(b []byte, v *uint64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, *v)
}

func AppendSint64Field(b []byte, v int64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, protowire.EncodeZigZag(v))
}

func AppendSint64FieldOpt(b []byte, v *int64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, protowire.EncodeZigZag(*v))
}

func AppendFixed32Field(b []byte, v uint32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, v)
}

func AppendFixed32FieldOpt(b []byte, v *uint32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, *v)
}

func AppendFixed64Field(b []byte, v uint64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, v)
}

func AppendFixed64FieldOpt(b []byte, v *uint64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, *v)
}

func AppendSfixed32Field(b []byte, v int32, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, uint32(v))
}

func AppendSfixed32FieldOpt(b []byte, v *int32, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed32Type)
	return protowire.AppendFixed32(b, uint32(*v))
}

func AppendSfixed64Field(b []byte, v int64, num protowire.Number) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, uint64(v))
}

func AppendSfixed64FieldOpt(b []byte, v *int64, num protowire.Number) []byte {
	if v == nil || *v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)
	return protowire.AppendFixed64(b, uint64(*v))
}

func AppendFieldDecorator[T any](appendField func([]byte, T, protowire.Number) []byte, num protowire.Number) func([]byte, T) []byte {
	return func(b []byte, value T) []byte {
		return appendField(b, value, num)
	}
}

func AppendCompactDecorator[T any](appendCompact func([]byte, T) []byte) func([]byte, T) []byte {
	return func(b []byte, value T) []byte {
		return appendCompact(b, value)
	}
}

func AppendBoolCompact(b []byte, v bool) []byte {
	if v {
		return protowire.AppendVarint(b, 1)
	}
	return protowire.AppendVarint(b, 0)
}

func AppendFloat32Compact(b []byte, v float32) []byte {
	return protowire.AppendFixed32(b, math.Float32bits(v))
}

func AppendFloat64Compact(b []byte, v float64) []byte {
	return protowire.AppendFixed64(b, math.Float64bits(v))
}

func AppendInt32Compact(b []byte, v int32) []byte {
	return protowire.AppendVarint(b, uint64(uint32(v)))
}

func AppendUint32Compact(b []byte, v uint32) []byte {
	return protowire.AppendVarint(b, uint64(v))
}

func AppendSint32Compact(b []byte, v int32) []byte {
	return protowire.AppendVarint(b, protowire.EncodeZigZag(int64(v)))
}

func AppendInt64Compact(b []byte, v int64) []byte {
	return protowire.AppendVarint(b, uint64(v))
}

func AppendUint64Compact(b []byte, v uint64) []byte {
	return protowire.AppendVarint(b, v)
}

func AppendSint64Compact(b []byte, v int64) []byte {
	return protowire.AppendVarint(b, protowire.EncodeZigZag(v))
}

func AppendFixed32Compact(b []byte, v uint32) []byte {
	return protowire.AppendFixed32(b, v)
}

func AppendSfixed32Compact(b []byte, v int32) []byte {
	return protowire.AppendFixed32(b, uint32(v))
}

func AppendFixed64Compact(b []byte, v uint64) []byte {
	return protowire.AppendFixed64(b, v)
}

func AppendSfixed64Compact(b []byte, v int64) []byte {
	return protowire.AppendFixed64(b, uint64(v))
}

type Encodable interface {
	Encode() []byte
}

func AppendMessageFieldDecorator[T Encodable](num protowire.Number) func([]byte, T) []byte {
	return func(b []byte, value T) []byte {
		return AppendBytesField(b, value.Encode(), num)
	}
}

func AppendRepeated[T any](b []byte, values []T, appendValue func([]byte, T) []byte) []byte {
	for _, value := range values {
		b = appendValue(b, value)
	}
	return b
}

func AppendRepeatedCompact[T any](b []byte, values []T, num protowire.Number, appendValue func([]byte, T) []byte) []byte {
	var packed []byte
	for _, value := range values {
		packed = appendValue(packed, value)
	}
	if len(packed) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, packed)
}

func AppendMap[K comparable, V any](
	b []byte,
	m map[K]V,
	num protowire.Number,
	appendKey func([]byte, K) []byte,
	appendValue func([]byte, V) []byte,
) []byte {
	for key, value := range m {
		var entry []byte
		entry = appendKey(entry, key)
		entry = appendValue(entry, value)
		b = protowire.AppendTag(b, num, protowire.BytesType)
		b = protowire.AppendBytes(b, entry)
	}
	return b
}
`
