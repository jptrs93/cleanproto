package tsg

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

// Node strips types but resolves ES module specifiers literally, so the
// extension-less './runtime' import is rewritten for the runnable copy.
func generateTS(t *testing.T, files []ir.File, dir string) string {
	t.Helper()
	outputs, err := (Generator{}).Generate(files, generate.Options{TsOut: dir})
	if err != nil {
		t.Fatal(err)
	}
	model := ""
	for i := range outputs {
		content := strings.ReplaceAll(string(outputs[i].Content), "from './runtime'", "from './runtime.ts'")
		if filepath.Base(outputs[i].Path) == "model.ts" {
			model = string(outputs[i].Content)
		}
		outputs[i].Content = []byte(content)
	}
	if err := generate.WriteFiles(outputs); err != nil {
		t.Fatal(err)
	}
	if model == "" {
		t.Fatal("model.ts not generated")
	}
	return model
}

func requireNodeWithTypes(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	if err := exec.Command(node, "--experimental-strip-types", "-e", "const n: number = 1;").Run(); err != nil {
		t.Skip("node cannot strip types")
	}
	return node
}

func runNodeScript(t *testing.T, dir, name string) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), script, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(requireNodeWithTypes(t), "--experimental-strip-types", "--no-warnings", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
}

func TestTSInt64KindsAsNumberAndBigInt(t *testing.T) {
	dir := t.TempDir()
	model := generateTS(t, parseWide(t), dir)
	for _, want := range []string{
		"export interface Wide {\n  num: number;\n  big: bigint;\n  nums: number[];\n  fixed: number;\n  signed: number;\n  plain: bigint;\n  sfixedBig: bigint;\n  bigs: bigint[];\n}",
		"writer.uint32(tag(1, WIRE.VARINT)).uint64(Math.trunc(message.num));",
		"writer.uint32(tag(2, WIRE.VARINT)).uint64(message.big.toString());",
		"writer.uint32(tag(4, WIRE.FIXED64)).fixed64(Math.trunc(message.fixed));",
		"writer.uint32(tag(5, WIRE.VARINT)).sint64(Math.trunc(message.signed));",
		"writer.uint32(tag(6, WIRE.VARINT)).uint64(message.plain.toString());",
		"writer.uint32(tag(7, WIRE.FIXED64)).sfixed64(message.sfixedBig.toString());",
		"const message: Wide = {num: 0, big: 0n, nums: [], fixed: 0, signed: 0, plain: 0n, sfixedBig: 0n, bigs: [] };",
		"message.num = readInt64(reader, \"uint64\");",
		"message.big = readInt64BigInt(reader, \"uint64\");",
		"message.nums.push(readInt64(reader, \"uint64\"));",
		"message.fixed = readInt64(reader, \"fixed64\");",
		"message.signed = readInt64(reader, \"sint64\");",
		"message.plain = readInt64BigInt(reader, \"uint64\");",
		"message.bigs.push(readInt64BigInt(reader, \"uint64\"));",
		"if (!Number.isSafeInteger(n)) {",
	} {
		if !strings.Contains(model, want) {
			t.Errorf("missing %q in:\n%s", want, model)
		}
	}
	requireNodeWithTypes(t)
	runNodeScript(t, dir, "int64_roundtrip.mts")
}
