package tsg

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/jptrs93/cleanproto/internal/generate"
	"github.com/jptrs93/cleanproto/internal/generate/templates"
	"github.com/jptrs93/cleanproto/internal/ir"
)

type Generator struct{}

func (g Generator) Name() string {
	return "ts"
}

func (g Generator) Generate(files []ir.File, options generate.Options) ([]generate.OutputFile, error) {
	tmpl, err := template.ParseFS(templates.FS, "ts_file.tmpl")
	if err != nil {
		return nil, err
	}
	msgIndex := indexMessages(files)
	var outputs []generate.OutputFile
	for _, file := range files {
		tsOut := options.TsOut
		if tsOut == "" {
			continue
		}
		data, err := buildTSFileData(file, msgIndex)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		outPath := filepath.Join(tsOut, "model.ts")
		outputs = append(outputs, generate.OutputFile{
			Path:    outPath,
			Content: buf.Bytes(),
		})
		if len(file.Services) > 0 {
			capi, err := buildTSCapiFile(file, msgIndex)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, generate.OutputFile{
				Path:    filepath.Join(tsOut, "capi.ts"),
				Content: []byte(capi),
			})
		}
	}
	return outputs, nil
}

func buildTSCapiFile(file ir.File, msgIndex map[string]ir.Message) (string, error) {
	type capiMethod struct {
		Name       string
		Path       string
		HTTPMethod string
		InputType  string
		OutputType string
	}
	methods := make([]capiMethod, 0)
	decodeImports := map[string]struct{}{}
	encodeImports := map[string]struct{}{}
	typeImports := map[string]struct{}{}
	for _, svc := range file.Services {
		for _, m := range svc.Methods {
			httpMethod, path, ok := deriveHTTP(m.Name)
			if !ok {
				continue
			}
			inType, ok := messageNameByFullName(msgIndex, m.InputFullName)
			if !ok {
				return "", fmt.Errorf("unknown method input type: %s", m.InputFullName)
			}
			outType, ok := messageNameByFullName(msgIndex, m.OutputFullName)
			if !ok {
				return "", fmt.Errorf("unknown method output type: %s", m.OutputFullName)
			}
			cm := capiMethod{
				Name:       lowerFirst(m.Name),
				Path:       path,
				HTTPMethod: httpMethod,
				InputType:  inType,
				OutputType: outType,
			}
			if inType != "Empty" {
				encodeImports["encode"+inType] = struct{}{}
				typeImports[inType] = struct{}{}
			}
			if outType != "Empty" {
				decodeImports["decode"+outType] = struct{}{}
				typeImports[outType] = struct{}{}
			}
			methods = append(methods, cm)
		}
	}
	if len(methods) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("import {\n")
	imports := make([]string, 0, len(decodeImports)+len(encodeImports))
	for name := range decodeImports {
		imports = append(imports, name)
	}
	for name := range encodeImports {
		imports = append(imports, name)
	}
	sort.Strings(imports)
	for _, name := range imports {
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteString(",\n")
	}
	b.WriteString("} from './model';\n")
	if len(typeImports) > 0 {
		types := make([]string, 0, len(typeImports))
		for name := range typeImports {
			types = append(types, name)
		}
		sort.Strings(types)
		b.WriteString("import type {\n")
		for _, name := range types {
			b.WriteString("  ")
			b.WriteString(name)
			b.WriteString(",\n")
		}
		b.WriteString("} from './model';\n\n")
	}
	b.WriteString("type HeaderProvider = () => Record<string, string>;\n")
	b.WriteString("type ErrorHandler = (response: Response) => Promise<never>;\n")
	b.WriteString("type RequestBody = BodyInit | Uint8Array<ArrayBufferLike>;\n\n")
	b.WriteString("export class Capi {\n")
	b.WriteString("  baseURL: string;\n")
	b.WriteString("  headerProvider: HeaderProvider;\n\n")
	b.WriteString("  errorHandler: ErrorHandler;\n\n")
	b.WriteString("  constructor(baseURL = '', headerProvider: HeaderProvider | null = null, errorHandler: ErrorHandler | null = null) {\n")
	b.WriteString("    this.baseURL = baseURL;\n")
	b.WriteString("    this.headerProvider = headerProvider == null ? () => ({}) : headerProvider;\n")
	b.WriteString("    this.errorHandler = errorHandler == null ? async (response: Response) => { throw new Error(`HTTP ${response.status}`); } : errorHandler;\n")
	b.WriteString("  }\n\n")
	b.WriteString("  async #request(path: string, { method = 'GET', body }: { method?: string; body?: RequestBody } = {}): Promise<Response> {\n")
	b.WriteString("    const headers = this.headerProvider() || {};\n")
	b.WriteString("    headers['Accept'] = 'application/x-protobuf';\n")
	b.WriteString("    if (body !== undefined) {\n")
	b.WriteString("      headers['Content-Type'] = 'application/x-protobuf';\n")
	b.WriteString("    }\n")
	b.WriteString("    return fetch(`${this.baseURL}${path}`, { method, headers, body, credentials: 'include' });\n")
	b.WriteString("  }\n\n")
	for _, m := range methods {
		b.WriteString("  async ")
		b.WriteString(m.Name)
		if m.InputType == "Empty" {
			if m.OutputType == "Empty" {
				b.WriteString("(): Promise<void> {\n")
			} else {
				b.WriteString("(): Promise<")
				b.WriteString(m.OutputType)
				b.WriteString("> {\n")
			}
			b.WriteString("    const response = await this.#request('")
			b.WriteString(m.Path)
			b.WriteString("', { method: '")
			b.WriteString(m.HTTPMethod)
			b.WriteString("' });\n")
		} else {
			if m.OutputType == "Empty" {
				b.WriteString("(payload: ")
				b.WriteString(m.InputType)
				b.WriteString("): Promise<void> {\n")
			} else {
				b.WriteString("(payload: ")
				b.WriteString(m.InputType)
				b.WriteString("): Promise<")
				b.WriteString(m.OutputType)
				b.WriteString("> {\n")
			}
			b.WriteString("    const response = await this.#request('")
			b.WriteString(m.Path)
			b.WriteString("', { method: '")
			b.WriteString(m.HTTPMethod)
			b.WriteString("', body: encode")
			b.WriteString(m.InputType)
			b.WriteString("(payload) });\n")
		}
		b.WriteString("    if (!response.ok) {\n")
		b.WriteString("      return this.errorHandler(response);\n")
		b.WriteString("    }\n")
		if m.OutputType == "Empty" {
			b.WriteString("    await response.arrayBuffer();\n")
			b.WriteString("  }\n\n")
			continue
		}
		b.WriteString("    return decode")
		b.WriteString(m.OutputType)
		b.WriteString("(await response.arrayBuffer());\n")
		b.WriteString("  }\n\n")
	}
	b.WriteString("}\n")
	return b.String(), nil
}

