package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jptrs93/cleanproto/internal/generate"
	gogen "github.com/jptrs93/cleanproto/internal/generate/go"
	jsg "github.com/jptrs93/cleanproto/internal/generate/js"
	"github.com/jptrs93/cleanproto/internal/parser"
)

type stringList []string

func (s *stringList) String() string {
	return fmt.Sprint([]string(*s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var importPaths stringList
	var goOut string
	var goPkg string
	var jsOut string

	flag.Var(&importPaths, "proto_path", "proto import path (repeatable)")
	flag.StringVar(&goOut, "go_out", "", "output directory for Go")
	flag.StringVar(&goPkg, "go_pkg", "", "Go package name for generated code")
	flag.StringVar(&jsOut, "js_out", "", "output directory for JS")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "no proto files provided")
		os.Exit(1)
	}
	if len(importPaths) == 0 {
		importPaths = append(importPaths, ".")
	}

	ctx := context.Background()
	p := parser.Parser{ImportPaths: importPaths}
	files, err := p.Parse(ctx, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if goOut == "" && jsOut == "" {
		hasOut := false
		for _, file := range files {
			if file.GoOut != "" || file.JsOut != "" {
				hasOut = true
				break
			}
		}
		if !hasOut {
			fmt.Fprintln(os.Stderr, "at least one of -go_out, -js_out, cleanproto.go_out, or cleanproto.js_out is required")
			os.Exit(1)
		}
	}

	options := generate.Options{
		GoPackage: goPkg,
		GoOut:     cleanPath(goOut),
		JsOut:     cleanPath(jsOut),
	}

	generators := []generate.Generator{
		gogen.Generator{},
		jsg.Generator{},
	}

	for _, gen := range generators {
		outputs, err := gen.Generate(files, options)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := generate.WriteFiles(outputs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}
