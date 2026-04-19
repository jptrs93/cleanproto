package gogen

import (
	"strings"
	"testing"

	"github.com/jptrs93/cleanproto/internal/ir"
)

func TestBuildGoMuxFileUsesToAuditWhenAuditModelsExist(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{
				Name:     "AuditReq",
				FullName: "example.AuditReq",
				Fields: []ir.Field{
					{Name: "visible", Kind: ir.KindString},
					{Name: "secret", Kind: ir.KindBytes, AuditIgnore: true},
				},
			},
			{
				Name:     "AuditResp",
				FullName: "example.AuditResp",
				Fields: []ir.Field{
					{Name: "visible", Kind: ir.KindString},
					{Name: "secret", Kind: ir.KindBytes, AuditIgnore: true},
				},
			},
			{
				Name:     "PlainReq",
				FullName: "example.PlainReq",
				Fields:   []ir.Field{{Name: "visible", Kind: ir.KindString}},
			},
			{
				Name:     "PlainResp",
				FullName: "example.PlainResp",
				Fields:   []ir.Field{{Name: "visible", Kind: ir.KindString}},
			},
		},
		Services: []ir.Service{{
			Name: "ExampleService",
			Methods: []ir.Method{
				{
					Name:           "PostAuditV1",
					InputFullName:  "example.AuditReq",
					OutputFullName: "example.AuditResp",
					Audit:          true,
				},
				{
					Name:           "PostPlainV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          true,
				},
			},
		}},
	}

	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}

	if !strings.Contains(mux, "audit(ctx, \"PostAuditV1\", err, req.ToAudit(), res.ToAudit())") {
		t.Fatalf("expected audited request/response payloads to use ToAudit, got:\n%s", mux)
	}
	if !strings.Contains(mux, "audit(ctx, \"PostPlainV1\", err, req, res)") {
		t.Fatalf("expected plain request/response payloads to stay unchanged, got:\n%s", mux)
	}
}
