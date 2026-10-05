package jsg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jptrs93/cleanproto/internal/generate"
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