func messageNameByFullName(msgIndex map[string]ir.Message, full string) (string, bool) {
	msg, ok := msgIndex[full]
	if !ok {
		if strings.HasSuffix(full, ".Empty") {
			return "Empty", true
		}
		return "", false
	}
	return msg.Name, true
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func deriveHTTP(name string) (method string, path string, ok bool) {
	prefixes := []string{"Get", "Post", "Put", "Patch", "Delete"}
	method = ""
	rest := ""
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			method = strings.ToUpper(p)
			rest = strings.TrimPrefix(name, p)
			break
		}
	}
	if method == "" || rest == "" {
		return "", "", false
	}
	underscoreParts := strings.Split(rest, "_")
	if len(underscoreParts) == 0 {
		return "", "", false
	}
	first := camelWords(underscoreParts[0])
	if len(first) == 0 {
		return "", "", false
	}
	base := "/" + strings.Join(first, "/")
	if len(underscoreParts) == 1 {
		return method, base, true
	}
	for _, seg := range underscoreParts[1:] {
		words := camelWords(seg)
		if len(words) == 0 {
			continue
		}
		base += "-" + strings.Join(words, "-")
	}
	return method, base, true
}

func camelWords(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower) || unicode.IsDigit(prev)) {
				out = append(out, strings.ToLower(b.String()))
				b.Reset()
			}
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, strings.ToLower(b.String()))
	}
	return out
}

type tsFileData struct {
	TypeDecls            []string
	Messages             []tsMessage
	NeedsReadInt64       bool
	NeedsReadInt64BigInt bool
	NeedsTimestamp       bool
	NeedsDuration        bool
	NeedsTimestampNative bool
	NeedsDurationBigInt  bool
}

