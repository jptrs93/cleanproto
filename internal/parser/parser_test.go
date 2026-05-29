package parser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseTestProto(t *testing.T, protoSource string) error {
	t.Helper()
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "demo.proto")
	if err := os.WriteFile(protoPath, []byte(protoSource), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	optionsPath := filepath.Join(dir, "options.proto")
	if err := os.WriteFile(optionsPath, []byte(optionsProtoSource), 0o644); err != nil {
		t.Fatalf("write options proto: %v", err)
	}
	p := Parser{ImportPaths: []string{dir}}
	_, err := p.Parse(context.Background(), []string{"demo.proto"})
	return err
}

func TestParseGoValueFromFieldOptions(t *testing.T) {
	const protoSource = `syntax = "proto3";

package demo;

import "options.proto";

option go_package = "demo";

message Child {
  int32 count = 1;
}

message Parent {
  Child child = 1 [(cp.go_value) = true];
}
`

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "demo.proto")
	if err := os.WriteFile(protoPath, []byte(protoSource), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	optionsPath := filepath.Join(dir, "options.proto")
	if err := os.WriteFile(optionsPath, []byte(optionsProtoSource), 0o644); err != nil {
		t.Fatalf("write options proto: %v", err)
	}

	p := Parser{ImportPaths: []string{dir}}
	files, err := p.Parse(context.Background(), []string{"demo.proto"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	field := files[0].Messages[1].Fields[0]
	if !field.GoValue {
		t.Fatalf("expected cp.go_value to set ir.Field.GoValue")
	}
}

func TestParseRejectsInvalidGoValueUsage(t *testing.T) {
	cases := []struct {
		name  string
		field string
	}{
		{name: "Scalar", field: `int32 count = 1 [(cp.go_value) = true];`},
		{name: "Repeated", field: `repeated Child child = 1 [(cp.go_value) = true];`},
		{name: "Map", field: `map<string, Child> child = 1 [(cp.go_value) = true];`},
		{name: "GoType", field: `Child child = 1 [(cp.go_type) = "time.Time", (cp.go_value) = true];`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			imports := "import \"options.proto\";\n"
			protoSource := `syntax = "proto3";

package demo;

` + imports + `
option go_package = "demo";

message Child {
  int32 count = 1;
}

message Parent {
  ` + tc.field + `
}
`
			err := parseTestProto(t, protoSource)
			if err == nil || !strings.Contains(err.Error(), "cp.go_value only applies to singular non-native message fields") {
				t.Fatalf("expected cp.go_value validation error, got %v", err)
			}
		})
	}
}

func TestParseCompressionFromMethodOptions(t *testing.T) {
	const protoSource = `syntax = "proto3";

package demo;

import "options.proto";

option go_package = "demo";

service DemoService {
	 rpc GetAutoV1(cp.Empty) returns (cp.Empty);
	 rpc GetAlwaysV1(cp.Empty) returns (cp.Empty) {
	   option (cp.compression) = COMPRESSION_MODE_ALWAYS;
	 }
	 rpc GetNeverV1(cp.Empty) returns (cp.Empty) {
	   option (cp.compression) = COMPRESSION_MODE_NEVER;
	 }
}
`

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "demo.proto")
	if err := os.WriteFile(protoPath, []byte(protoSource), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	optionsPath := filepath.Join(dir, "options.proto")
	if err := os.WriteFile(optionsPath, []byte(optionsProtoSource), 0o644); err != nil {
		t.Fatalf("write options proto: %v", err)
	}

	p := Parser{ImportPaths: []string{dir}}
	files, err := p.Parse(context.Background(), []string{"demo.proto"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(files[0].Services))
	}
	methods := files[0].Services[0].Methods
	if len(methods) != 3 {
		t.Fatalf("expected 3 methods, got %d", len(methods))
	}

	if methods[0].CompressionMode != 0 {
		t.Fatalf("expected auto compression mode, got %d", methods[0].CompressionMode)
	}
	if methods[1].CompressionMode != 1 {
		t.Fatalf("expected always compression mode, got %d", methods[1].CompressionMode)
	}
	if methods[2].CompressionMode != 2 {
		t.Fatalf("expected never compression mode, got %d", methods[2].CompressionMode)
	}
}

func TestParseURLFromMethodOptions(t *testing.T) {
	const protoSource = `syntax = "proto3";

package demo;

import "options.proto";

option go_package = "demo";

service DemoService {
	 rpc GetBooksV1(cp.Empty) returns (cp.Empty) {
	   option (cp.url) = "/v1/library/books";
	 }
}
`

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "demo.proto")
	if err := os.WriteFile(protoPath, []byte(protoSource), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	optionsPath := filepath.Join(dir, "options.proto")
	if err := os.WriteFile(optionsPath, []byte(optionsProtoSource), 0o644); err != nil {
		t.Fatalf("write options proto: %v", err)
	}

	p := Parser{ImportPaths: []string{dir}}
	files, err := p.Parse(context.Background(), []string{"demo.proto"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	methods := files[0].Services[0].Methods
	if methods[0].URL != "/v1/library/books" {
		t.Fatalf("expected URL override, got %q", methods[0].URL)
	}
}
