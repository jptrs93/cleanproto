package generate

import "cleanproto/internal/ir"

type OutputFile struct {
	Path    string
	Content []byte
}

type Options struct {
	GoPackage string
	GoOut     string
	JsOut     string
}

type Generator interface {
	Name() string
	Generate(files []ir.File, options Options) ([]OutputFile, error)
}