type tsMessage struct {
	WriteFunc         string
	EncodeFunc        string
	DecodeMessageFunc string
	DecodeFunc        string
	NeedsTimestamp    bool
	NeedsDuration     bool
}

func buildTSFileData(file ir.File, msgIndex map[string]ir.Message) (tsFileData, error) {
	var data tsFileData
	for _, msg := range file.Messages {
		msgForTS := msg
		msgForTS.Fields = tsVisibleFields(msg.Fields)
		typedef, err := buildTSTypeDecl(msgForTS, msgIndex)
		if err != nil {
			return tsFileData{}, err
		}
		data.TypeDecls = append(data.TypeDecls, typedef)
		tsMsg, needsReadInt64, err := buildTSMessage(msgForTS, msgIndex)
		if err != nil {
			return tsFileData{}, err
		}
		if needsReadInt64 {
			data.NeedsReadInt64 = true
		}
		if tsMsg.NeedsTimestamp {
			data.NeedsTimestamp = true
		}
		if tsMsg.NeedsDuration {
			data.NeedsDuration = true
		}
		for _, field := range msgForTS.Fields {
			effType := tsEffectiveType(field)
			if effType == "bigint" && (field.Kind == ir.KindInt64 || field.IsTimestamp || field.IsDuration) {
				data.NeedsReadInt64BigInt = true
			}
			if effType != "" && field.IsTimestamp {
				data.NeedsTimestampNative = true
			}
			if effType == "bigint" && field.IsDuration {
				data.NeedsDurationBigInt = true
			}
		}
		data.Messages = append(data.Messages, tsMsg)
	}
	return data, nil
}

func buildTSTypeDecl(msg ir.Message, msgIndex map[string]ir.Message) (string, error) {
	var b strings.Builder
	if ok, field := tsIsRepeatedWrapper(msg); ok {
		elemType, err := tsWrapperElemType(field, msgIndex)
		if err != nil {
			return "", err
		}
		b.WriteString("export type ")
		b.WriteString(msg.Name)
		b.WriteString(" = ")
		b.WriteString(elemType)
		b.WriteString("[];")
		return b.String(), nil
	}
	b.WriteString("export interface ")
	b.WriteString(msg.Name)
	b.WriteString(" {\n")
	for _, field := range msg.Fields {
		typeName, err := tsTypeForDecl(field, msgIndex)
		if err != nil {
			return "", err
		}
		b.WriteString("  ")
		b.WriteString(field.Name)
		if field.IsOptional {
			b.WriteString("?")
		}
		b.WriteString(": ")
		b.WriteString(typeName)
		b.WriteString(";\n")
	}
	b.WriteString("}")
	return b.String(), nil
}

func buildTSMessage(msg ir.Message, msgIndex map[string]ir.Message) (tsMessage, bool, error) {
	writeFunc, needsReadInt64, needsTimestampWrite, needsDurationWrite, err := buildWriteFunc(msg, msgIndex)
	if err != nil {
		return tsMessage{}, false, err
	}
	encodeFunc := buildEncodeFunc(msg)
	decodeMessageFunc, needsReadInt64Decode, needsTimestampDecode, needsDurationDecode, err := buildDecodeMessageFunc(msg, msgIndex)
	if err != nil {
		return tsMessage{}, false, err
	}
	decodeFunc := buildDecodeFunc(msg)
	return tsMessage{
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
	fmt.Fprintf(&b, "export function write%s(message: %s, writer: Writer): void {\n", msg.Name, msg.Name)
	if ok, field := tsIsRepeatedWrapper(msg); ok {
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
		lines, err := tsEncodeField(field, msgIndex, "item", "            ")
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
		if !field.TsEncode {
			continue
		}
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
			b.WriteString(tsMapKeyCast(field.MapKeyKind))
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
			lines, err := tsEncodeField(field, msgIndex, "item", "            ")
			if err != nil {
				return "", false, false, false, err
			}
			b.WriteString(lines)
			b.WriteString("        }\n")
			b.WriteString("    }\n")
			continue
		}
		cond := tsPresenceCheck(field, fieldName)
		if cond != "" {
			b.WriteString("    if (")
			b.WriteString(cond)
			b.WriteString(") {\n")
		}
		lines, err := tsEncodeField(field, msgIndex, fieldName, "        ")
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
		if !field.TsEncode {
			continue
		}
		if isTSReadInt64(field) {
			needsReadInt64 = true
			break
		}
	}
	return b.String(), needsReadInt64, needsTimestamp, needsDuration, nil
}

