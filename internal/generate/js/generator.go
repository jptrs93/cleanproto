package jsg

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jptrs93/cleanproto/internal/generate"
	"github.com/jptrs93/cleanproto/internal/generate/templates"
	"github.com/jptrs93/cleanproto/internal/ir"
)

type Generator struct{}

func (g Generator) Name() string {
	return "js"
}

func (g Generator) Generate(files []ir.File, options generate.Options) ([]generate.OutputFile, error) {
	tmpl, err := template.ParseFS(templates.FS, "js_file.tmpl")
	if err != nil {
		return nil, err
	}
	msgIndex := indexMessages(files)
	var outputs []generate.OutputFile
	for _, file := range files {
		jsOut := options.JsOut
		if jsOut == "" {
			continue
		}
		data, err := buildJSFileData(file, msgIndex)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		outPath := filepath.Join(jsOut, "model.js")
		outputs = append(outputs, generate.OutputFile{
			Path:    outPath,
			Content: buf.Bytes(),
		})
	}
	return outputs, nil
}

type jsFileData struct {
	Typedefs             []string
	Messages             []jsMessage
	NeedsReadInt64       bool
	NeedsReadInt64BigInt bool
	NeedsTimestamp       bool
	NeedsDuration        bool
	NeedsTimestampNative bool
	NeedsDurationBigInt  bool
}

type jsMessage struct {
	WriteFunc         string
	EncodeFunc        string
	DecodeMessageFunc string
	DecodeFunc        string
	NeedsTimestamp    bool
	NeedsDuration     bool
}

func buildJSFileData(file ir.File, msgIndex map[string]ir.Message) (jsFileData, error) {
	var data jsFileData
	for _, msg := range file.Messages {
		typedef, err := buildJSTypedef(msg, msgIndex)
		if err != nil {
			return jsFileData{}, err
		}
		data.Typedefs = append(data.Typedefs, typedef)
		jsMsg, needsReadInt64, err := buildJSMessage(msg, msgIndex)
		if err != nil {
			return jsFileData{}, err
		}
		if needsReadInt64 {
			data.NeedsReadInt64 = true
		}
		if jsMsg.NeedsTimestamp {
			data.NeedsTimestamp = true
		}
		if jsMsg.NeedsDuration {
			data.NeedsDuration = true
		}
		for _, field := range msg.Fields {
			if field.JSType == "bigint" && (field.Kind == ir.KindInt64 || field.IsTimestamp || field.IsDuration) {
				data.NeedsReadInt64BigInt = true
			}
			if field.JSType != "" && field.IsTimestamp {
				data.NeedsTimestampNative = true
			}
			if field.JSType == "bigint" && field.IsDuration {
				data.NeedsDurationBigInt = true
			}
		}
		data.Messages = append(data.Messages, jsMsg)
	}
	return data, nil
}

func buildJSTypedef(msg ir.Message, msgIndex map[string]ir.Message) (string, error) {
	var b strings.Builder
	if ok, field := jsIsRepeatedWrapper(msg); ok {
		elemType, err := jsWrapperElemType(field, msgIndex)
		if err != nil {
			return "", err
		}
		b.WriteString("/**\n")
		b.WriteString(" * @typedef {")
		b.WriteString(elemType)
		b.WriteString("[]} ")
		b.WriteString(msg.Name)
		b.WriteString("\n */")
		return b.String(), nil
	}
	b.WriteString("/**\n")
	b.WriteString(" * @typedef {Object} ")
	b.WriteString(msg.Name)
	b.WriteString("\n")
	for _, field := range msg.Fields {
		jsType, err := jsDocType(field, msgIndex)
		if err != nil {
			return "", err
		}
		b.WriteString(" * @property {")
		b.WriteString(jsType)
		b.WriteString("} ")
		b.WriteString(field.Name)
		b.WriteString("\n")
	}
	b.WriteString(" */")
	return b.String(), nil
}

func buildJSMessage(msg ir.Message, msgIndex map[string]ir.Message) (jsMessage, bool, error) {
	writeFunc, needsReadInt64, needsTimestampWrite, needsDurationWrite, err := buildWriteFunc(msg, msgIndex)
	if err != nil {
		return jsMessage{}, false, err
	}
	encodeFunc := buildEncodeFunc(msg)
	decodeMessageFunc, needsReadInt64Decode, needsTimestampDecode, needsDurationDecode, err := buildDecodeMessageFunc(msg, msgIndex)
	if err != nil {
		return jsMessage{}, false, err
	}
	decodeFunc := buildDecodeFunc(msg)
	return jsMessage{
		WriteFunc:         writeFunc,
		EncodeFunc:        encodeFunc,
		DecodeMessageFunc: decodeMessageFunc,
		DecodeFunc:        decodeFunc,
		NeedsTimestamp:    needsTimestampWrite || needsTimestampDecode,
		NeedsDuration:     needsDurationWrite || needsDurationDecode,
	}, needsReadInt64 || needsReadInt64Decode, nil
}

