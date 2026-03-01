package generate

import "github.com/jptrs93/cleanproto/internal/ir"

type OutputFile struct {
	Path    string
	Content []byte
}

type Options struct {
	GoPackage  string
	GoOut      string
	JsOut      string
	GoJSONTags string
}

type Generator interface {
	Name() string
	Generate(files []ir.File, options Options) ([]OutputFile, error)
}
