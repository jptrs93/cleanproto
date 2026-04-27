package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