func buildWriteFunc(msg ir.Message, msgIndex map[string]ir.Message) (string, bool, bool, bool, error) {
	var b strings.Builder
	needsReadInt64 := false
	needsTimestamp := false
	needsDuration := false
	fmt.Fprintf(&b, "/**\n * @param {%s} message\n * @param {Writer} writer\n */\n", msg.Name)
	fmt.Fprintf(&b, "export function write%s(message, writer) {\n", msg.Name)
	if ok, field := jsIsRepeatedWrapper(msg); ok {
		if field.IsPacked && jsIsPackable(field.Kind) {
			b.WriteString("    if (message) {\n")
			b.WriteString("        const packedWriter = Writer.create();\n")
			b.WriteString("        for (const item of message) {\n")
			b.WriteString("            packedWriter.")
			b.WriteString(jsWriterMethod(field.Kind))
			b.WriteString("(item);\n")
			b.WriteString("        }\n")
			b.WriteString("        if (packedWriter.len > 0) {\n")
			b.WriteString("            writer.uint32(tag(")
			b.WriteString(fmt.Sprintf("%d", field.Number))
			b.WriteString(", WIRE.LDELIM)).bytes(packedWriter.finish());\n")
			b.WriteString("        }\n")
			b.WriteString("    }\n")
			b.WriteString("}\n")
			return b.String(), needsReadInt64, field.IsTimestamp, field.IsDuration, nil
		}
		b.WriteString("    if (message) {\n")
		b.WriteString("        for (const item of message) {\n")
		lines, err := jsEncodeField(field, msgIndex, "item", "            ")
		if err != nil {
			return "", false, false, false, err
		}
		if field.IsTimestamp {
			needsTimestamp = true
		}
		if field.IsDuration {
			needsDuration = true
		}
		b.WriteString(lines)
		b.WriteString("        }\n")
		b.WriteString("    }\n")
		b.WriteString("}\n")
		return b.String(), needsReadInt64, needsTimestamp, needsDuration, nil
	}
	for _, field := range msg.Fields {
		fieldName := "message." + field.Name
		if field.IsTimestamp {
			needsTimestamp = true
		}
		if field.IsDuration {
			needsDuration = true
		}
		if field.IsMap {
			b.WriteString("    if (message.")
			b.WriteString(field.Name)
			b.WriteString(" && Object.keys(message.")
			b.WriteString(field.Name)
			b.WriteString(").length > 0) {\n")
			b.WriteString("        for (const [rawKey, value] of Object.entries(message.")
			b.WriteString(field.Name)
			b.WriteString(")) {\n")
			b.WriteString("            const key = ")
			b.WriteString(jsMapKeyCast(field.MapKeyKind))
			b.WriteString(";\n")
			b.WriteString("            writer.uint32(tag(")
			b.WriteString(fmt.Sprintf("%d", field.Number))
			b.WriteString(", WIRE.LDELIM)).fork();\n")
			b.WriteString("            writer.uint32(tag(1, ")
			b.WriteString(jsWireType(field.MapKeyKind))
			b.WriteString(")).")
			b.WriteString(jsWriterMethod(field.MapKeyKind))
			b.WriteString("(key);\n")
			mapValueLines, err := jsEncodeMapValue(field, msgIndex)
			if err != nil {
				return "", false, false, false, err
			}
			b.WriteString(mapValueLines)
			b.WriteString("            writer.ldelim();\n")
			b.WriteString("        }\n")
			b.WriteString("    }\n")
			continue
		}
		if field.IsRepeated {
			if field.IsPacked && jsIsPackable(field.Kind) {
				b.WriteString("    if (message.")
				b.WriteString(field.Name)
				b.WriteString(") {\n")
				b.WriteString("        const packedWriter = Writer.create();\n")
				b.WriteString("        for (const item of message.")
				b.WriteString(field.Name)
				b.WriteString(") {\n")
				b.WriteString("            packedWriter.")
				b.WriteString(jsWriterMethod(field.Kind))
				b.WriteString("(item);\n")
				b.WriteString("        }\n")
				b.WriteString("        if (packedWriter.len > 0) {\n")
				b.WriteString("            writer.uint32(tag(")
				b.WriteString(fmt.Sprintf("%d", field.Number))
				b.WriteString(", WIRE.LDELIM)).bytes(packedWriter.finish());\n")
				b.WriteString("        }\n")
				b.WriteString("    }\n")
				continue
			}
			b.WriteString("    if (message.")
			b.WriteString(field.Name)
			b.WriteString(" && message.")
			b.WriteString(field.Name)
			b.WriteString(".length > 0) {\n")
			b.WriteString("        for (const item of message.")
			b.WriteString(field.Name)
			b.WriteString(") {\n")
			lines, err := jsEncodeField(field, msgIndex, "item", "            ")
			if err != nil {
				return "", false, false, false, err
			}
			b.WriteString(lines)
			b.WriteString("        }\n")
			b.WriteString("    }\n")
			continue
		}
		cond := jsPresenceCheck(field, fieldName)
		if cond != "" {
			b.WriteString("    if (")
			b.WriteString(cond)
			b.WriteString(") {\n")
		}
		lines, err := jsEncodeField(field, msgIndex, fieldName, "        ")
		if err != nil {
			return "", false, false, false, err
		}
		b.WriteString(lines)
		if cond != "" {
			b.WriteString("    }\n")
		}
	}
	b.WriteString("}\n")
	for _, field := range msg.Fields {
		if isJSReadInt64(field) {
			needsReadInt64 = true
			break
		}
	}
	return b.String(), needsReadInt64, needsTimestamp, needsDuration, nil
}

