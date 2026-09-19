package gogen

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/jptrs93/cleanproto/internal/generate"
	"github.com/jptrs93/cleanproto/internal/ir"
)

// Presence is a property of the schema, independent of Go's storage decisions.
func goMessageField(f ir.Field) bool {
	return f.Kind == ir.KindMessage && !f.IsMap && !f.IsTimestamp && !f.IsDuration && f.GoType == ""
}
func goMaybeField(f ir.Field) bool {
	return f.OneofName == "" && !f.IsRepeated && !f.IsMap && !f.GoValue && (f.IsOptional || f.Kind == ir.KindMessage)
}
func oneofType(m ir.Message, name string) string { return m.Name + ir.GoName(name) + "Oneof" }
func oneofRequired(m ir.Message, name string) bool {
	for _, g := range m.Oneofs {
		if g.Name == name {
			return g.Required
		}
	}
	return false
}

func goFieldType(f ir.Field, msgs map[string]ir.Message, enums map[string]ir.Enum) (string, bool, error) {
	if f.OneofName != "" {
		base := f
		base.OneofName = ""
		base.IsOptional = false
		base.GoValue = true
		base.GoIndirect = false
		t, math, err := goWireFieldType(base, msgs, enums)
		return "*" + t, math, err
	}
	if goMaybeField(f) {
		base := f
		base.IsOptional = false
		base.GoValue = true
		t, math, err := goWireFieldType(base, msgs, enums)
		if f.GoIndirect {
			t = "*" + t
		}
		return "Maybe[" + t + "]", math, err
	}
	t, math, err := goWireFieldType(f, msgs, enums)
	if f.GoValue && f.GoIndirect {
		t = "*" + t
	}
	return t, math, err
}

// One output directory is one Go package. Merge inputs before emitting fixed
// filenames, and never silently overwrite another package's generated files.
func prepareGoFiles(files []ir.File) ([]ir.File, error) {
	if len(files) == 0 {
		return nil, nil
	}
	out := files[0]
	out.Messages = nil
	out.Enums = nil
	out.Services = nil
	seen := map[string]bool{}
	names := map[string]string{"Maybe": "runtime"}
	claim := func(name, owner string) error {
		if old, ok := names[name]; ok {
			return fmt.Errorf("generated Go name %s collides (%s and %s)", name, old, owner)
		}
		names[name] = owner
		return nil
	}
	for _, file := range files {
		if file.GoPackage != out.GoPackage {
			return nil, fmt.Errorf("one Go output directory cannot contain packages %q and %q; generate them separately", out.GoPackage, file.GoPackage)
		}
		for _, e := range file.Enums {
			if seen[e.FullName] {
				continue
			}
			seen[e.FullName] = true
			if err := claim(e.Name, e.FullName); err != nil {
				return nil, err
			}
			out.Enums = append(out.Enums, e)
		}
		for _, m := range file.Messages {
			if seen[m.FullName] {
				continue
			}
			seen[m.FullName] = true
			if err := claim(m.Name, m.FullName); err != nil {
				return nil, err
			}
			members := map[string]bool{"Encode": true, "EncodeChecked": true, "Validate": true, "IsZero": true, "ToAudit": true, "UnknownFields": true}
			for _, g := range m.Oneofs {
				n := ir.GoName(g.Name)
				if members[n] {
					return nil, fmt.Errorf("%s: conflicting member %s", m.FullName, n)
				}
				members[n] = true
				if err := claim(oneofType(m, g.Name), m.FullName+"."+g.Name); err != nil {
					return nil, err
				}
			}
			m.Fields = append([]ir.Field(nil), m.Fields...)
			for _, f := range m.Fields {
				if f.GoIgnore || f.OneofName != "" {
					continue
				}
				n := ir.GoName(f.Name)
				if members[n] {
					return nil, fmt.Errorf("%s: conflicting member %s", m.FullName, n)
				}
				members[n] = true
			}
			out.Messages = append(out.Messages, m)
		}
		out.Services = append(out.Services, file.Services...)
	}
	index := indexMessages([]ir.File{out})
	var reaches func(string, string, map[string]bool) bool
	reaches = func(from, to string, visited map[string]bool) bool {
		if from == to {
			return true
		}
		if visited[from] {
			return false
		}
		visited[from] = true
		for _, f := range index[from].Fields {
			if !f.GoIgnore && f.OneofName == "" && goMessageField(f) && !f.IsRepeated && reaches(f.MessageFullName, to, visited) {
				return true
			}
		}
		return false
	}
	for i := range out.Messages {
		m := &out.Messages[i]
		for j := range m.Fields {
			f := &m.Fields[j]
			if goMessageField(*f) && !f.IsRepeated && f.OneofName == "" {
				f.GoIndirect = reaches(f.MessageFullName, m.FullName, map[string]bool{})
			}
		}
	}
	return []ir.File{out}, nil
}

