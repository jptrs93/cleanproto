package gogen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/jptrs93/cleanproto/internal/generate"
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

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
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

func TestBuildGoFileDataGoValueMessageField(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{
				Name:     "Child",
				FullName: "example.Child",
				Fields: []ir.Field{
					{Name: "count", Number: 1, Kind: ir.KindInt32, GoEncode: true},
					{Name: "label", Number: 2, Kind: ir.KindString, GoEncode: true},
				},
			},
			{
				Name:     "Parent",
				FullName: "example.Parent",
				Fields: []ir.Field{
					{Name: "value_child", Number: 1, Kind: ir.KindMessage, MessageFullName: "example.Child", GoEncode: true, GoValue: true},
					{Name: "pointer_child", Number: 2, Kind: ir.KindMessage, MessageFullName: "example.Child", GoEncode: true},
				},
			},
		},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	data, err := buildGoFileData(file, msgIndex, nil, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}

	var parent, child goMessage
	for _, msg := range data.Messages {
		if msg.Name == "Parent" {
			parent = msg
		}
		if msg.Name == "Child" {
			child = msg
		}
	}
	if len(parent.Fields) != 2 {
		t.Fatalf("expected parent fields, got %#v", parent.Fields)
	}
	if parent.Fields[0].Type != "Child" {
		t.Fatalf("expected go_value message field to be Child, got %q", parent.Fields[0].Type)
	}
	if parent.Fields[1].Type != "*Child" {
		t.Fatalf("expected default message field to stay *Child, got %q", parent.Fields[1].Type)
	}
	if !child.HasIsZero || !strings.Contains(child.IsZeroExpr, "m.Count == 0") || !strings.Contains(child.IsZeroExpr, "m.Label == \"\"") {
		t.Fatalf("expected Child IsZero expression for value-message encoding, got has=%v expr=%q", child.HasIsZero, child.IsZeroExpr)
	}
	encode := strings.Join(parent.EncodeLines, "\n")
	if !strings.Contains(encode, "if !m.ValueChild.IsZero() {") {
		t.Fatalf("expected value message encode to skip zero nested message, got:\n%s", encode)
	}
	if !strings.Contains(encode, "b = protowire.AppendBytes(b, m.ValueChild.Encode())") {
		t.Fatalf("expected value message encode to include non-zero nested message, got:\n%s", encode)
	}
	if !strings.Contains(encode, "if m.PointerChild != nil {") {
		t.Fatalf("expected default message encode to keep pointer nil guard, got:\n%s", encode)
	}

	var decode strings.Builder
	for _, c := range parent.DecodeCases {
		decode.WriteString(strings.Join(c.Lines, "\n"))
	}
	if !strings.Contains(decode.String(), "m.ValueChild = *item") {
		t.Fatalf("expected value message decode to assign decoded value, got:\n%s", decode.String())
	}
	if !strings.Contains(decode.String(), "m.PointerChild = item") {
		t.Fatalf("expected default message decode to keep pointer assignment, got:\n%s", decode.String())
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

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
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
		"UnaryCompression    func(http.Handler) http.HandlerFunc",
		"StreamCompression   func(http.Handler) http.HandlerFunc",
		"verifyAuth := config.VerifyAuth",
		"func ApplyPostAuthMiddlewares(h PostAuthHandlerFunc, middlewares ...PostAuthMiddlewareFunc) PostAuthHandlerFunc",
		"func buildHandlerFunc(config *MuxConfig, verifyAuth VerifyAuthFunc, policy AccessPolicy, postAuthHandler PostAuthHandlerFunc, compressionMode int32, streaming bool) http.HandlerFunc",
		"authCtx, err := verifyAuth(ctx, w, r, policy)",
		"if compressionMode == compressionModeNever",
		"compress := config.UnaryCompression",
		"compress = config.StreamCompression",
		"return compress(routeHandler)",
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
	if strings.Contains(mux, "ExampleServiceHandler") {
		t.Fatalf("expected single-service mux to keep ServerHandler name, got:\n%s", mux)
	}
	if strings.Contains(mux, "CreateExampleServiceMux") {
		t.Fatalf("expected single-service mux to keep CreateMux name, got:\n%s", mux)
	}

	utilSource := strings.ReplaceAll(muxUtilSource, "__PACKAGE__", "example")
	if _, err := parser.ParseFile(token.NewFileSet(), "mux_util.gen.go", utilSource, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux util source to parse: %v\n%s", err, utilSource)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoMuxFileEmitsClientStreamingHandler(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "Book", FullName: "example.Book", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
			{Name: "Library", FullName: "example.Library", Fields: []ir.Field{{Name: "name", Kind: ir.KindString}}},
		},
		Services: []ir.Service{{
			Name: "LibraryService",
			Methods: []ir.Method{
				{
					Name:              "PostLibraryBook_BulkV1",
					InputFullName:     "example.Book",
					OutputFullName:    "example.Library",
					IsStreamingClient: true,
				},
			},
		}},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, map[string]bool{"example.Book": true}, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}

	checks := []string{
		"PostLibraryBookBulkV1(context.Context, iter.Seq2[*Book, error]) (*Library, error)",
		"sr := NewStreamReader(r.Body, config.MaxRequestBodySize)",
		"seq := func(yield func(*Book, error) bool) {",
		"req, err := DecodeBook(payload)",
		"if err := req.Validate(); err != nil {",
		"res, err := h.PostLibraryBookBulkV1(authCtx, seq)",
		"Respond(authCtx, r, w, res, err)",
		"m.HandleFunc(\"POST /v1/library/book-bulk\"",
	}
	for _, check := range checks {
		if !strings.Contains(mux, check) {
			t.Fatalf("expected generated mux to contain %q, got:\n%s", check, mux)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoMuxFileRejectsClientStreamingMisuse(t *testing.T) {
	base := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "Book", FullName: "example.Book"},
			{Name: "Library", FullName: "example.Library"},
		},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range base.Messages {
		msgIndex[msg.FullName] = msg
	}

	cases := []struct {
		name    string
		method  ir.Method
		wantSub string
	}{
		{
			name: "EmptyInput",
			method: ir.Method{
				Name:              "PostThingV1",
				InputFullName:     "cp.Empty",
				OutputFullName:    "example.Library",
				IsStreamingClient: true,
			},
			wantSub: "cannot have Empty input",
		},
		{
			name: "GetVerb",
			method: ir.Method{
				Name:              "GetThingV1",
				InputFullName:     "example.Book",
				OutputFullName:    "example.Library",
				IsStreamingClient: true,
			},
			wantSub: "cannot use a Get* verb",
		},
		{
			name: "GoCustom",
			method: ir.Method{
				Name:              "PostThingV1",
				InputFullName:     "example.Book",
				OutputFullName:    "example.Library",
				IsStreamingClient: true,
				GoCustom:          true,
			},
			wantSub: "cannot also use cp.go_custom",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := base
			f.Services = []ir.Service{{Name: "S", Methods: []ir.Method{tc.method}}}
			_, err := buildGoMuxFile(f, msgIndex, nil, f.GoPackage, "")
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestBuildGoMuxFileEmitsBidiStreamingHandler(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "GetBookReq", FullName: "example.GetBookReq", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
			{Name: "Book", FullName: "example.Book", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
		},
		Services: []ir.Service{{
			Name: "LibraryService",
			Methods: []ir.Method{
				{
					Name:              "PostLibraryBook_LookupV1",
					InputFullName:     "example.GetBookReq",
					OutputFullName:    "example.Book",
					IsStreamingClient: true,
					IsStreamingServer: true,
				},
			},
		}},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, map[string]bool{"example.GetBookReq": true}, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}

	checks := []string{
		"PostLibraryBookLookupV1(context.Context, iter.Seq2[*GetBookReq, error]) iter.Seq2[*Book, error]",
		"sr := NewStreamReader(r.Body, config.MaxRequestBodySize)",
		"reqSeq := func(yield func(*GetBookReq, error) bool) {",
		"req, err := DecodeGetBookReq(payload)",
		"if err := req.Validate(); err != nil {",
		"respSeq := h.PostLibraryBookLookupV1(authCtx, reqSeq)",
		"stream := NewStreamWriter(w)",
		"for resp, yieldErr := range respSeq {",
		"stream.Write(resp.Encode())",
		"stream.Finish(authCtx, streamErr)",
		"m.HandleFunc(\"POST /v1/library/book-lookup\"",
		", true))",
	}
	for _, check := range checks {
		if !strings.Contains(mux, check) {
			t.Fatalf("expected generated mux to contain %q, got:\n%s", check, mux)
		}
	}
	if strings.Contains(mux, "decodeWithMaxBodySize(r, config.MaxRequestBodySize, DecodeGetBookReq)") {
		t.Fatalf("bidi handler must not unary-decode the request body, got:\n%s", mux)
	}
	if strings.Contains(mux, "Respond(authCtx, r, w,") {
		t.Fatalf("bidi handler must not call Respond, got:\n%s", mux)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoMuxFileRejectsBidiMisuse(t *testing.T) {
	base := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "Book", FullName: "example.Book"},
		},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range base.Messages {
		msgIndex[msg.FullName] = msg
	}

	cases := []struct {
		name    string
		method  ir.Method
		wantSub string
	}{
		{
			name: "EmptyInput",
			method: ir.Method{
				Name:              "PostThingV1",
				InputFullName:     "cp.Empty",
				OutputFullName:    "example.Book",
				IsStreamingClient: true,
				IsStreamingServer: true,
			},
			wantSub: "cannot have Empty input",
		},
		{
			name: "EmptyOutput",
			method: ir.Method{
				Name:              "PostThingV1",
				InputFullName:     "example.Book",
				OutputFullName:    "cp.Empty",
				IsStreamingClient: true,
				IsStreamingServer: true,
			},
			wantSub: "cannot have Empty output",
		},
		{
			name: "GetVerb",
			method: ir.Method{
				Name:              "GetThingV1",
				InputFullName:     "example.Book",
				OutputFullName:    "example.Book",
				IsStreamingClient: true,
				IsStreamingServer: true,
			},
			wantSub: "cannot use a Get* verb",
		},
		{
			name: "GoCustom",
			method: ir.Method{
				Name:              "PostThingV1",
				InputFullName:     "example.Book",
				OutputFullName:    "example.Book",
				IsStreamingClient: true,
				IsStreamingServer: true,
				GoCustom:          true,
			},
			wantSub: "cannot also use cp.go_custom",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := base
			f.Services = []ir.Service{{Name: "S", Methods: []ir.Method{tc.method}}}
			_, err := buildGoMuxFile(f, msgIndex, nil, f.GoPackage, "")
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
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

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "AuthContext")
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

func TestBuildGoMuxFileSplitsMultipleServices(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "FooReply", FullName: "example.FooReply", Fields: []ir.Field{{Name: "value", Kind: ir.KindString}}},
			{Name: "BarReply", FullName: "example.BarReply", Fields: []ir.Field{{Name: "value", Kind: ir.KindString}}},
		},
		Services: []ir.Service{
			{
				Name: "FooService",
				Methods: []ir.Method{{
					Name:           "GetFooV1",
					InputFullName:  "cp.Empty",
					OutputFullName: "example.FooReply",
				}},
			},
			{
				Name: "BarService",
				Methods: []ir.Method{{
					Name:           "GetBarV1",
					InputFullName:  "cp.Empty",
					OutputFullName: "example.BarReply",
				}},
			},
		},
	}

	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}

	checks := []string{
		"type FooServiceHandler interface",
		"GetFooV1(context.Context) (*FooReply, error)",
		"func CreateFooServiceMux(h FooServiceHandler, config *MuxConfig) *http.ServeMux",
		"type BarServiceHandler interface",
		"GetBarV1(context.Context) (*BarReply, error)",
		"func CreateBarServiceMux(h BarServiceHandler, config *MuxConfig) *http.ServeMux",
	}
	for _, check := range checks {
		if !strings.Contains(mux, check) {
			t.Fatalf("expected generated mux to contain %q, got:\n%s", check, mux)
		}
	}
	if strings.Contains(mux, "type ServerHandler interface") {
		t.Fatalf("expected multi-service mux to avoid flattened ServerHandler, got:\n%s", mux)
	}
	if strings.Contains(mux, "func CreateMux(") {
		t.Fatalf("expected multi-service mux to avoid flattened CreateMux, got:\n%s", mux)
	}

	fooInterface := generatedSection(t, mux, "type FooServiceHandler interface", "}\n\nfunc CreateFooServiceMux")
	if strings.Contains(fooInterface, "GetBarV1") {
		t.Fatalf("expected FooServiceHandler to only contain foo methods, got:\n%s", fooInterface)
	}
	barInterface := generatedSection(t, mux, "type BarServiceHandler interface", "}\n\nfunc CreateBarServiceMux")
	if strings.Contains(barInterface, "GetFooV1") {
		t.Fatalf("expected BarServiceHandler to only contain bar methods, got:\n%s", barInterface)
	}
	fooMux := generatedSection(t, mux, "func CreateFooServiceMux", "\n}\n\ntype BarServiceHandler interface")
	if strings.Contains(fooMux, "GetBarV1") {
		t.Fatalf("expected CreateFooServiceMux to only register foo methods, got:\n%s", fooMux)
	}
	barMux := generatedSection(t, mux, "func CreateBarServiceMux", "\n}")
	if strings.Contains(barMux, "GetFooV1") {
		t.Fatalf("expected CreateBarServiceMux to only register bar methods, got:\n%s", barMux)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoMuxFileUsesURLOverride(t *testing.T) {
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
				URL:            "/v1/custom/reply",
			}},
		}},
	}

	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}
	if !strings.Contains(mux, "m.HandleFunc(\"GET /v1/custom/reply\"") {
		t.Fatalf("expected generated mux to use URL override, got:\n%s", mux)
	}
	if strings.Contains(mux, "GET /v1/reply") {
		t.Fatalf("expected generated mux to avoid derived path, got:\n%s", mux)
	}
}