func buildEncodeFunc(msg ir.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/**\n * @param {%s} message\n * @returns {Uint8Array}\n */\n", msg.Name)
	fmt.Fprintf(&b, "export function encode%s(message) {\n", msg.Name)
	b.WriteString("    const writer = Writer.create();\n")
	fmt.Fprintf(&b, "    write%s(message, writer);\n", msg.Name)
	b.WriteString("    return writer.finish();\n")
	b.WriteString("}\n")
	return b.String()
}

func buildDecodeFunc(msg ir.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/**\n * @param {ArrayBuffer} buffer\n * @returns {%s}\n */\n", msg.Name)
	fmt.Fprintf(&b, "export function decode%s(buffer) {\n", msg.Name)
	b.WriteString("    const reader = Reader.create(new Uint8Array(buffer));\n")
	fmt.Fprintf(&b, "    return decode%sMessage(reader);\n", msg.Name)
	b.WriteString("}\n")
	return b.String()
}

func buildDecodeMessageFunc(msg ir.Message, msgIndex map[string]ir.Message) (string, bool, bool, bool, error) {
	var b strings.Builder
	needsReadInt64 := false
	needsTimestamp := false
	needsDuration := false
	fmt.Fprintf(&b, "/**\n * @param {Reader} reader\n * @param {number} [length]\n * @returns {%s}\n */\n", msg.Name)
	fmt.Fprintf(&b, "function decode%sMessage(reader, length) {\n", msg.Name)
	b.WriteString("    const end = length === undefined ? reader.len : reader.pos + length;\n")
	if ok, field := jsIsRepeatedWrapper(msg); ok {
		b.WriteString("    const message = [];\n")
		b.WriteString("    while (reader.pos < end) {\n")
		b.WriteString("        const tag = reader.uint32();\n")
		b.WriteString("        switch (tag >>> 3) {\n")
		b.WriteString("            case ")
		b.WriteString(fmt.Sprintf("%d", field.Number))
		b.WriteString(": {\n")
		lines, usesReadInt64, usesTimestamp, err := jsDecodeWrapperField(field, msgIndex)
		if err != nil {
			return "", false, false, false, err
		}
		if usesReadInt64 {
			needsReadInt64 = true
		}
		if usesTimestamp {
			needsTimestamp = true
		}
		if field.IsDuration {
			needsDuration = true
		}
		b.WriteString(lines)
		b.WriteString("                break;\n")
		b.WriteString("            }\n")
		b.WriteString("            default:\n")
		b.WriteString("                reader.skipType(tag & 7);\n")
		b.WriteString("        }\n")
		b.WriteString("    }\n")
		b.WriteString("    return message;\n")
		b.WriteString("}\n")
		return b.String(), needsReadInt64, needsTimestamp, needsDuration, nil
	}
	b.WriteString("    const message = {")
	for i, field := range msg.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(jsDefaultValue(field, msgIndex))
	}
	b.WriteString(" };\n")
	b.WriteString("    while (reader.pos < end) {\n")
	b.WriteString("        const tag = reader.uint32();\n")
	b.WriteString("        switch (tag >>> 3) {\n")
	for _, field := range msg.Fields {
		b.WriteString("            case ")
		b.WriteString(fmt.Sprintf("%d", field.Number))
		b.WriteString(": {\n")
		lines, usesReadInt64, usesTimestamp, err := jsDecodeField(field, msgIndex, "message")
		if err != nil {
			return "", false, false, false, err
		}
		if usesReadInt64 {
			needsReadInt64 = true
		}
		if usesTimestamp {
			needsTimestamp = true
		}
		if field.IsDuration {
			needsDuration = true
		}
		b.WriteString(lines)
		b.WriteString("                break;\n")
		b.WriteString("            }\n")
	}
	b.WriteString("            default:\n")
	b.WriteString("                reader.skipType(tag & 7);\n")
	b.WriteString("        }\n")
	b.WriteString("    }\n")
	b.WriteString("    return message;\n")
	b.WriteString("}\n")
	return b.String(), needsReadInt64, needsTimestamp, needsDuration, nil
}

func jsDocType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	if field.IsMap {
		valueType, err := jsMapValueType(field, msgIndex)
		if err != nil {
			return "", err
		}
		return "Object.<string, " + valueType + ">", nil
	}
	t, err := jsBaseType(field, msgIndex)
	if err != nil {
		return "", err
	}
	if field.IsRepeated {
		return t + "[]", nil
	}
	return t, nil
}