// Reuse the primitive wire emitters with a scoped pointer local. Pointers here
// are codec implementation details; presence in the public API remains Maybe.
func buildPresenceCodec(m ir.Message, msgs map[string]ir.Message, enums map[string]ir.Enum) ([]string, []goDecodeCase, bool, bool, error) {
	var enc []string
	var dec []goDecodeCase
	var needMsg, needTmp bool
	for _, f := range m.Fields {
		if f.GoIgnore {
			lines := []string{"b, err = SkipFieldValue(b, num, typ)"}
			if f.OneofName != "" {
				empty := oneofType(m, f.OneofName) + "{}"
				if !oneofRequired(m, f.OneofName) {
					empty = "Maybe[" + oneofType(m, f.OneofName) + "]{}"
				}
				lines = append(lines, "if err == nil { m."+ir.GoName(f.OneofName)+" = "+empty+" }")
			}
			dec = append(dec, goDecodeCase{Number: f.Number, Lines: lines})
			continue
		}
		name := "m." + ir.GoName(f.Name)
		adapted := goMaybeField(f) || f.OneofName != ""
		raw := f
		if adapted {
			raw.IsOptional = true
			raw.GoValue = false
		}
		raw.OneofName = ""
		// Message values have presence even when their contents are all zero.
		if f.GoValue {
			raw.GoValue = false
		}
		lines, err := buildGoEncodeLines(ir.Message{Fields: []ir.Field{raw}}, msgs, enums)
		if err != nil {
			return nil, nil, false, false, err
		}
		cases, nm, nt, err := buildGoDecodeCases(ir.Message{Fields: []ir.Field{raw}}, msgs, enums)
		if err != nil {
			return nil, nil, false, false, err
		}
		needMsg = needMsg || nm
		needTmp = needTmp || nt
		c := cases[0]
		replace := func(in []string, expr string) []string {
			out := make([]string, len(in))
			for i, l := range in {
				out[i] = strings.ReplaceAll(l, name, expr)
			}
			return out
		}
		if adapted || f.GoValue {
			typ, _, err := goWireFieldType(raw, msgs, enums)
			if err != nil {
				return nil, nil, false, false, err
			}
			var before, after, decodeBefore, decodeAfter []string
			before = append(before, "{")
			decodeBefore = append(decodeBefore, "var value "+typ)
			switch {
			case f.OneofName != "":
				group := "m." + ir.GoName(f.OneofName)
				member := ir.GoName(f.Name)
				choice := oneofType(m, f.OneofName)
				if oneofRequired(m, f.OneofName) {
					before = append(before, "value := "+group+"."+member)
					decodeBefore = append(decodeBefore, "value = "+group+"."+member)
				} else {
					before = append(before, "var choice "+choice, "if "+group+".Present { choice = "+group+".Value }", "value := choice."+member)
					decodeBefore = append(decodeBefore, "if "+group+".Present { value = "+group+".Value."+member+" }")
				}
				assignment := choice + "{" + member + ": value}"
				if !oneofRequired(m, f.OneofName) {
					assignment = "Maybe[" + choice + "]{Value: " + assignment + ", Present: true}"
				}
				decodeAfter = append(decodeAfter, "if err == nil { "+group+" = "+assignment+" }")
			case goMaybeField(f):
				publicType, _, err := goFieldType(f, msgs, enums)
				if err != nil {
					return nil, nil, false, false, err
				}
				before = append(before, "if "+name+".Present {", "v := "+name+".Value")
				if f.GoIndirect {
					before = append(before, "value := v")
					decodeBefore = append(decodeBefore, "if "+name+".Present { value = "+name+".Value }")
					decodeAfter = append(decodeAfter, "if err == nil { "+name+" = "+publicType+"{Value: value, Present: true} }")
				} else {
					before = append(before, "value := &v")
					decodeBefore = append(decodeBefore, "if "+name+".Present { v := "+name+".Value; value = &v }")
					decodeAfter = append(decodeAfter, "if err == nil { "+name+" = "+publicType+"{Value: *value, Present: true} }")
				}
				after = append(after, "}")
			default:
				if f.GoIndirect {
					before = append(before, "value := "+name)
					decodeBefore = append(decodeBefore, "value = "+name)
					decodeAfter = append(decodeAfter, "if err == nil { "+name+" = value }")
				} else {
					before = append(before, "value := &"+name)
					decodeBefore = append(decodeBefore, "value = &"+name)
					decodeAfter = append(decodeAfter, "if err == nil { "+name+" = *value }")
				}
			}
			after = append(after, "}")
			if f.GoEncode {
				enc = append(enc, before...)
				enc = append(enc, replace(lines, "value")...)
				enc = append(enc, after...)
			}
			if f.Kind == ir.KindMessage && (f.IsTimestamp || f.IsDuration) {
				native := "Timestamp"
				if f.IsDuration {
					native = "Duration"
				}
				needMsg = true
				c.Lines = []string{"b, msgBytes, err = ConsumeMessage(b, typ)", "if err == nil {", "if value != nil { msgBytes = append(Encode" + native + "(*value), msgBytes...) }", "decoded, decodeErr := Decode" + native + "(msgBytes)", "err = decodeErr", "if err == nil { value = &decoded }", "}"}
			}

			c.Lines = append(decodeBefore, replace(c.Lines, "value")...)
			c.Lines = append(c.Lines, decodeAfter...)
		} else {
			enc = append(enc, lines...)
		}
		dec = append(dec, c)
	}
	return enc, dec, needMsg, needTmp, nil
}

