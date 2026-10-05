package jsg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jptrs93/cleanproto/internal/generate"
	gogen "github.com/jptrs93/cleanproto/internal/generate/go"
	"github.com/jptrs93/cleanproto/internal/ir"
	schemaparser "github.com/jptrs93/cleanproto/internal/parser"
)

const wideProto = `syntax = "proto3";
package wide;
option go_package = "wide";
import "options.proto";
message Wide {
  uint64 num = 1 [(cp.js_type) = "number", (cp.ts_type) = "number"];
  uint64 big = 2 [(cp.js_type) = "bigint"];
  repeated uint64 nums = 3 [(cp.js_type) = "number", (cp.ts_type) = "number"];
  fixed64 fixed = 4 [(cp.js_type) = "number", (cp.ts_type) = "number"];
  sint64 signed = 5 [(cp.js_type) = "number", (cp.ts_type) = "number"];
  uint64 plain = 6;
  sfixed64 sfixed_big = 7 [(cp.js_type) = "bigint", (cp.ts_type) = "bigint"];
  repeated uint64 bigs = 8 [packed = false, (cp.js_type) = "bigint", (cp.ts_type) = "bigint"];
}
`

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func parseChoices(t *testing.T) []ir.File {
	t.Helper()
	root := repoRoot(t)
	p := schemaparser.Parser{ImportPaths: []string{filepath.Join(root, "example"), root}}
	files, err := p.Parse(context.Background(), []string{"choices.proto"})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func parseWide(t *testing.T) []ir.File {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wide.proto"), []byte(wideProto), 0600); err != nil {
		t.Fatal(err)
	}
	p := schemaparser.Parser{ImportPaths: []string{dir, repoRoot(t)}}
	files, err := p.Parse(context.Background(), []string{"wide.proto"})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func generateJS(t *testing.T, files []ir.File, dir string) string {
	t.Helper()
	outputs, err := (Generator{}).Generate(files, generate.Options{JsOut: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := generate.WriteFiles(outputs); err != nil {
		t.Fatal(err)
	}
	for _, out := range outputs {
		if filepath.Base(out.Path) == "model.js" {
			return string(out.Content)
		}
	}
	t.Fatal("model.js not generated")
	return ""
}

func requireNode(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	return node
}

func runNode(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(requireNode(t), args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
}

func TestJSOneofGeneration(t *testing.T) {
	model := generateJS(t, parseChoices(t), t.TempDir())
	for _, want := range []string{
		" * @typedef {Object} CertSourceValueOneof\n * @property {AcmeCertSource} [acme]\n * @property {SecretCertSource} [secret]\n */",
		" * @typedef {Object} CertSource\n * @property {CertSourceValueOneof} value\n */",
		" * @property {CoverageChoiceOneof} [choice]\n",
		" * @typedef {Object} ScalarChoiceValueOneof\n * @property {boolean} [flag]\n * @property {string} [text]\n * @property {Uint8Array} [data]\n * @property {number} [status]\n * @property {Date} [updated]\n * @property {number} [delay]\n * @property {Uint8Array} [uuid]\n */",
		"const selected = [message.value.acme, message.value.secret].filter((v) => v !== undefined && v !== null).length;",
		"throw new Error(\"CertSource.value: expected exactly one alternative, got \" + selected);",
		"throw new Error(\"CertSource.value: expected exactly one alternative, got none\");",
		"throw new Error(\"ScalarChoice.value: expected at most one alternative, got \" + selected);",
		"        if (message.value.acme !== undefined && message.value.acme !== null) {\n            writer.uint32(tag(1, WIRE.LDELIM)).fork();\n            writeAcmeCertSource(message.value.acme, writer);\n            writer.ldelim();\n        }",
		"    const message = {value: {} };",
		"    const message = {count: undefined, data: undefined, updated: new Date(0), delay: 0, items: [], byName: {}, requiredChild: undefined, choice: undefined };",
		"message.value = { acme: decodeAcmeCertSourceMessage(reader, reader.uint32()) };",
		"message.choice = { number: reader.int32() };",
		"message.choice = { blob: reader.bytes() };",
		"message.value = { status: reader.int32() };",
		"message.value = { updated: decodeTimestampMessage(reader, reader.uint32()) };",
		"message.value = { delay: decodeDurationMessage(reader, reader.uint32()) };",
	} {
		if !strings.Contains(model, want) {
			t.Errorf("missing %q in:\n%s", want, model)
		}
	}
}

func TestJSOneofNodeRoundTripMatchesGo(t *testing.T) {
	requireNode(t)
	files := parseChoices(t)
	dir := t.TempDir()
	generateJS(t, files, filepath.Join(dir, "js"))

	goDir := filepath.Join(dir, "go")
	goOutputs, err := (gogen.Generator{}).Generate(files, generate.Options{GoOut: goDir, GoJSONTags: "snake"})
	if err != nil {
		t.Fatal(err)
	}
	if err := generate.WriteFiles(goOutputs); err != nil {
		t.Fatal(err)
	}
	wireMain, err := os.ReadFile("testdata/oneof_wire_main.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "cmd", "wire"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "cmd", "wire", "main.go"), wireMain, 0600); err != nil {
		t.Fatal(err)
	}
	mod := []byte("module choices.test\n\ngo 1.26.0\n\nrequire (\n github.com/google/uuid v1.6.0\n google.golang.org/protobuf v1.33.1-0.20240319125436-3039476726e4\n)\n")
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), mod, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "-mod=mod", "./cmd/wire")
	cmd.Dir = goDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	wire, err := cmd.Output()
	if err != nil {
		t.Fatalf("go wire printer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wire.json"), wire, 0600); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("testdata/oneof_roundtrip.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "oneof_roundtrip.mjs"), script, 0600); err != nil {
		t.Fatal(err)
	}
	runNode(t, dir, "oneof_roundtrip.mjs", "wire.json")
}

func TestJSInt64KindsAsNumberAndBigInt(t *testing.T) {
	dir := t.TempDir()
	model := generateJS(t, parseWide(t), filepath.Join(dir, "js"))
	for _, want := range []string{
		" * @property {number} num\n * @property {bigint} big\n * @property {number[]} nums\n * @property {number} fixed\n * @property {number} signed\n * @property {number} plain\n * @property {bigint} sfixedBig\n * @property {bigint[]} bigs\n",
		"writer.uint32(tag(1, WIRE.VARINT)).uint64(Math.trunc(message.num));",
		"writer.uint32(tag(2, WIRE.VARINT)).uint64(message.big.toString());",
		"writer.uint32(tag(4, WIRE.FIXED64)).fixed64(Math.trunc(message.fixed));",
		"writer.uint32(tag(5, WIRE.VARINT)).sint64(Math.trunc(message.signed));",
		"writer.uint32(tag(7, WIRE.FIXED64)).sfixed64(message.sfixedBig.toString());",
		"writer.uint32(tag(8, WIRE.VARINT)).uint64(item.toString());",
		"const message = {num: 0, big: 0n, nums: [], fixed: 0, signed: 0, plain: 0, sfixedBig: 0n, bigs: [] };",
		"message.num = readInt64(reader, \"uint64\");",
		"message.big = readInt64BigInt(reader, \"uint64\");",
		"message.nums.push(readInt64(reader, \"uint64\"));",
		"message.fixed = readInt64(reader, \"fixed64\");",
		"message.signed = readInt64(reader, \"sint64\");",
		"message.sfixedBig = readInt64BigInt(reader, \"sfixed64\");",
		"message.bigs.push(readInt64BigInt(reader, \"uint64\"));",
		"if (!Number.isSafeInteger(n)) {",
	} {
		if !strings.Contains(model, want) {
			t.Errorf("missing %q in:\n%s", want, model)
		}
	}
	requireNode(t)
	script, err := os.ReadFile("testdata/int64_roundtrip.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "int64_roundtrip.mjs"), script, 0600); err != nil {
		t.Fatal(err)
	}
	runNode(t, dir, "int64_roundtrip.mjs")
}