func jsDefaultValue(field ir.Field, msgIndex map[string]ir.Message) string {
	if field.IsMap {
		return "{}"
	}
	if field.IsRepeated {
		return "[]"
	}
	if field.JSType == "bigint" {
		if field.IsOptional {
			return "undefined"
		}
		return "0n"
	}
	if field.JSType == "number" {
		if field.IsOptional {
			return "undefined"
		}
		return "0"
	}
	if field.IsTimestamp {
		if field.IsOptional {
			return "undefined"
		}
		return "new Date(0)"
	}
	if field.IsDuration {
		if field.IsOptional {
			return "undefined"
		}
		return "0"
	}
	if field.IsOptional {
		return "undefined"
	}
	switch field.Kind {
	case ir.KindString:
		return "\"\""
	case ir.KindBytes:
		return "new Uint8Array(0)"
	case ir.KindBool:
		return "false"
	case ir.KindMessage:
		return "undefined"
	default:
		return "0"
	}
}

func jsBaseType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	if field.JSType != "" {
		return field.JSType, nil
	}
	if field.IsTimestamp {
		return "Date", nil
	}
	if field.IsDuration {
		return "number", nil
	}
	switch field.Kind {
	case ir.KindString:
		return "string", nil
	case ir.KindBytes:
		return "Uint8Array", nil
	case ir.KindBool:
		return "boolean", nil
	case ir.KindMessage:
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			return "", fmt.Errorf("unknown message type: %s", field.MessageFullName)
		}
		return msg.Name, nil
	default:
		return "number", nil
	}
}

func jsPresenceCheck(field ir.Field, name string) string {
	if field.IsOptional {
		return name + " !== undefined && " + name + " !== null"
	}
	if field.JSType == "bigint" {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0n"
	}
	if field.JSType == "number" {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0"
	}
	if field.Kind == ir.KindMessage {
		return name + " !== undefined && " + name + " !== null"
	}
	if field.IsTimestamp {
		return name + " instanceof Date"
	}
	if field.IsDuration {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0"
	}
	switch field.Kind {
	case ir.KindString:
		return name + " !== undefined && " + name + " !== null && " + name + " !== \"\""
	case ir.KindBytes:
		return name + " && " + name + ".length > 0"
	case ir.KindBool:
		return name + " === true"
	default:
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0"
	}
}

func jsEncodeField(field ir.Field, msgIndex map[string]ir.Message, name, indent string) (string, error) {
	var b strings.Builder
	if field.JSType != "" {
		lines, err := jsEncodeNativeField(field, name, indent)
		if err != nil {
			return "", err
		}
		b.WriteString(lines)
		return b.String(), nil
	}
	if field.IsTimestamp {
		fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
		fmt.Fprintf(&b, "%swriteTimestamp(%s, writer);\n", indent, name)
		fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
		return b.String(), nil
	}
	if field.IsDuration {
		fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
		fmt.Fprintf(&b, "%swriteDuration(%s, writer);\n", indent, name)
		fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
		return b.String(), nil
	}
	wire := jsWireType(field.Kind)
	if field.Kind == ir.KindMessage {
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			return "", fmt.Errorf("unknown message type: %s", field.MessageFullName)
		}
		fmt.Fprintf(&b, "%swriter.uint32(tag(%d, %s)).fork();\n", indent, field.Number, wire)
		fmt.Fprintf(&b, "%swrite%s(%s, writer);\n", indent, msg.Name, name)
		fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
		return b.String(), nil
	}
	method := jsWriterMethod(field.Kind)
	fmt.Fprintf(&b, "%swriter.uint32(tag(%d, %s)).%s(%s);\n", indent, field.Number, wire, method, name)
	return b.String(), nil
}

