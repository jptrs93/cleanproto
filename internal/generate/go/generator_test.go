package gogen

import (
	"go/parser"
	"go/token"
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

	if !strings.Contains(mux, "audit(authCtx, \"PostAuditV1\", err, req.ToAudit(), res.ToAudit())") {
		t.Fatalf("expected audited request/response payloads to use ToAudit, got:\n%s", mux)
	}
	if !strings.Contains(mux, "audit(authCtx, \"PostPlainV1\", err, req, res)") {
		t.Fatalf("expected plain request/response payloads to stay unchanged, got:\n%s", mux)
	}
}

func TestBuildGoMuxFileAddsCompressionOptionsAndRouteModes(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "Reply",
			FullName: "example.Reply",
			Fields:   []ir.Field{{Name: "value", Kind: ir.KindString}},
		}},
		Services: []ir.Service{{
			Name: "ExampleService",
			Methods: []ir.Method{
				{Name: "GetAutoV1", InputFullName: "cp.Empty", OutputFullName: "example.Reply"},
				{Name: "GetAlwaysV1", InputFullName: "cp.Empty", OutputFullName: "example.Reply", CompressionMode: 1},
				{Name: "GetNeverV1", InputFullName: "cp.Empty", OutputFullName: "example.Reply", CompressionMode: 2},
				{Name: "GetStreamAlwaysV1", InputFullName: "cp.Empty", OutputFullName: "example.Reply", CompressionMode: 1, IsStreamingServer: true},
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

	checks := []string{
		"type MuxConfig struct",
		"type VerifyAuthFunc func(context.Context, http.ResponseWriter, *http.Request, AccessPolicy) (context.Context, error)",
		"type PostAuthHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)",
		"type PostAuthMiddlewareFunc func(next PostAuthHandlerFunc) PostAuthHandlerFunc",
		"VerifyAuth          VerifyAuthFunc",
		"Audit               AuditFunc",
		"Middlewares         []MiddlewareFunc",
		"PostAuthMiddlewares []PostAuthMiddlewareFunc",
		"func CreateMux(h ServerHandler, config *MuxConfig) *http.ServeMux",
		"Compression         *CompressionOptions",
		"verifyAuth := config.VerifyAuth",
		"func ApplyPostAuthMiddlewares(h PostAuthHandlerFunc, middlewares ...PostAuthMiddlewareFunc) PostAuthHandlerFunc",
		"func buildHandlerFunc(config *MuxConfig, verifyAuth VerifyAuthFunc, policy AccessPolicy, postAuthHandler PostAuthHandlerFunc, compressionMode int32, streaming bool) http.HandlerFunc",
		"authCtx, err := verifyAuth(ctx, w, r, policy)",
		"w = WrapResponseCompression(w, r, config.Compression, compressionMode, streaming)",
		"config.PostAuthMiddlewares...)",
		"config.Middlewares...)",
		"getAutoV1AccessPolicy := AccessPolicy{}",
		"buildHandlerFunc(config, verifyAuth, getAutoV1AccessPolicy, postAuthHandlerGetAutoV1, compressionModeAuto, false)",
		"buildHandlerFunc(config, verifyAuth, getAlwaysV1AccessPolicy, postAuthHandlerGetAlwaysV1, compressionModeAlways, false)",
		"buildHandlerFunc(config, verifyAuth, getNeverV1AccessPolicy, postAuthHandlerGetNeverV1, compressionModeNever, false)",
		"buildHandlerFunc(config, verifyAuth, getStreamAlwaysV1AccessPolicy, postAuthHandlerGetStreamAlwaysV1, compressionModeAlways, true)",
	}
	for _, check := range checks {
		if !strings.Contains(mux, check) {
			t.Fatalf("expected generated mux to contain %q, got:\n%s", check, mux)
		}
	}

	if !strings.Contains(muxUtilSource, "type CompressionOptions struct") {
		t.Fatalf("expected mux util runtime to include compression options, got:\n%s", muxUtilSource)
	}
	if !strings.Contains(muxUtilSource, "func WrapResponseCompression(") {
		t.Fatalf("expected mux util runtime to wrap responses for compression, got:\n%s", muxUtilSource)
	}
	if !strings.Contains(muxUtilSource, "return mode == compressionModeAlways") {
		t.Fatalf("expected streaming compression to require explicit ALWAYS mode, got:\n%s", muxUtilSource)
	}
	if !strings.Contains(muxUtilSource, "AbortResponseCompression(s.w)") {
		t.Fatalf("expected stream aborts to mark compression as aborted, got:\n%s", muxUtilSource)
	}

	utilSource := strings.ReplaceAll(muxUtilSource, "__PACKAGE__", "example")
	if _, err := parser.ParseFile(token.NewFileSet(), "mux_util.gen.go", utilSource, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux util source to parse: %v\n%s", err, utilSource)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoMuxFileDoesNotTypeAssertDefaultAuthContext(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "Reply",
			FullName: "example.Reply",
			Fields:   []ir.Field{{Name: "value", Kind: ir.KindString}},
		}},
		Services: []ir.Service{{
			Name: "ExampleService",
			Methods: []ir.Method{{
				Name:           "GetReplyV1",
				InputFullName:  "cp.Empty",
				OutputFullName: "example.Reply",
			}},
		}},
	}

	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, file.GoPackage, "AuthContext")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}
	if strings.Contains(mux, "if v, ok := ctx.(AuthContext)") {
		t.Fatalf("expected default VerifyAuth stub to avoid type assertions, got:\n%s", mux)
	}
	if !strings.Contains(mux, "var authCtx AuthContext") {
		t.Fatalf("expected default VerifyAuth stub to return zero auth context, got:\n%s", mux)
	}
}