func TestBuildGoMuxFileErrorsOnServiceNameCollision(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "Reply",
			FullName: "example.Reply",
			Fields:   []ir.Field{{Name: "value", Kind: ir.KindString}},
		}},
		Services: []ir.Service{
			{
				Name: "Foo_Bar",
				Methods: []ir.Method{{
					Name:           "GetFooV1",
					InputFullName:  "cp.Empty",
					OutputFullName: "example.Reply",
				}},
			},
			{
				Name: "FooBar",
				Methods: []ir.Method{{
					Name:           "GetBarV1",
					InputFullName:  "cp.Empty",
					OutputFullName: "example.Reply",
				}},
			},
		},
	}
	msgIndex := map[string]ir.Message{"example.Reply": file.Messages[0]}

	_, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
	if err == nil {
		t.Fatal("expected service name collision error")
	}
	if !strings.Contains(err.Error(), "duplicate generated service handler name: FooBarHandler") {
		t.Fatalf("expected duplicate handler error, got: %v", err)
	}
}

func TestBuildGoClientFileUsesCapiNameAndServiceRoutes(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "Book", FullName: "example.Book", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
			{Name: "GetBookReq", FullName: "example.GetBookReq", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
			{Name: "CheckoutBookReq", FullName: "example.CheckoutBookReq", Fields: []ir.Field{{Name: "id", Kind: ir.KindString}}},
		},
		Services: []ir.Service{{
			Name: "LibraryService",
			Methods: []ir.Method{
				{Name: "GetLibraryBookV1", InputFullName: "example.GetBookReq", OutputFullName: "example.Book", URL: "/v1/custom/book"},
				{Name: "PostLibraryBook_CheckoutV1", InputFullName: "example.CheckoutBookReq", OutputFullName: "cp.Empty"},
				{Name: "PostLibraryBook_BulkV1", InputFullName: "example.Book", OutputFullName: "example.Book", IsStreamingClient: true},
				{Name: "PostLibraryBook_LookupV1", InputFullName: "example.GetBookReq", OutputFullName: "example.Book", IsStreamingClient: true, IsStreamingServer: true},
			},
		}},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	client, err := buildGoClientFile(file, msgIndex, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoClientFile: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "client.gen.go", client, parser.AllErrors); err != nil {
		t.Fatalf("generated client should parse: %v\n%s", err, client)
	}
	checks := []string{
		"type LibraryCapi struct",
		"func NewLibraryCapi(baseURL string, opts ...LibraryCapiOption) *LibraryCapi",
		"func (c *LibraryCapi) GetLibraryBookV1(ctx context.Context, req *GetBookReq) (*Book, error)",
		"\"/v1/custom/book\"",
		"func (c *LibraryCapi) PostLibraryBookCheckoutV1(ctx context.Context, req *CheckoutBookReq) error",
		"func (c *LibraryCapi) PostLibraryBookBulkV1(ctx context.Context, reqs iter.Seq2[*Book, error]) (*Book, error)",
		"func (c *LibraryCapi) PostLibraryBookLookupV1(ctx context.Context, reqs iter.Seq2[*GetBookReq, error]) iter.Seq2[*Book, error]",
	}
	for _, check := range checks {
		if !strings.Contains(client, check) {
			t.Fatalf("expected generated client to contain %q, got:\n%s", check, client)
		}
	}
	if strings.Contains(client, "LibraryServiceCapi") {
		t.Fatalf("expected Service suffix to be trimmed from client name, got:\n%s", client)
	}
}