func jsDecodeField(field ir.Field, msgIndex map[string]ir.Message, target string) (string, bool, bool, error) {
	var b strings.Builder
	fieldName := target + "." + field.Name
	if field.JSType != "" {
		lines, needsReadInt64, err := jsDecodeNativeField(field, fieldName)
		if err != nil {
			return "", false, false, err
		}
		b.WriteString(lines)
		return b.String(), needsReadInt64, false, nil
	}
	if field.IsMap {
		mapLines, needsReadInt64, err := jsDecodeMapField(fieldName, field, msgIndex)
		if err != nil {
			return "", false, false, err
		}
		b.WriteString(mapLines)
		return b.String(), needsReadInt64, false, nil
	}
	if field.IsRepeated {
		if field.IsTimestamp {
			lines, needsReadInt64 := jsDecodeTimestampRepeated(fieldName, field)
			b.WriteString(lines)
			return b.String(), needsReadInt64, true, nil
		}
		if field.IsDuration {
			lines, needsReadInt64 := jsDecodeDurationRepeated(fieldName)
			b.WriteString(lines)
			return b.String(), needsReadInt64, false, nil
		}
		if field.Kind == ir.KindMessage {
			msg, ok := msgIndex[field.MessageFullName]
			if !ok {
				return "", false, false, fmt.Errorf("unknown message type: %s", field.MessageFullName)
			}
			fmt.Fprintf(&b, "                %s.push(decode%sMessage(reader, reader.uint32()));\n", fieldName, msg.Name)
			return b.String(), false, false, nil
		}
		if field.IsPacked && jsIsPackable(field.Kind) {
			packedLines, needsReadInt64 := jsDecodePackedField(fieldName, field)
			b.WriteString(packedLines)
			return b.String(), needsReadInt64, false, nil
		}
		if isJSReadInt64(field) {
			fmt.Fprintf(&b, "                %s.push(readInt64(reader, \"%s\"));\n", fieldName, jsReaderMethod(field.Kind))
			return b.String(), true, false, nil
		}
		fmt.Fprintf(&b, "                %s.push(reader.%s());\n", fieldName, jsReaderMethod(field.Kind))
		return b.String(), false, false, nil
	}
	if field.IsTimestamp {
		lines, needsReadInt64 := jsDecodeTimestampSingle(fieldName, field)
		b.WriteString(lines)
		return b.String(), needsReadInt64, true, nil
	}
	if field.IsDuration {
		lines, needsReadInt64 := jsDecodeDurationSingle(fieldName)
		b.WriteString(lines)
		return b.String(), needsReadInt64, false, nil
	}

	if field.Kind == ir.KindMessage {
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			return "", false, false, fmt.Errorf("unknown message type: %s", field.MessageFullName)
		}
		fmt.Fprintf(&b, "                %s = decode%sMessage(reader, reader.uint32());\n", fieldName, msg.Name)
		return b.String(), false, false, nil
	}
	if isJSReadInt64(field) {
		fmt.Fprintf(&b, "                %s = readInt64(reader, \"%s\");\n", fieldName, jsReaderMethod(field.Kind))
		return b.String(), true, false, nil
	}
	fmt.Fprintf(&b, "                %s = reader.%s();\n", fieldName, jsReaderMethod(field.Kind))
	return b.String(), false, false, nil
}

func jsEncodeNativeField(field ir.Field, name, indent string) (string, error) {
	var b strings.Builder
	switch field.JSType {
	case "number":
		if field.IsTimestamp {
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
			fmt.Fprintf(&b, "%swriteTimestampFromMillis(%s, writer);\n", indent, name)
			fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
			return b.String(), nil
		}
		if field.IsDuration {
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
			fmt.Fprintf(&b, "%swriteDuration(%s, writer);\n", indent, name)
			fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
			return b.String(), nil
		}
		switch field.Kind {
		case ir.KindInt32:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int32(Math.trunc(%s));\n", indent, field.Number, name)
			return b.String(), nil
		case ir.KindInt64:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int64(Math.trunc(%s));\n", indent, field.Number, name)
			return b.String(), nil
		}
	case "bigint":
		if field.IsTimestamp {
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
			fmt.Fprintf(&b, "%swriteTimestampFromBigInt(%s, writer);\n", indent, name)
			fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
			return b.String(), nil
		}
		if field.IsDuration {
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
			fmt.Fprintf(&b, "%swriteDurationFromBigInt(%s, writer);\n", indent, name)
			fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
			return b.String(), nil
		}
		switch field.Kind {
		case ir.KindInt32:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int32(Number(%s));\n", indent, field.Number, name)
			return b.String(), nil
		case ir.KindInt64:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int64(%s.toString());\n", indent, field.Number, name)
			return b.String(), nil
		}
	}
	return "", fmt.Errorf("unsupported js native type conversion for field: %s", field.Name)
}

