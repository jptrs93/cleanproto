package templates

import "embed"

//go:embed *.tmpl protowireu.go.txt runtime.ts.txt runtime.js.txt
var FS embed.FS

//go:embed protowireu.go.txt
var ProtowireUSource string

//go:embed runtime.ts.txt
var TSRuntimeSource string

//go:embed runtime.js.txt
var JSRuntimeSource string