func TestGoGeneratorClientOnlySkipsMuxFile(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "Reply",
			FullName: "example.Reply",
			Fields:   []ir.Field{{Name: "value", Number: 1, Kind: ir.KindString, GoEncode: true}},
		}},
		Services: []ir.Service{{
			Name: "LibraryService",
			Methods: []ir.Method{{
				Name:           "GetReplyV1",
				InputFullName:  "cp.Empty",
				OutputFullName: "example.Reply",
			}},
		}},
	}

	outputs, err := Generator{}.Generate([]ir.File{file}, generate.Options{GoOut: "gen/go", GoClient: true, GoServer: false})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	paths := map[string]bool{}
	for _, output := range outputs {
		paths[output.Path] = true
	}
	if !paths["gen/go/client.gen.go"] {
		t.Fatalf("expected client.gen.go in outputs, got %#v", paths)
	}
	if paths["gen/go/mux.gen.go"] {
		t.Fatalf("did not expect mux.gen.go when GoServer is false, got %#v", paths)
	}
}

func generatedSection(t *testing.T, source string, start string, end string) string {
	t.Helper()
	startIdx := strings.Index(source, start)
	if startIdx == -1 {
		t.Fatalf("missing section start %q in:\n%s", start, source)
	}
	endIdx := strings.Index(source[startIdx:], end)
	if endIdx == -1 {
		t.Fatalf("missing section end %q after %q in:\n%s", end, start, source[startIdx:])
	}
	return source[startIdx : startIdx+endIdx+len(end)]
}