func jsDecodeNativeField(field ir.Field, fieldName string) (string, bool, error) {
	var b strings.Builder
	if field.IsRepeated {
		if field.Kind == ir.KindInt64 {
			if field.IsPacked {
				b.WriteString("                if ((tag & 7) === WIRE.LDELIM) {\n")
				b.WriteString("                    const end2 = reader.uint32() + reader.pos;\n")
				b.WriteString("                    while (reader.pos < end2) {\n")
				if field.JSType == "bigint" {
					b.WriteString("                        ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64BigInt(reader, \"int64\"));\n")
				} else {
					b.WriteString("                        ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64(reader, \"int64\"));\n")
				}
				b.WriteString("                    }\n")
				b.WriteString("                } else {\n")
				if field.JSType == "bigint" {
					b.WriteString("                    ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64BigInt(reader, \"int64\"));\n")
				} else {
					b.WriteString("                    ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64(reader, \"int64\"));\n")
				}
				b.WriteString("                }\n")
				return b.String(), true, nil
			}
			if field.JSType == "bigint" {
				b.WriteString("                ")
				b.WriteString(fieldName)
				b.WriteString(".push(readInt64BigInt(reader, \"int64\"));\n")
			} else {
				b.WriteString("                ")
				b.WriteString(fieldName)
				b.WriteString(".push(readInt64(reader, \"int64\"));\n")
			}
			return b.String(), true, nil
		}
		if field.IsTimestamp {
			b.WriteString("                ")
			b.WriteString(fieldName)
			if field.JSType == "bigint" {
				b.WriteString(".push(decodeTimestampBigIntMessage(reader, reader.uint32()));\n")
				return b.String(), true, nil
			}
			b.WriteString(".push(decodeTimestampMillisMessage(reader, reader.uint32()));\n")
			return b.String(), true, nil
		}
		if field.IsDuration {
			b.WriteString("                ")
			b.WriteString(fieldName)
			if field.JSType == "bigint" {
				b.WriteString(".push(decodeDurationBigIntMessage(reader, reader.uint32()));\n")
				return b.String(), true, nil
			}
			b.WriteString(".push(decodeDurationMessage(reader, reader.uint32()));\n")
			return b.String(), true, nil
		}
		if field.Kind == ir.KindInt32 {
			b.WriteString("                ")
			b.WriteString(fieldName)
			if field.JSType == "bigint" {
				b.WriteString(".push(BigInt(reader.int32()));\n")
			} else {
				b.WriteString(".push(reader.int32());\n")
			}
			return b.String(), false, nil
		}
	}

	if field.IsTimestamp {
		if field.JSType == "bigint" {
			return "                " + fieldName + " = decodeTimestampBigIntMessage(reader, reader.uint32());\n", true, nil
		}
		return "                " + fieldName + " = decodeTimestampMillisMessage(reader, reader.uint32());\n", true, nil
	}
	if field.IsDuration {
		if field.JSType == "bigint" {
			return "                " + fieldName + " = decodeDurationBigIntMessage(reader, reader.uint32());\n", true, nil
		}
		return "                " + fieldName + " = decodeDurationMessage(reader, reader.uint32());\n", true, nil
	}
	if field.Kind == ir.KindInt64 {
		if field.JSType == "bigint" {
			return "                " + fieldName + " = readInt64BigInt(reader, \"int64\");\n", true, nil
		}
		return "                " + fieldName + " = readInt64(reader, \"int64\");\n", true, nil
	}
	if field.Kind == ir.KindInt32 {
		if field.JSType == "bigint" {
			return "                " + fieldName + " = BigInt(reader.int32());\n", false, nil
		}
		return "                " + fieldName + " = reader.int32();\n", false, nil
	}
	return "", false, fmt.Errorf("unsupported js native type conversion for field: %s", field.Name)
}

func isJSReadInt64(field ir.Field) bool {
	switch field.Kind {
	case ir.KindInt64, ir.KindUint64, ir.KindSint64, ir.KindFixed64, ir.KindSfixed64:
		return true
	default:
		return false
	}
}

func jsWireType(kind ir.Kind) string {
	switch kind {
	case ir.KindString, ir.KindBytes, ir.KindMessage:
		return "WIRE.LDELIM"
	case ir.KindFixed32, ir.KindSfixed32, ir.KindFloat:
		return "WIRE.FIXED32"
	case ir.KindFixed64, ir.KindSfixed64, ir.KindDouble:
		return "WIRE.FIXED64"
	default:
		return "WIRE.VARINT"
	}
}

func jsWriterMethod(kind ir.Kind) string {
	switch kind {
	case ir.KindBool:
		return "bool"
	case ir.KindInt32:
		return "int32"
	case ir.KindInt64:
		return "int64"
	case ir.KindUint32:
		return "uint32"
	case ir.KindUint64:
		return "uint64"
	case ir.KindSint32:
		return "sint32"
	case ir.KindSint64:
		return "sint64"
	case ir.KindFixed32:
		return "fixed32"
	case ir.KindFixed64:
		return "fixed64"
	case ir.KindSfixed32:
		return "sfixed32"
	case ir.KindSfixed64:
		return "sfixed64"
	case ir.KindFloat:
		return "float"
	case ir.KindDouble:
		return "double"
	case ir.KindString:
		return "string"
	case ir.KindBytes:
		return "bytes"
	case ir.KindEnum:
		return "int32"
	default:
		return "int32"
	}
}

func jsReaderMethod(kind ir.Kind) string {
	switch kind {
	case ir.KindBool:
		return "bool"
	case ir.KindInt32:
		return "int32"
	case ir.KindInt64:
		return "int64"
	case ir.KindUint32:
		return "uint32"
	case ir.KindUint64:
		return "uint64"
	case ir.KindSint32:
		return "sint32"
	case ir.KindSint64:
		return "sint64"
	case ir.KindFixed32:
		return "fixed32"
	case ir.KindFixed64:
		return "fixed64"
	case ir.KindSfixed32:
		return "sfixed32"
	case ir.KindSfixed64:
		return "sfixed64"
	case ir.KindFloat:
		return "float"
	case ir.KindDouble:
		return "double"
	case ir.KindString:
		return "string"
	case ir.KindBytes:
		return "bytes"
	case ir.KindEnum:
		return "int32"
	default:
		return "int32"
	}
}

