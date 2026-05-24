package templates

import "embed"

//go:embed *.tmpl protowireu.go.txt runtime.ts.txt
var FS embed.FS

//go:embed protowireu.go.txt
var ProtowireUSource string

//go:embed runtime.ts.txt
var TSRuntimeSource string