func buildChoiceSource(file ir.File, msgs map[string]ir.Message, enums map[string]ir.Enum, jsonTags string, audit bool) (string, error) {
	var b strings.Builder
	for _, m := range file.Messages {
		for _, g := range m.Oneofs {
			name := oneofType(m, g.Name)
			fmt.Fprintf(&b, "\n%stype %s struct {\n", goDoc(g.Doc), name)
			fields := []ir.Field{}
			members := map[string]bool{"Validate": true, "MarshalJSON": true}
			for _, f := range m.Fields {
				if f.OneofName != g.Name || f.GoIgnore {
					continue
				}
				n := ir.GoName(f.Name)
				if members[n] {
					return "", fmt.Errorf("%s: conflicting choice member %s", name, n)
				}
				members[n] = true
				fields = append(fields, f)
				typ, _, err := goFieldType(f, msgs, enums)
				if err != nil {
					return "", err
				}
				tag := ""
				if jsonTags == "snake" {
					tag = " `json:\"" + f.ProtoName + ",omitempty\"`"
				}
				if f.JSONIgnore {
					tag = " `json:\"-\"`"
				}
				fmt.Fprintf(&b, "%s%s %s%s\n", goDoc(f.Doc), n, typ, tag)
			}
			b.WriteString("}\n")

			fmt.Fprintf(&b, "func (v %s) MarshalJSON() ([]byte,error) {\n", name)
			if !audit {
				b.WriteString("if err := v.Validate(); err != nil { return nil,err }\n")
				for _, f := range fields {
					if goMessageField(f) {
						n := ir.GoName(f.Name)
						fmt.Fprintf(&b, "if v.%s != nil { if err := v.%s.Validate(); err != nil { return nil,err } }\n", n, n)
					}
				}
			}
			for _, f := range fields {
				if f.Kind == ir.KindBytes && f.GoType != "github.com/google/uuid.UUID" {
					n := ir.GoName(f.Name)
					typ, _, _ := goFieldType(f, msgs, enums)
					base := strings.TrimPrefix(typ, "*")
					fmt.Fprintf(&b, "if v.%s != nil && *v.%s == nil { empty := %s{}; v.%s = &empty }\n", n, n, base, n)
				}
			}
			fmt.Fprintf(&b, "type jsonValue %s\nreturn json.Marshal(jsonValue(v))\n}\n", name)
			fmt.Fprintf(&b, "func (v %s) Validate() error {\nselected := 0\n", name)
			for _, f := range fields {
				fmt.Fprintf(&b, "if v.%s != nil { selected++ }\n", ir.GoName(f.Name))
			}
			b.WriteString("if selected != 1 { return newValidationError(nil, \"exactly one alternative must be selected\") }; return nil\n}\n")

		}
	}
	return b.String(), nil
}

const maybeSource = `
import ("encoding/json"; "reflect")

// Maybe separates absence from a present zero value. Its zero value is absent.
// Maybe is emitted once per generated package, keeping generated code standalone.
// Value is ignored when Present is false.
type Maybe[T any] struct { Value T; Present bool }
// IsZero supports encoding/json's omitzero option.
func (m Maybe[T]) IsZero() bool { return !m.Present }
func (m Maybe[T]) MarshalJSON() ([]byte, error) {
 if !m.Present { return []byte("null"), nil }
 // Custom marshalers start a fresh encoding/json traversal. Validate generated
 // payloads first, including recursive pointers, so a cycle cannot recurse
 // through successive Maybe marshalers indefinitely.
 if v, ok := any(m.Value).(interface{ Validate() error }); ok {
  if err := v.Validate(); err != nil { return nil, err }
 } else if v, ok := any(&m.Value).(interface{ Validate() error }); ok {
  if err := v.Validate(); err != nil { return nil, err }
 }
 v := reflect.ValueOf(m.Value)
 if v.IsValid() && v.Kind() == reflect.Slice && v.IsNil() { return json.Marshal(reflect.MakeSlice(v.Type(), 0, 0).Interface()) }
 if v.IsValid() && v.Kind() == reflect.Map && v.IsNil() { return json.Marshal(reflect.MakeMap(v.Type()).Interface()) }
 return json.Marshal(m.Value) }
// JSON null denotes absence. Use a wrapper message to model a selected null payload.
func (m *Maybe[T]) UnmarshalJSON(b []byte) error {
 if string(b) == "null" { *m = Maybe[T]{}; return nil }
 var v T
 if err := json.Unmarshal(b, &v); err != nil { return err }
 *m = Maybe[T]{Value: v, Present: true}; return nil
}
`