func jsIsPackable(kind ir.Kind) bool {
	switch kind {
	case ir.KindString, ir.KindBytes, ir.KindMessage:
		return false
	default:
		return true
	}
}

func jsMapKeyCast(kind ir.Kind) string {
	switch kind {
	case ir.KindString:
		return "rawKey"
	case ir.KindBool:
		return "rawKey === \"true\""
	default:
		return "Number(rawKey)"
	}
}

func jsMapValueType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	switch field.MapValueKind {
	case ir.KindMessage:
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		return msg.Name, nil
	case ir.KindBytes:
		return "Uint8Array", nil
	case ir.KindBool:
		return "boolean", nil
	case ir.KindString:
		return "string", nil
	default:
		return "number", nil
	}
}

func jsEncodeMapValue(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	var b strings.Builder
	if field.MapValueKind == ir.KindMessage {
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		b.WriteString("            if (value) {\n")
		b.WriteString("                writer.uint32(tag(2, WIRE.LDELIM)).fork();\n")
		b.WriteString("                write")
		b.WriteString(msg.Name)
		b.WriteString("(value, writer);\n")
		b.WriteString("                writer.ldelim();\n")
		b.WriteString("            }\n")
		return b.String(), nil
	}
	if field.MapValueKind == ir.KindBytes {
		b.WriteString("            if (value && value.length > 0) {\n")
		b.WriteString("                writer.uint32(tag(2, WIRE.LDELIM)).bytes(value);\n")
		b.WriteString("            }\n")
		return b.String(), nil
	}
	method := jsWriterMethod(field.MapValueKind)
	cond := jsMapValuePresence(field.MapValueKind)
	if cond != "" {
		b.WriteString("            if (")
		b.WriteString(cond)
		b.WriteString(") {\n")
		b.WriteString("                writer.uint32(tag(2, ")
		b.WriteString(jsWireType(field.MapValueKind))
		b.WriteString(")).")
		b.WriteString(method)
		b.WriteString("(value);\n")
		b.WriteString("            }\n")
	} else {
		b.WriteString("            writer.uint32(tag(2, ")
		b.WriteString(jsWireType(field.MapValueKind))
		b.WriteString(")).")
		b.WriteString(method)
		b.WriteString("(value);\n")
	}
	return b.String(), nil
}

func jsMapValuePresence(kind ir.Kind) string {
	switch kind {
	case ir.KindString:
		return "value !== undefined && value !== null && value !== \"\""
	case ir.KindBool:
		return "value === true"
	default:
		return "value !== undefined && value !== null && value !== 0"
	}
}

func jsDecodeMapField(fieldName string, field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	var b strings.Builder
	needsReadInt64 := false
	b.WriteString("                const end2 = reader.uint32() + reader.pos;\n")
	b.WriteString("                let key = ")
	b.WriteString(jsMapKeyDefault(field.MapKeyKind))
	b.WriteString(";\n")
	b.WriteString("                let value = ")
	b.WriteString(jsMapValueDefault(field, msgIndex))
	b.WriteString(";\n")
	b.WriteString("                while (reader.pos < end2) {\n")
	b.WriteString("                    const tag2 = reader.uint32();\n")
	b.WriteString("                    switch (tag2 >>> 3) {\n")
	b.WriteString("                        case 1:\n")
	keyRead, keyNeedsReadInt64 := jsReadValue(field.MapKeyKind, msgIndex, "key")
	if keyNeedsReadInt64 {
		needsReadInt64 = true
	}
	b.WriteString(keyRead)
	b.WriteString("                            break;\n")
	b.WriteString("                        case 2:\n")
	valueRead, valueNeedsReadInt64, err := jsReadMapValue(field, msgIndex)
	if err != nil {
		return "", false, err
	}
	if valueNeedsReadInt64 {
		needsReadInt64 = true
	}
	b.WriteString(valueRead)
	b.WriteString("                            break;\n")
	b.WriteString("                        default:\n")
	b.WriteString("                            reader.skipType(tag2 & 7);\n")
	b.WriteString("                    }\n")
	b.WriteString("                }\n")
	b.WriteString("                if (!")
	b.WriteString(fieldName)
	b.WriteString(") { ")
	b.WriteString(fieldName)
	b.WriteString(" = {}; }\n")
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString("[String(key)] = value;\n")
	return b.String(), needsReadInt64, nil
}

func jsReadMapValue(field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	if field.MapValueKind == ir.KindMessage {
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", false, fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		return "                            value = decode" + msg.Name + "Message(reader, reader.uint32());\n", false, nil
	}
	if isJSReadInt64(ir.Field{Kind: field.MapValueKind}) {
		return "                            value = readInt64(reader, \"" + jsReaderMethod(field.MapValueKind) + "\");\n", true, nil
	}
	return "                            value = reader." + jsReaderMethod(field.MapValueKind) + "();\n", false, nil
}

func jsReadValue(kind ir.Kind, msgIndex map[string]ir.Message, target string) (string, bool) {
	if isJSReadInt64(ir.Field{Kind: kind}) {
		return "                            " + target + " = readInt64(reader, \"" + jsReaderMethod(kind) + "\");\n", true
	}
	return "                            " + target + " = reader." + jsReaderMethod(kind) + "();\n", false
}

