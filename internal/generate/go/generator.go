package gogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"cleanproto/internal/generate"
	"cleanproto/internal/generate/templates"
	"cleanproto/internal/ir"
)

type Generator struct{}

func (g Generator) Name() string {
	return "go"
}

func (g Generator) Generate(files []ir.File, options generate.Options) ([]generate.OutputFile, error) {
	if options.GoOut == "" {
		return nil, nil
	}
	if options.GoPackage == "" {
		return nil, fmt.Errorf("go package name is required")
	}
	tmpl, err := template.ParseFS(templates.FS, "go_file.tmpl")
	if err != nil {
		return nil, err
	}

	msgIndex := indexMessages(files)
	var outputs []generate.OutputFile
	for _, file := range files {
		data, err := buildGoFileData(file, msgIndex, options.GoPackage)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		base := strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path))
		outPath := filepath.Join(options.GoOut, base+"_cp.pb.go")
		outputs = append(outputs, generate.OutputFile{
			Path:    outPath,
			Content: buf.Bytes(),
		})
	}
	utilContent, err := loadUtilSource(options.GoPackage)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, generate.OutputFile{
		Path:    filepath.Join(options.GoOut, "util.go"),
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
	for _, msg := range file.Messages {
		goMsg, mathNeeded, err := buildGoMessage(msg, msgIndex)
		if err != nil {
			return goFileData{}, err
		}
		if mathNeeded {
			usesMath = true
		}
		data.Messages = append(data.Messages, goMsg)
	}
	imports := []string{
		"google.golang.org/protobuf/encoding/protowire",
	}
	if usesMath {
		imports = append([]string{"math"}, imports...)
	}
	data.Imports = imports
	return data, nil
}

func buildGoMessage(msg ir.Message, msgIndex map[string]ir.Message) (goMessage, bool, error) {
	out := goMessage{Name: msg.Name}
	var usesMath bool
	for _, field := range msg.Fields {
		goType, mathNeeded, err := goFieldType(field, msgIndex)
		if err != nil {
			return goMessage{}, false, err
		}
		if mathNeeded {
			usesMath = true
		}
		out.Fields = append(out.Fields, goField{
			Name: ir.GoName(field.Name),
			Type: goType,
		})
	}

	encodeLines, err := buildGoEncodeLines(msg, msgIndex)
	if err != nil {
		return goMessage{}, false, err
	}
	out.EncodeLines = encodeLines

	decodeCases, needsMsgBytes, needsTmpBytes, err := buildGoDecodeCases(msg, msgIndex)
	if err != nil {
		return goMessage{}, false, err
	}
	out.DecodeCases = decodeCases
	out.NeedsMsgBytes = needsMsgBytes
	out.NeedsTmpBytes = needsTmpBytes

	return out, usesMath, nil
}

func goFieldType(field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
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
	lines = append(lines, fmt.Sprintf("if len(%s) > 0 {", fieldName))
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
	lines = append(lines, "}")
	return lines, nil
}

func goDecodeMap(fieldName string, field ir.Field, msgIndex map[string]ir.Message) ([]string, bool, error) {
	var lines []string
	lines = append(lines, "b, msgBytes, err = ConsumeMessage(b, typ)")
	lines = append(lines, "if err != nil {", "return nil, err", "}")
	lines = append(lines, "var keySet bool")
	lines = append(lines, fmt.Sprintf("var key %s", mustGoMapKeyType(field.MapKeyKind)))
	lines = append(lines, fmt.Sprintf("var value %s", mustGoMapValueType(field, msgIndex)))
	lines = append(lines, "var num2 protowire.Number")
	lines = append(lines, "var typ2 protowire.Type")
	lines = append(lines, "var err2 error")
	lines = append(lines, "entryBytes := msgBytes")
	lines = append(lines, "for len(entryBytes) > 0 {")
	lines = append(lines, "entryBytes, num2, typ2, err2 = ConsumeTag(entryBytes)")
	lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
	lines = append(lines, "switch num2 {")
	lines = append(lines, "case 1:")
	keyLines, err := goDecodeMapScalar(field.MapKeyKind, "key", "entryBytes")
	if err != nil {
		return nil, false, err
	}
	lines = append(lines, prefixLines(keyLines, "")...)
	lines = append(lines, "keySet = true")
	lines = append(lines, "case 2:")
	if field.MapValueKind == ir.KindMessage {
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return nil, false, fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		lines = append(lines, "var msgItem []byte")
		lines = append(lines, "entryBytes, msgItem, err2 = ConsumeMessage(entryBytes, typ2)")
		lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
		lines = append(lines, fmt.Sprintf("value, err2 = Decode%s(msgItem)", msg.Name))
		lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
	} else if field.MapValueKind == ir.KindBytes {
		lines = append(lines, "var tmp []byte")
		lines = append(lines, "entryBytes, tmp, err2 = ConsumeBytes(entryBytes, typ2)")
		lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
		lines = append(lines, "value = append([]byte(nil), tmp...)")
	} else {
		valLines, err := goDecodeMapScalar(field.MapValueKind, "value", "entryBytes")
		if err != nil {
			return nil, false, err
		}
		lines = append(lines, prefixLines(valLines, "")...)
	}
	lines = append(lines, "default:")
	lines = append(lines, "entryBytes, err2 = SkipFieldValue(entryBytes, num2, typ2)")
	lines = append(lines, "if err2 != nil {", "return nil, err2", "}")
	lines = append(lines, "}")
	lines = append(lines, "}")
	lines = append(lines, fmt.Sprintf("if %s == nil {", fieldName))
	lines = append(lines, fmt.Sprintf("%s = make(map[%s]%s)", fieldName, mustGoMapKeyType(field.MapKeyKind), mustGoMapValueType(field, msgIndex)))
	lines = append(lines, "}")
	lines = append(lines, "if keySet {")
	lines = append(lines, fmt.Sprintf("%s[key] = value", fieldName))
	lines = append(lines, "}")
	return lines, true, nil
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
	lines = append(lines, fmt.Sprintf("if len(%s) > 0 {", fieldName))
	lines = append(lines, "var packed []byte")
	lines = append(lines, fmt.Sprintf("for _, item := range %s {", fieldName))
	packedLines, err := goEncodePackedItem("item", field)
	if err != nil {
		return nil, err
	}
	lines = append(lines, packedLines...)
	lines = append(lines, "}")
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
			if field.IsPacked && isGoPackable(field.Kind) {
				lines, err := goDecodePacked(fieldName, field)
				if err != nil {
					return nil, false, false, err
				}
				c.Lines = append(c.Lines, lines...)
			} else {
				decodeLines, tmpBytes, err := goDecodeScalar(field, "item")
				if err != nil {
					return nil, false, false, err
				}
				if tmpBytes {
					needsTmpBytes = true
				}
				c.Lines = append(c.Lines, decodeLines...)
				c.Lines = append(c.Lines, fmt.Sprintf("%s = append(%s, item)", fieldName, fieldName))
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
			decodeLines, tmpBytes, err := goDecodeScalar(field, "item")
			if err != nil {
				return nil, false, false, err
			}
			if tmpBytes {
				needsTmpBytes = true
			}
			c.Lines = append(c.Lines, decodeLines...)
			c.Lines = append(c.Lines, fmt.Sprintf("%s = &item", fieldName))
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
	return []byte(updated), nil
}