func (g *validateGen) emitPresenceField(b *strings.Builder, m ir.Message, f ir.Field) error {
	name := "m." + ir.GoName(f.Name)
	raw := f
	if goMaybeField(f) || f.OneofName != "" || f.GoValue {
		b.WriteString("{\n")
		switch {
		case f.OneofName != "":
			group := "m." + ir.GoName(f.OneofName)
			if oneofRequired(m, f.OneofName) {
				fmt.Fprintf(b, "value := %s.%s\n", group, ir.GoName(f.Name))
			} else {
				fmt.Fprintf(b, "var choice %s\nif %s.Present { choice = %s.Value }\nvalue := choice.%s\n", oneofType(m, f.OneofName), group, group, ir.GoName(f.Name))
			}
			raw.Constraints.Required = false
		case goMaybeField(f):
			fmt.Fprintf(b, "v, present := %s.Value, %s.Present\n", name, name)
			typ, _, err := goFieldType(f, g.msgIndex, g.enumIndex)
			if err != nil {
				return err
			}
			typ = strings.TrimSuffix(strings.TrimPrefix(typ, "Maybe["), "]")
			if f.GoIndirect {
				fmt.Fprintf(b, "var value %s\nif present { value = v; if value == nil { return newValidationError([]string{%q}, \"present message cannot be nil\") } }\n", typ, fieldProtoName(f))
			} else {
				fmt.Fprintf(b, "var value *%s\nif present { value = &v }\n", typ)
			}
		default:
			if f.GoIndirect {
				fmt.Fprintf(b, "value := %s\nif value == nil { return newValidationError([]string{%q}, \"message is required\") }\n", name, fieldProtoName(f))
			} else {
				fmt.Fprintf(b, "value := &%s\n", name)
			}
		}
		raw.IsOptional = true
		raw.GoValue = false
		var inner strings.Builder
		if err := g.emitField(&inner, raw); err != nil {
			return err
		}
		b.WriteString(strings.ReplaceAll(inner.String(), name, "value"))
		b.WriteString("_ = value\n}\n")
		return nil
	}
	return g.emitField(b, f)
}

// Check the entire generated namespace, including helper functions and methods,
// before writing any files. This catches collisions introduced by schema names.
func checkGoDeclarations(outputs []generate.OutputFile) error {
	seen := map[string]string{}
	var receiverName func(ast.Expr) string
	receiverName = func(e ast.Expr) string {
		switch e := e.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.StarExpr:
			return receiverName(e.X)
		case *ast.IndexExpr:
			return receiverName(e.X)
		case *ast.IndexListExpr:
			return receiverName(e.X)
		}
		return ""
	}
	for _, output := range outputs {
		if filepath.Ext(output.Path) != ".go" {
			continue
		}
		file, err := goparser.ParseFile(token.NewFileSet(), output.Path, output.Content, 0)
		if err != nil {
			return fmt.Errorf("generated Go syntax: %w", err)
		}
		claim := func(name string) error {
			if name == "_" || name == "init" {
				return nil
			}
			if old, ok := seen[name]; ok {
				return fmt.Errorf("generated Go declaration %s conflicts in %s and %s; rename the schema item", name, old, output.Path)
			}
			seen[name] = output.Path
			return nil
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if d.Recv != nil {
					name = receiverName(d.Recv.List[0].Type) + "." + name
				}
				if err := claim(name); err != nil {
					return err
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if err := claim(spec.Name.Name); err != nil {
							return err
						}
					case *ast.ValueSpec:
						for _, n := range spec.Names {
							if err := claim(n.Name); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func goDoc(doc string) string {
	if strings.TrimSpace(doc) == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(doc, "\n") {
		b.WriteString("// ")
		b.WriteString(strings.TrimSpace(line))
		b.WriteByte('\n')
	}
	return b.String()
}
