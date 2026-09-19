package gogen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jptrs93/cleanproto/internal/generate"
	schemaparser "github.com/jptrs93/cleanproto/internal/parser"
)

// Compile and execute real generated code: string assertions alone miss type
// errors at the intersection of presence, collections, codecs and validation.
func TestPresenceGeneratedPackage(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	p := schemaparser.Parser{ImportPaths: []string{filepath.Join(root, "example"), root}}
	files, err := p.Parse(context.Background(), []string{"choices.proto"})
	if err != nil {
		t.Fatal(err)
	}
	for _, withTransport := range []bool{false, true} {
		t.Run(map[bool]string{false: "models", true: "transport"}[withTransport], func(t *testing.T) {
			dir := t.TempDir()
			outputs, err := (Generator{}).Generate(files, generate.Options{GoOut: dir, GoJSONTags: "snake", GoServer: withTransport, GoClient: withTransport, GoJSON: withTransport})
			if err != nil {
				t.Fatal(err)
			}
			if err := generate.WriteFiles(outputs); err != nil {
				t.Fatal(err)
			}
			testSource, err := os.ReadFile("testdata/presence_test.go.txt")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "presence_test.go"), testSource, 0600); err != nil {
				t.Fatal(err)
			}
			if withTransport {
				transport, err := os.ReadFile("testdata/transport_test.go.txt")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "transport_test.go"), transport, 0600); err != nil {
					t.Fatal(err)
				}
			}
			mod := []byte("module choices.test\n\ngo 1.26.0\n\nrequire (\n github.com/google/uuid v1.6.0\n google.golang.org/protobuf v1.33.1-0.20240319125436-3039476726e4\n)\n")
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-mod=mod", "./...")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated package: %v\n%s", err, out)
			}
		})
	}
}

func TestGoGenerationMergesInputsAndRejectsCollisions(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, src := range map[string]string{
		"a.proto":                `syntax="proto3"; package sample; option go_package="sample"; message A { string name=1; }`,
		"b.proto":                `syntax="proto3"; package sample; option go_package="sample"; import "a.proto"; message B { A value=1; }`,
		"plain.proto":            `syntax="proto3"; package sample; option go_package="sample"; message Some {} message None {} message Direct { oneof value { int32 kind=1; int32 unset=2; string as_kind=3; } }`,
		"collision.proto":        `syntax="proto3"; package sample; option go_package="sample"; message Maybe {}`,
		"choice_collision.proto": `syntax="proto3"; package sample; option go_package="sample"; message Parent { oneof value { int32 validate=1; } }`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p := schemaparser.Parser{ImportPaths: []string{dir, root}}
	files, err := p.Parse(context.Background(), []string{"a.proto", "b.proto"})
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := (Generator{}).Generate(files, generate.Options{GoOut: dir})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, out := range outputs {
		if filepath.Base(out.Path) == "model.gen.go" {
			count++
			if !strings.Contains(string(out.Content), "type A struct") || !strings.Contains(string(out.Content), "type B struct") {
				t.Fatal("input discarded")
			}
		}
	}
	if count != 1 {
		t.Fatal("multiple model outputs overwrite one another")
	}
	plain, err := p.Parse(context.Background(), []string{"plain.proto"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := (Generator{}).Generate(plain, generate.Options{GoOut: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range direct {
		for _, removed := range []string{"func Some[", "func None[", "func NewDirectValueOneof", "type DirectValueOneofKind", "func (v DirectValueOneof) Kind(", "func (v DirectValueOneof) As", "func (m Maybe[T]) Get(", "func (m Maybe[T]) IsPresent(", "func (m Maybe[T]) OrElse("} {
			if strings.Contains(string(out.Content), removed) {
				t.Fatalf("removed API still generated: %s", removed)
			}
		}
	}

	for _, name := range []string{"collision.proto", "choice_collision.proto"} {
		files, err := p.Parse(context.Background(), []string{name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := (Generator{}).Generate(files, generate.Options{GoOut: dir}); err == nil {
			t.Fatalf("accepted collision %s", name)
		}
	}
}
