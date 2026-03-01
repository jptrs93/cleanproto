package templates

import "embed"

//go:embed *.tmpl protowireu.go.txt
var FS embed.FS

//go:embed protowireu.go.txt
var ProtowireUSource string