func jsMapKeyDefault(kind ir.Kind) string {
	switch kind {
	case ir.KindBool:
		return "false"
	case ir.KindString:
		return "\"\""
	default:
		return "0"
	}
}

func jsMapValueDefault(field ir.Field, msgIndex map[string]ir.Message) string {
	switch field.MapValueKind {
	case ir.KindBool:
		return "false"
	case ir.KindString:
		return "\"\""
	case ir.KindBytes:
		return "new Uint8Array(0)"
	case ir.KindMessage:
		return "undefined"
	default:
		return "0"
	}
}

func jsDecodePackedField(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	needsReadInt64 := isJSReadInt64(field)
	b.WriteString("                if ((tag & 7) === WIRE.LDELIM) {\n")
	b.WriteString("                    const end2 = reader.uint32() + reader.pos;\n")
	b.WriteString("                    while (reader.pos < end2) {\n")
	if needsReadInt64 {
		b.WriteString("                        ")
		b.WriteString(fieldName)
		b.WriteString(".push(readInt64(reader, \"")
		b.WriteString(jsReaderMethod(field.Kind))
		b.WriteString("\"));\n")
	} else {
		b.WriteString("                        ")
		b.WriteString(fieldName)
		b.WriteString(".push(reader.")
		b.WriteString(jsReaderMethod(field.Kind))
		b.WriteString("());\n")
	}
	b.WriteString("                    }\n")
	b.WriteString("                } else {\n")
	if needsReadInt64 {
		b.WriteString("                    ")
		b.WriteString(fieldName)
		b.WriteString(".push(readInt64(reader, \"")
		b.WriteString(jsReaderMethod(field.Kind))
		b.WriteString("\"));\n")
	} else {
		b.WriteString("                    ")
		b.WriteString(fieldName)
		b.WriteString(".push(reader.")
		b.WriteString(jsReaderMethod(field.Kind))
		b.WriteString("());\n")
	}
	b.WriteString("                }\n")
	return b.String(), needsReadInt64
}

func jsDecodeTimestampSingle(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(" = decodeTimestampMessage(reader, reader.uint32());\n")
	return b.String(), true
}

func jsDecodeTimestampRepeated(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(".push(decodeTimestampMessage(reader, reader.uint32()));\n")
	return b.String(), true
}

func jsDecodeTimestampWrapper(field ir.Field) (string, bool) {
	return jsDecodeTimestampRepeated("message", field)
}

func jsDecodeDurationSingle(fieldName string) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(" = decodeDurationMessage(reader, reader.uint32());\n")
	return b.String(), true
}

func jsDecodeDurationRepeated(fieldName string) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(".push(decodeDurationMessage(reader, reader.uint32()));\n")
	return b.String(), true
}

func jsDecodeDurationWrapper() (string, bool) {
	return jsDecodeDurationRepeated("message")
}

func jsIsRepeatedWrapper(msg ir.Message) (bool, ir.Field) {
	if len(msg.Fields) != 1 {
		return false, ir.Field{}
	}
	field := msg.Fields[0]
	if field.IsRepeated && !field.IsMap {
		return true, field
	}
	return false, ir.Field{}
}

func jsWrapperElemType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	baseField := ir.Field{
		Kind:            field.Kind,
		MessageFullName: field.MessageFullName,
		EnumFullName:    field.EnumFullName,
	}
	return jsBaseType(baseField, msgIndex)
}

func jsDecodeWrapperField(field ir.Field, msgIndex map[string]ir.Message) (string, bool, bool, error) {
	if field.JSType != "" {
		lines, needsReadInt64, err := jsDecodeNativeField(field, "message")
		if err != nil {
			return "", false, false, err
		}
		return lines, needsReadInt64, false, nil
	}
	if field.IsTimestamp {
		lines, needsReadInt64 := jsDecodeTimestampWrapper(field)
		return lines, needsReadInt64, true, nil
	}
	if field.IsDuration {
		lines, needsReadInt64 := jsDecodeDurationWrapper()
		return lines, needsReadInt64, false, nil
	}
	if field.Kind == ir.KindMessage {
		msg, ok := msgIndex[field.MessageFullName]
		if !ok {
			return "", false, false, fmt.Errorf("unknown message type: %s", field.MessageFullName)
		}
		return "                message.push(decode" + msg.Name + "Message(reader, reader.uint32()));\n", false, false, nil
	}
	if field.IsPacked && jsIsPackable(field.Kind) {
		lines, needsReadInt64 := jsDecodePackedField("message", field)
		return lines, needsReadInt64, false, nil
	}
	if isJSReadInt64(field) {
		return "                message.push(readInt64(reader, \"" + jsReaderMethod(field.Kind) + "\"));\n", true, false, nil
	}
	return "                message.push(reader." + jsReaderMethod(field.Kind) + "());\n", false, false, nil
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