func buildEncodeFunc(msg ir.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export function encode%s(message: %s): Uint8Array {\n", msg.Name, msg.Name)
	b.WriteString("    const writer = Writer.create();\n")
	fmt.Fprintf(&b, "    write%s(message, writer);\n", msg.Name)
	b.WriteString("    return writer.finish();\n")
	b.WriteString("}\n")
	return b.String()
}

func buildDecodeFunc(msg ir.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export function decode%s(buffer: ArrayBuffer): %s {\n", msg.Name, msg.Name)
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
	fmt.Fprintf(&b, "function decode%sMessage(reader: Reader, length?: number): %s {\n", msg.Name, msg.Name)
	b.WriteString("    const end = length === undefined ? reader.len : reader.pos + length;\n")
	if ok, field := tsIsRepeatedWrapper(msg); ok {
		b.WriteString("    const message: ")
		b.WriteString(msg.Name)
		b.WriteString(" = [];\n")
		b.WriteString("    while (reader.pos < end) {\n")
		b.WriteString("        const tag = reader.uint32();\n")
		b.WriteString("        switch (tag >>> 3) {\n")
		b.WriteString("            case ")
		b.WriteString(fmt.Sprintf("%d", field.Number))
		b.WriteString(": {\n")
		lines, usesReadInt64, usesTimestamp, err := tsDecodeWrapperField(field, msgIndex)
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
	b.WriteString("    const message: ")
	b.WriteString(msg.Name)
	b.WriteString(" = {")
	for i, field := range msg.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(tsDefaultValue(field, msgIndex))
	}
	b.WriteString(" };\n")
	b.WriteString("    while (reader.pos < end) {\n")
	b.WriteString("        const tag = reader.uint32();\n")
	b.WriteString("        switch (tag >>> 3) {\n")
	for _, field := range msg.Fields {
		b.WriteString("            case ")
		b.WriteString(fmt.Sprintf("%d", field.Number))
		b.WriteString(": {\n")
		lines, usesReadInt64, usesTimestamp, err := tsDecodeField(field, msgIndex, "message")
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

func tsTypeForDecl(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	if field.IsMap {
		valueType, err := tsMapValueType(field, msgIndex)
		if err != nil {
			return "", err
		}
		return "Record<string, " + valueType + ">", nil
	}
	t, err := tsBaseType(field, msgIndex)
	if err != nil {
		return "", err
	}
	if field.IsRepeated {
		return t + "[]", nil
	}
	return t, nil
}

func tsDefaultValue(field ir.Field, msgIndex map[string]ir.Message) string {
	if field.IsMap {
		return "{}"
	}
	if field.IsRepeated {
		return "[]"
	}
	if field.TSType == "bigint" {
		if field.IsOptional {
			return "undefined"
		}
		return "0n"
	}
	if field.TSType == "number" {
		if field.IsOptional {
			return "undefined"
		}
		return "0"
	}
	if field.TSType == "Date" {
		if field.IsOptional {
			return "undefined"
		}
		return "new Date(0)"
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
	if field.Kind == ir.KindInt64 || field.Kind == ir.KindUint64 || field.Kind == ir.KindSint64 || field.Kind == ir.KindFixed64 || field.Kind == ir.KindSfixed64 {
		if field.IsOptional {
			return "undefined"
		}
		return "0n"
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

func tsBaseType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	if field.TSType != "" {
		return field.TSType, nil
	}
	if field.IsTimestamp {
		return "Date", nil
	}
	if field.IsDuration {
		return "number", nil
	}
	if field.Kind == ir.KindInt64 || field.Kind == ir.KindUint64 || field.Kind == ir.KindSint64 || field.Kind == ir.KindFixed64 || field.Kind == ir.KindSfixed64 {
		return "bigint", nil
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

func tsEffectiveType(field ir.Field) string {
	if field.TSType != "" {
		return field.TSType
	}
	if field.Kind == ir.KindInt64 || field.Kind == ir.KindUint64 || field.Kind == ir.KindSint64 || field.Kind == ir.KindFixed64 || field.Kind == ir.KindSfixed64 {
		return "bigint"
	}
	return ""
}

func tsPresenceCheck(field ir.Field, name string) string {
	if field.IsOptional {
		return name + " !== undefined && " + name + " !== null"
	}
	if field.TSType == "bigint" {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0n"
	}
	if field.TSType == "number" {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0"
	}
	if field.TSType == "Date" {
		return name + " instanceof Date && " + name + ".getTime() !== 0"
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
	if field.Kind == ir.KindInt64 || field.Kind == ir.KindUint64 || field.Kind == ir.KindSint64 || field.Kind == ir.KindFixed64 || field.Kind == ir.KindSfixed64 {
		return name + " !== undefined && " + name + " !== null && " + name + " !== 0n"
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

func tsEncodeField(field ir.Field, msgIndex map[string]ir.Message, name, indent string) (string, error) {
	var b strings.Builder
	effType := tsEffectiveType(field)
	if effType != "" {
		nativeField := field
		nativeField.TSType = effType
		lines, err := tsEncodeNativeField(nativeField, name, indent)
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

func tsDecodeField(field ir.Field, msgIndex map[string]ir.Message, target string) (string, bool, bool, error) {
	var b strings.Builder
	fieldName := target + "." + field.Name
	effType := tsEffectiveType(field)
	if effType != "" {
		nativeField := field
		nativeField.TSType = effType
		lines, needsReadInt64, err := tsDecodeNativeField(nativeField, fieldName)
		if err != nil {
			return "", false, false, err
		}
		b.WriteString(lines)
		return b.String(), needsReadInt64, false, nil
	}
	if field.IsMap {
		mapLines, needsReadInt64, err := tsDecodeMapField(fieldName, field, msgIndex)
		if err != nil {
			return "", false, false, err
		}
		b.WriteString(mapLines)
		return b.String(), needsReadInt64, false, nil
	}
	if field.IsRepeated {
		if field.IsTimestamp {
			lines, needsReadInt64 := tsDecodeTimestampRepeated(fieldName, field)
			b.WriteString(lines)
			return b.String(), needsReadInt64, true, nil
		}
		if field.IsDuration {
			lines, needsReadInt64 := tsDecodeDurationRepeated(fieldName)
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
			packedLines, needsReadInt64 := tsDecodePackedField(fieldName, field)
			b.WriteString(packedLines)
			return b.String(), needsReadInt64, false, nil
		}
		if isTSReadInt64(field) {
			fmt.Fprintf(&b, "                %s.push(readInt64(reader, \"%s\"));\n", fieldName, jsReaderMethod(field.Kind))
			return b.String(), true, false, nil
		}
		fmt.Fprintf(&b, "                %s.push(reader.%s());\n", fieldName, jsReaderMethod(field.Kind))
		return b.String(), false, false, nil
	}
	if field.IsTimestamp {
		lines, needsReadInt64 := tsDecodeTimestampSingle(fieldName, field)
		b.WriteString(lines)
		return b.String(), needsReadInt64, true, nil
	}
	if field.IsDuration {
		lines, needsReadInt64 := tsDecodeDurationSingle(fieldName)
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
	if isTSReadInt64(field) {
		fmt.Fprintf(&b, "                %s = readInt64(reader, \"%s\");\n", fieldName, jsReaderMethod(field.Kind))
		return b.String(), true, false, nil
	}
	fmt.Fprintf(&b, "                %s = reader.%s();\n", fieldName, jsReaderMethod(field.Kind))
	return b.String(), false, false, nil
}

func tsEncodeNativeField(field ir.Field, name, indent string) (string, error) {
	var b strings.Builder
	switch field.TSType {
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
	case "Date":
		if field.IsTimestamp {
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.LDELIM)).fork();\n", indent, field.Number)
			fmt.Fprintf(&b, "%swriteTimestamp(%s, writer);\n", indent, name)
			fmt.Fprintf(&b, "%swriter.ldelim();\n", indent)
			return b.String(), nil
		}
		switch field.Kind {
		case ir.KindInt32:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int32(Math.trunc(%s.getTime() / 1000));\n", indent, field.Number, name)
			return b.String(), nil
		case ir.KindInt64:
			fmt.Fprintf(&b, "%swriter.uint32(tag(%d, WIRE.VARINT)).int64(Math.trunc(%s.getTime() / 1000));\n", indent, field.Number, name)
			return b.String(), nil
		}
	}
	return "", fmt.Errorf("unsupported js native type conversion for field: %s", field.Name)
}

func tsDecodeNativeField(field ir.Field, fieldName string) (string, bool, error) {
	var b strings.Builder
	if field.IsRepeated {
		if field.Kind == ir.KindInt64 {
			if field.IsPacked {
				b.WriteString("                const end2 = reader.uint32() + reader.pos;\n")
				b.WriteString("                while (reader.pos < end2) {\n")
				if field.TSType == "bigint" {
					b.WriteString("                    ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64BigInt(reader, \"int64\"));\n")
				} else if field.TSType == "Date" {
					b.WriteString("                    ")
					b.WriteString(fieldName)
					b.WriteString(".push(new Date(readInt64(reader, \"int64\") * 1000));\n")
				} else {
					b.WriteString("                    ")
					b.WriteString(fieldName)
					b.WriteString(".push(readInt64(reader, \"int64\"));\n")
				}
				b.WriteString("                }\n")
				return b.String(), true, nil
			}
			if field.TSType == "bigint" {
				b.WriteString("                ")
				b.WriteString(fieldName)
				b.WriteString(".push(readInt64BigInt(reader, \"int64\"));\n")
			} else if field.TSType == "Date" {
				b.WriteString("                ")
				b.WriteString(fieldName)
				b.WriteString(".push(new Date(readInt64(reader, \"int64\") * 1000));\n")
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
			if field.TSType == "bigint" {
				b.WriteString(".push(decodeTimestampBigIntMessage(reader, reader.uint32()));\n")
				return b.String(), true, nil
			}
			if field.TSType == "Date" {
				b.WriteString(".push(decodeTimestampMessage(reader, reader.uint32()));\n")
				return b.String(), true, nil
			}
			b.WriteString(".push(decodeTimestampMillisMessage(reader, reader.uint32()));\n")
			return b.String(), true, nil
		}
		if field.IsDuration {
			b.WriteString("                ")
			b.WriteString(fieldName)
			if field.TSType == "bigint" {
				b.WriteString(".push(decodeDurationBigIntMessage(reader, reader.uint32()));\n")
				return b.String(), true, nil
			}
			b.WriteString(".push(decodeDurationMessage(reader, reader.uint32()));\n")
			return b.String(), true, nil
		}
		if field.Kind == ir.KindInt32 {
			b.WriteString("                ")
			b.WriteString(fieldName)
			if field.TSType == "bigint" {
				b.WriteString(".push(BigInt(reader.int32()));\n")
			} else if field.TSType == "Date" {
				b.WriteString(".push(new Date(reader.int32() * 1000));\n")
			} else {
				b.WriteString(".push(reader.int32());\n")
			}
			return b.String(), false, nil
		}
	}

	if field.IsTimestamp {
		if field.TSType == "bigint" {
			return "                " + fieldName + " = decodeTimestampBigIntMessage(reader, reader.uint32());\n", true, nil
		}
		if field.TSType == "Date" {
			return "                " + fieldName + " = decodeTimestampMessage(reader, reader.uint32());\n", true, nil
		}
		return "                " + fieldName + " = decodeTimestampMillisMessage(reader, reader.uint32());\n", true, nil
	}
	if field.IsDuration {
		if field.TSType == "bigint" {
			return "                " + fieldName + " = decodeDurationBigIntMessage(reader, reader.uint32());\n", true, nil
		}
		return "                " + fieldName + " = decodeDurationMessage(reader, reader.uint32());\n", true, nil
	}
	if field.Kind == ir.KindInt64 {
		if field.TSType == "bigint" {
			return "                " + fieldName + " = readInt64BigInt(reader, \"int64\");\n", true, nil
		}
		if field.TSType == "Date" {
			return "                " + fieldName + " = new Date(readInt64(reader, \"int64\") * 1000);\n", true, nil
		}
		return "                " + fieldName + " = readInt64(reader, \"int64\");\n", true, nil
	}
	if field.Kind == ir.KindInt32 {
		if field.TSType == "bigint" {
			return "                " + fieldName + " = BigInt(reader.int32());\n", false, nil
		}
		if field.TSType == "Date" {
			return "                " + fieldName + " = new Date(reader.int32() * 1000);\n", false, nil
		}
		return "                " + fieldName + " = reader.int32();\n", false, nil
	}
	return "", false, fmt.Errorf("unsupported js native type conversion for field: %s", field.Name)
}

func isTSReadInt64(field ir.Field) bool {
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

func tsMapKeyCast(kind ir.Kind) string {
	switch kind {
	case ir.KindString:
		return "rawKey"
	case ir.KindBool:
		return "rawKey === \"true\""
	default:
		return "Number(rawKey)"
	}
}

func tsMapValueType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
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
	case ir.KindInt64, ir.KindUint64, ir.KindSint64, ir.KindFixed64, ir.KindSfixed64:
		return "bigint", nil
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
	cond := tsMapValuePresence(field.MapValueKind)
	valueExpr := "value"
	if field.MapValueKind == ir.KindInt64 || field.MapValueKind == ir.KindUint64 || field.MapValueKind == ir.KindSint64 || field.MapValueKind == ir.KindFixed64 || field.MapValueKind == ir.KindSfixed64 {
		valueExpr = "value.toString()"
	}
	if cond != "" {
		b.WriteString("            if (")
		b.WriteString(cond)
		b.WriteString(") {\n")
		b.WriteString("                writer.uint32(tag(2, ")
		b.WriteString(jsWireType(field.MapValueKind))
		b.WriteString(")).")
		b.WriteString(method)
		b.WriteString("(")
		b.WriteString(valueExpr)
		b.WriteString(");\n")
		b.WriteString("            }\n")
	} else {
		b.WriteString("            writer.uint32(tag(2, ")
		b.WriteString(jsWireType(field.MapValueKind))
		b.WriteString(")).")
		b.WriteString(method)
		b.WriteString("(")
		b.WriteString(valueExpr)
		b.WriteString(");\n")
	}
	return b.String(), nil
}

func tsMapValuePresence(kind ir.Kind) string {
	switch kind {
	case ir.KindString:
		return "value !== undefined && value !== null && value !== \"\""
	case ir.KindBool:
		return "value === true"
	case ir.KindInt64, ir.KindUint64, ir.KindSint64, ir.KindFixed64, ir.KindSfixed64:
		return "value !== undefined && value !== null && value !== 0n"
	default:
		return "value !== undefined && value !== null && value !== 0"
	}
}

func tsDecodeMapField(fieldName string, field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	var b strings.Builder
	needsReadInt64 := false
	b.WriteString("                const end2 = reader.uint32() + reader.pos;\n")
	b.WriteString("                let key = ")
	b.WriteString(tsMapKeyDefault(field.MapKeyKind))
	b.WriteString(";\n")
	b.WriteString("                let value = ")
	b.WriteString(tsMapValueDefault(field, msgIndex))
	b.WriteString(";\n")
	b.WriteString("                while (reader.pos < end2) {\n")
	b.WriteString("                    const tag2 = reader.uint32();\n")
	b.WriteString("                    switch (tag2 >>> 3) {\n")
	b.WriteString("                        case 1:\n")
	keyRead, keyNeedsReadInt64 := tsReadValue(field.MapKeyKind, msgIndex, "key")
	if keyNeedsReadInt64 {
		needsReadInt64 = true
	}
	b.WriteString(keyRead)
	b.WriteString("                            break;\n")
	b.WriteString("                        case 2:\n")
	valueRead, valueNeedsReadInt64, err := tsReadMapValue(field, msgIndex)
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

func tsReadMapValue(field ir.Field, msgIndex map[string]ir.Message) (string, bool, error) {
	if field.MapValueKind == ir.KindMessage {
		msg, ok := msgIndex[field.MapValueMessage]
		if !ok {
			return "", false, fmt.Errorf("unknown map value message: %s", field.MapValueMessage)
		}
		return "                            value = decode" + msg.Name + "Message(reader, reader.uint32());\n", false, nil
	}
	if isTSReadInt64(ir.Field{Kind: field.MapValueKind}) {
		return "                            value = readInt64BigInt(reader, \"" + jsReaderMethod(field.MapValueKind) + "\");\n", true, nil
	}
	return "                            value = reader." + jsReaderMethod(field.MapValueKind) + "();\n", false, nil
}

func tsReadValue(kind ir.Kind, msgIndex map[string]ir.Message, target string) (string, bool) {
	if isTSReadInt64(ir.Field{Kind: kind}) {
		return "                            " + target + " = readInt64(reader, \"" + jsReaderMethod(kind) + "\");\n", true
	}
	return "                            " + target + " = reader." + jsReaderMethod(kind) + "();\n", false
}

func tsMapKeyDefault(kind ir.Kind) string {
	switch kind {
	case ir.KindBool:
		return "false"
	case ir.KindString:
		return "\"\""
	default:
		return "0"
	}
}

func tsMapValueDefault(field ir.Field, msgIndex map[string]ir.Message) string {
	switch field.MapValueKind {
	case ir.KindBool:
		return "false"
	case ir.KindString:
		return "\"\""
	case ir.KindBytes:
		return "new Uint8Array(0)"
	case ir.KindMessage:
		return "undefined"
	case ir.KindInt64, ir.KindUint64, ir.KindSint64, ir.KindFixed64, ir.KindSfixed64:
		return "0n"
	default:
		return "0"
	}
}

func tsDecodePackedField(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	needsReadInt64 := isTSReadInt64(field)
	b.WriteString("                const end2 = reader.uint32() + reader.pos;\n")
	b.WriteString("                while (reader.pos < end2) {\n")
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

func tsDecodeTimestampSingle(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(" = decodeTimestampMessage(reader, reader.uint32());\n")
	return b.String(), true
}

func tsDecodeTimestampRepeated(fieldName string, field ir.Field) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(".push(decodeTimestampMessage(reader, reader.uint32()));\n")
	return b.String(), true
}

func tsDecodeTimestampWrapper(field ir.Field) (string, bool) {
	return tsDecodeTimestampRepeated("message", field)
}

func tsDecodeDurationSingle(fieldName string) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(" = decodeDurationMessage(reader, reader.uint32());\n")
	return b.String(), true
}

func tsDecodeDurationRepeated(fieldName string) (string, bool) {
	var b strings.Builder
	b.WriteString("                ")
	b.WriteString(fieldName)
	b.WriteString(".push(decodeDurationMessage(reader, reader.uint32()));\n")
	return b.String(), true
}

func tsDecodeDurationWrapper() (string, bool) {
	return tsDecodeDurationRepeated("message")
}

func tsIsRepeatedWrapper(msg ir.Message) (bool, ir.Field) {
	if len(msg.Fields) != 1 {
		return false, ir.Field{}
	}
	field := msg.Fields[0]
	if field.IsRepeated && !field.IsMap {
		return true, field
	}
	return false, ir.Field{}
}

func tsVisibleFields(fields []ir.Field) []ir.Field {
	visible := make([]ir.Field, 0, len(fields))
	for _, field := range fields {
		if field.TsIgnore {
			continue
		}
		visible = append(visible, field)
	}
	return visible
}

func tsWrapperElemType(field ir.Field, msgIndex map[string]ir.Message) (string, error) {
	baseField := ir.Field{
		Kind:            field.Kind,
		MessageFullName: field.MessageFullName,
		EnumFullName:    field.EnumFullName,
	}
	return tsBaseType(baseField, msgIndex)
}

func tsDecodeWrapperField(field ir.Field, msgIndex map[string]ir.Message) (string, bool, bool, error) {
	effType := tsEffectiveType(field)
	if effType != "" {
		nativeField := field
		nativeField.TSType = effType
		lines, needsReadInt64, err := tsDecodeNativeField(nativeField, "message")
		if err != nil {
			return "", false, false, err
		}
		return lines, needsReadInt64, false, nil
	}
	if field.IsTimestamp {
		lines, needsReadInt64 := tsDecodeTimestampWrapper(field)
		return lines, needsReadInt64, true, nil
	}
	if field.IsDuration {
		lines, needsReadInt64 := tsDecodeDurationWrapper()
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
		lines, needsReadInt64 := tsDecodePackedField("message", field)
		return lines, needsReadInt64, false, nil
	}
	if isTSReadInt64(field) {
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
