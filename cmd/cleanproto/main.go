package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"cleanproto/internal/generate"
	gogen "cleanproto/internal/generate/go"
	jsg "cleanproto/internal/generate/js"
	"cleanproto/internal/parser"
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
	if goOut == "" && jsOut == "" {
		fmt.Fprintln(os.Stderr, "at least one of -go_out or -js_out is required")
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
