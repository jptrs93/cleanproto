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
					Audit:          ir.AuditModeFull,
				},
				{
					Name:           "PostPlainV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          ir.AuditModeFull,
				},
				{
					Name:           "PostOperationV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          ir.AuditModeOperation,
				},
				{
					Name:           "PostRequestOnlyV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          ir.AuditModeRequest,
				},
				{
					Name:           "PostResponseOnlyV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          ir.AuditModeResponse,
				},
				{
					Name:           "PostUnauditedV1",
					InputFullName:  "example.PlainReq",
					OutputFullName: "example.PlainResp",
					Audit:          ir.AuditModeNone,
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
	if !strings.Contains(mux, "audit(authCtx, \"PostOperationV1\", err, nil, nil)") {
		t.Fatalf("expected operation-only audit to carry no payloads, got:\n%s", mux)
	}
	if !strings.Contains(mux, "audit(authCtx, \"PostRequestOnlyV1\", err, req, nil)") {
		t.Fatalf("expected request-only audit to carry the request alone, got:\n%s", mux)
	}
	if !strings.Contains(mux, "audit(authCtx, \"PostResponseOnlyV1\", err, nil, res)") {
		t.Fatalf("expected response-only audit to carry the response alone, got:\n%s", mux)
	}
	if strings.Contains(mux, "\"PostUnauditedV1\"") {
		t.Fatalf("expected AUDIT_MODE_NONE to emit no audit call, got:\n%s", mux)
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

	data, err := buildGoFileData(file, msgIndex, nil, file.GoPackage, "", false, nil, nil)
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
	if !strings.Contains(encode, "b = AppendBytes(b, m.ValueChild.Encode())") {
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

// Every element of a repeated field must reach the wire, zero-valued or not:
// a skipped element shifts all later ones against any parallel column.
func TestBuildGoFileDataRepeatedElementsKeepPositions(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{
				Name:     "Item",
				FullName: "example.Item",
				Fields:   []ir.Field{{Name: "id", Number: 1, Kind: ir.KindString, GoEncode: true}},
			},
			{
				Name:     "Columns",
				FullName: "example.Columns",
				Fields: []ir.Field{
					{Name: "names", Number: 1, Kind: ir.KindString, IsRepeated: true, GoEncode: true},
					{Name: "codes", Number: 2, Kind: ir.KindInt32, IsRepeated: true, GoEncode: true},
					{Name: "items", Number: 3, Kind: ir.KindMessage, MessageFullName: "example.Item", IsRepeated: true, GoEncode: true},
					{Name: "times", Number: 4, Kind: ir.KindMessage, IsTimestamp: true, IsRepeated: true, GoEncode: true},
					{Name: "waits", Number: 5, Kind: ir.KindMessage, IsDuration: true, IsRepeated: true, GoEncode: true},
				},
			},
		},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	data, err := buildGoFileData(file, msgIndex, nil, file.GoPackage, "", false, nil, nil)
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}

	var columns goMessage
	for _, msg := range data.Messages {
		if msg.Name == "Columns" {
			columns = msg
		}
	}
	encode := strings.Join(columns.EncodeLines, "\n")
	for _, want := range []string{
		"b = AppendRepeated(b, m.Names, AppendFieldDecorator(AppendStringElem, 1))",
		"b = AppendRepeated(b, m.Codes, AppendFieldDecorator(AppendInt32Elem, 2))",
		"b = AppendBytes(b, EncodeTimestamp(item))",
		"b = AppendBytes(b, EncodeDuration(item))",
	} {
		if !strings.Contains(encode, want) {
			t.Fatalf("expected encode to contain %q, got:\n%s", want, encode)
		}
	}
	if strings.Contains(encode, "if item.IsZero() {") || strings.Contains(encode, "if item == 0 {") {
		t.Fatalf("expected repeated timestamp/duration elements to be emitted unconditionally, got:\n%s", encode)
	}
	if !strings.Contains(encode, "b = AppendTag(b, 3, BytesType)\nif item == nil {\nb = AppendBytes(b, nil)\ncontinue\n}") {
		t.Fatalf("expected nil repeated message element to write an empty message in place, got:\n%s", encode)
	}
}

func TestBuildGoFileDataPackageLocalCustomGoType(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "Custom",
			FullName: "example.Custom",
			Fields: []ir.Field{
				{Name: "status", Number: 1, Kind: ir.KindInt32, GoType: "StatusCode", GoEncode: true},
				{Name: "status_opt", Number: 2, Kind: ir.KindInt32, GoType: "StatusCode", GoEncode: true, IsOptional: true},
				{Name: "statuses", Number: 3, Kind: ir.KindInt32, GoType: "StatusCode", GoEncode: true, IsRepeated: true, IsPacked: true},
			},
		}},
	}

	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}

	data, err := buildGoFileData(file, msgIndex, nil, file.GoPackage, "", false, nil, nil)
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}
	if len(data.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(data.Messages))
	}
	msg := data.Messages[0]
	if len(msg.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %#v", msg.Fields)
	}
	if msg.Fields[0].Type != "StatusCode" {
		t.Fatalf("expected singular custom field type, got %q", msg.Fields[0].Type)
	}
	if msg.Fields[1].Type != "*StatusCode" {
		t.Fatalf("expected optional custom field type, got %q", msg.Fields[1].Type)
	}
	if msg.Fields[2].Type != "[]StatusCode" {
		t.Fatalf("expected repeated custom field type, got %q", msg.Fields[2].Type)
	}

	encode := strings.Join(msg.EncodeLines, "\n")
	encodeChecks := []string{
		"b = AppendInt32Field(b, int32(m.Status), 1)",
		"if m.StatusOpt != nil {",
		"b = AppendInt32Elem(b, int32(*m.StatusOpt), 2)",
		"packed = AppendInt32Compact(packed, int32(item))",
	}
	for _, check := range encodeChecks {
		if !strings.Contains(encode, check) {
			t.Fatalf("expected custom Go type encode to contain %q, got:\n%s", check, encode)
		}
	}

	var decode strings.Builder
	for _, c := range msg.DecodeCases {
		decode.WriteString(strings.Join(c.Lines, "\n"))
		decode.WriteString("\n")
	}
	decodeChecks := []string{
		"var raw int32",
		"m.Status = StatusCode(raw)",
		"tmp := StatusCode(raw)",
		"m.StatusOpt = &tmp",
		"m.Statuses = append(m.Statuses, StatusCode(raw))",
	}
	for _, check := range decodeChecks {
		if !strings.Contains(decode.String(), check) {
			t.Fatalf("expected custom Go type decode to contain %q, got:\n%s", check, decode.String())
		}
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

	for _, goJSON := range []bool{false, true} {
		utilSource := string(buildGoMuxUtilSource("example", goJSON))
		if _, err := parser.ParseFile(token.NewFileSet(), "mux_util.gen.go", utilSource, parser.AllErrors); err != nil {
			t.Fatalf("expected generated mux util source (goJSON=%v) to parse: %v\n%s", goJSON, err, utilSource)
		}
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
	if !paths["gen/go/encode.gen.go"] {
		t.Fatalf("expected encode.gen.go in outputs, got %#v", paths)
	}
	if paths["gen/go/mux.gen.go"] {
		t.Fatalf("did not expect mux.gen.go when GoServer is false, got %#v", paths)
	}
}

func TestGoGeneratorClientServiceDropsOtherServiceTypes(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Enums: []ir.Enum{
			{Name: "SharedEnum", FullName: "example.SharedEnum", Values: []ir.EnumValue{{Name: "UNSET"}}},
			{Name: "OtherEnum", FullName: "example.OtherEnum", Values: []ir.EnumValue{{Name: "UNSET"}}},
			{Name: "StandaloneEnum", FullName: "example.StandaloneEnum", Values: []ir.EnumValue{{Name: "UNSET"}}},
		},
		Messages: []ir.Message{
			{
				Name: "KeptReq", FullName: "example.KeptReq",
				Fields: []ir.Field{
					{Name: "nested", Number: 1, Kind: ir.KindMessage, MessageFullName: "example.KeptNested"},
					{Name: "shared", Number: 2, Kind: ir.KindEnum, EnumFullName: "example.SharedEnum"},
				},
			},
			{Name: "KeptNested", FullName: "example.KeptNested", Fields: []ir.Field{{Name: "v", Number: 1, Kind: ir.KindString}}},
			{Name: "KeptResp", FullName: "example.KeptResp", Fields: []ir.Field{{Name: "v", Number: 1, Kind: ir.KindString}}},
			{
				Name: "SharedMsg", FullName: "example.SharedMsg",
				Fields: []ir.Field{{Name: "v", Number: 1, Kind: ir.KindString}},
			},
			{
				Name: "OtherReq", FullName: "example.OtherReq",
				Fields: []ir.Field{
					{Name: "other", Number: 1, Kind: ir.KindEnum, EnumFullName: "example.OtherEnum"},
					{Name: "shared", Number: 2, Kind: ir.KindMessage, MessageFullName: "example.SharedMsg"},
				},
			},
			{Name: "OtherResp", FullName: "example.OtherResp", Fields: []ir.Field{{Name: "v", Number: 1, Kind: ir.KindString}}},
			// Standalone: not referenced by any RPC, must be kept.
			{
				Name: "StandaloneMsg", FullName: "example.StandaloneMsg",
				Fields: []ir.Field{{Name: "e", Number: 1, Kind: ir.KindEnum, EnumFullName: "example.StandaloneEnum"}},
			},
		},
		Services: []ir.Service{
			{
				Name: "KeptService",
				Methods: []ir.Method{{
					Name: "GetKeptV1", InputFullName: "example.KeptReq", OutputFullName: "example.KeptResp",
				}, {
					Name: "GetSharedV1", InputFullName: "example.SharedMsg", OutputFullName: "example.KeptResp",
				}},
			},
			{
				Name: "OtherService",
				Methods: []ir.Method{{
					Name: "GetOtherV1", InputFullName: "example.OtherReq", OutputFullName: "example.OtherResp",
				}},
			},
		},
	}

	outputs, err := Generator{}.Generate([]ir.File{file}, generate.Options{
		GoOut: "gen/go", GoClient: true, GoServer: false, GoClientService: "KeptService",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var model string
	var encode string
	for _, output := range outputs {
		if output.Path == "gen/go/model.gen.go" {
			model = string(output.Content)
		}
		if output.Path == "gen/go/encode.gen.go" {
			encode = string(output.Content)
		}
	}
	if model == "" {
		t.Fatalf("missing model.gen.go in outputs")
	}
	if encode == "" {
		t.Fatalf("missing encode.gen.go in outputs")
	}

	mustHave := []string{
		"type KeptReq struct",       // client service request
		"type KeptNested struct",    // transitively reachable from KeptReq
		"type KeptResp struct",      // client service response
		"type SharedMsg struct",     // used by KeptService (and OtherService)
		"type SharedEnum int32",     // enum used by KeptReq
		"type StandaloneMsg struct", // not used by any RPC -> kept
		"type StandaloneEnum int32", // reachable only from StandaloneMsg
	}
	for _, want := range mustHave {
		if !strings.Contains(model, want) {
			t.Errorf("expected model.gen.go to contain %q\n%s", want, model)
		}
	}

	mustDrop := []string{
		"type OtherReq struct",
		"type OtherResp struct",
		"type OtherEnum int32",
	}
	for _, notWant := range mustDrop {
		if strings.Contains(model, notWant) {
			t.Errorf("expected model.gen.go NOT to contain %q\n%s", notWant, model)
		}
		if strings.Contains(encode, notWant) {
			t.Errorf("expected encode.gen.go NOT to contain %q\n%s", notWant, encode)
		}
	}
	if strings.Contains(model, "func (m *KeptReq) Encode() []byte") {
		t.Errorf("expected model.gen.go to exclude encode methods\n%s", model)
	}
	if !strings.Contains(encode, "func (m *KeptReq) Encode() []byte") {
		t.Errorf("expected encode.gen.go to contain encode methods\n%s", encode)
	}
}

func TestGoGeneratorMovesIsZeroToEncodeFile(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{
				Name:     "Child",
				FullName: "example.Child",
				Fields: []ir.Field{{Name: "label", Number: 1, Kind: ir.KindString, GoEncode: true}},
			},
			{
				Name:     "Parent",
				FullName: "example.Parent",
				Fields: []ir.Field{{Name: "value_child", Number: 1, Kind: ir.KindMessage, MessageFullName: "example.Child", GoEncode: true, GoValue: true}},
			},
		},
	}

	outputs, err := Generator{}.Generate([]ir.File{file}, generate.Options{GoOut: "gen/go"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var model string
	var encode string
	for _, output := range outputs {
		if output.Path == "gen/go/model.gen.go" {
			model = string(output.Content)
		}
		if output.Path == "gen/go/encode.gen.go" {
			encode = string(output.Content)
		}
	}
	if model == "" || encode == "" {
		t.Fatalf("expected both model.gen.go and encode.gen.go, got %#v", outputs)
	}
	if strings.Contains(model, "func (m Child) IsZero() bool") {
		t.Fatalf("expected model.gen.go to exclude IsZero methods\n%s", model)
	}
	if !strings.Contains(encode, "func (m Child) IsZero() bool") {
		t.Fatalf("expected encode.gen.go to contain IsZero methods\n%s", encode)
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

func TestBuildGoMuxUtilSourceJSONMode(t *testing.T) {
	jsonOnly := []string{
		`"encoding/json"`,
		"func negotiatedJSONResponse(r *http.Request) bool {",
		"func negotiatedJSONRequest(r *http.Request) bool {",
		"func hasJSONMediaType(header string) bool {",
		"func decodeRequestPayload[T any](",
		`w.Header().Set("Content-Type", "application/json")`,
		"handleReqErr(ctx, err, path, negotiatedJSONResponse(r), w)",
	}

	withJSON := string(buildGoMuxUtilSource("example", true))
	for _, want := range jsonOnly {
		if !strings.Contains(withJSON, want) {
			t.Errorf("expected -go.json mux util to contain %q", want)
		}
	}

	// Without the flag the file must be exactly what it always was: no JSON
	// helpers, and critically no encoding/json import pulled in for nothing.
	withoutJSON := string(buildGoMuxUtilSource("example", false))
	for _, unwanted := range jsonOnly {
		if strings.Contains(withoutJSON, unwanted) {
			t.Errorf("expected default mux util to omit %q", unwanted)
		}
	}
	if strings.Contains(withoutJSON, "cp:json") {
		t.Error("marker comments leaked into generated output")
	}
	if strings.Contains(withJSON, "cp:json") {
		t.Error("marker comments leaked into generated output")
	}
	// Streaming stays protobuf-framed in both modes.
	if !strings.Contains(withJSON, `h.Set("Content-Type", "application/protobuf-stream")`) {
		t.Error("expected streaming responses to stay protobuf in JSON mode")
	}
}

func TestBuildGoMessageJSONTagsExcludeNonEncodedFields(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{{
			Name:     "ApiErr",
			FullName: "example.ApiErr",
			Fields: []ir.Field{
				{Name: "display_err", Number: 1, Kind: ir.KindString, GoEncode: true},
				{Name: "internal_err", Number: 2, Kind: ir.KindString, GoEncode: false},
			},
		}},
	}
	msgIndex := map[string]ir.Message{"example.ApiErr": file.Messages[0]}

	// go_encode=false stays off the wire in JSON mode, matching protobuf.
	data, err := buildGoFileData(file, msgIndex, nil, file.GoPackage, "snake", true, nil, nil)
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}
	tags := map[string]string{}
	for _, f := range data.Messages[0].Fields {
		tags[f.Name] = f.JSONTag
	}
	if tags["DisplayErr"] != "display_err,omitempty" {
		t.Errorf("DisplayErr tag = %q, want display_err,omitempty", tags["DisplayErr"])
	}
	if tags["InternalErr"] != "-" {
		t.Errorf("InternalErr tag = %q, want - (go_encode=false must not reach JSON clients)", tags["InternalErr"])
	}

	// Without -go.json the tags are only for the caller's own marshalling, so
	// go_encode=false is left alone and the field keeps its name.
	data, err = buildGoFileData(file, msgIndex, nil, file.GoPackage, "snake", false, nil, nil)
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}
	for _, f := range data.Messages[0].Fields {
		if f.Name == "InternalErr" && f.JSONTag != "internal_err,omitempty" {
			t.Errorf("InternalErr tag = %q, want internal_err,omitempty without -go.json", f.JSONTag)
		}
	}
}

// multipartTestFile is a two-part response (Book then Library) on an RPC that
// satisfies the multipart contract, for tests that vary one thing about it.
func multipartTestFile() (ir.File, map[string]ir.Message) {
	file := ir.File{
		GoPackage: "example",
		Messages: []ir.Message{
			{Name: "Book", FullName: "example.Book", Fields: []ir.Field{{Name: "id", ProtoName: "id", Number: 1, Kind: ir.KindString}}},
			{Name: "Library", FullName: "example.Library", Fields: []ir.Field{{Name: "name", ProtoName: "name", Number: 1, Kind: ir.KindString}}},
			{Name: "GetBookReq", FullName: "example.GetBookReq", Fields: []ir.Field{{Name: "id", ProtoName: "id", Number: 1, Kind: ir.KindString}}},
			{Name: "BookDetailRes", FullName: "example.BookDetailRes", Fields: []ir.Field{
				{Name: "book", ProtoName: "book", Number: 1, Kind: ir.KindMessage, MessageFullName: "example.Book"},
				{Name: "library", ProtoName: "library", Number: 2, Kind: ir.KindMessage, MessageFullName: "example.Library"},
			}},
		},
		Services: []ir.Service{{
			Name: "LibraryService",
			Methods: []ir.Method{
				{
					Name:              "GetLibraryBook_DetailV1",
					InputFullName:     "example.GetBookReq",
					OutputFullName:    "example.BookDetailRes",
					MultipartResponse: true,
					CompressionMode:   compressionNever,
				},
			},
		}},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}
	return file, msgIndex
}

func TestBuildGoMuxFileEmitsMultipartHandler(t *testing.T) {
	file, msgIndex := multipartTestFile()

	mux, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoMuxFile: %v", err)
	}

	checks := []string{
		"GetLibraryBookDetailV1(context.Context, *GetBookReq) (func() (*Book, error), func() (*Library, error))",
		"partFnBook, partFnLibrary := h.GetLibraryBookDetailV1(authCtx, req)",
		// Part 1 runs before anything is committed, so it still gets a status code.
		"partBook, err := partFnBook()",
		"HandleReqErr(authCtx, err, r, w)",
		"parts := NewPartsWriter(w)",
		// Every later part can only abort.
		"partLibrary, err := partFnLibrary()",
		"parts.Abort(authCtx, err)",
		"m.HandleFunc(\"GET /v1/library/book-detail\"",
	}
	for _, check := range checks {
		if !strings.Contains(mux, check) {
			t.Fatalf("expected generated mux to contain %q, got:\n%s", check, mux)
		}
	}
	if strings.Contains(mux, "Respond(authCtx, r, w") && strings.Contains(mux, "DecodeBookDetailRes") {
		t.Fatalf("multipart response must not be sent as a single encoded message, got:\n%s", mux)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mux.gen.go", mux, parser.AllErrors); err != nil {
		t.Fatalf("expected generated mux source to parse: %v\n%s", err, mux)
	}
}

func TestBuildGoClientFileEmitsMultipartMethod(t *testing.T) {
	file, msgIndex := multipartTestFile()

	client, err := buildGoClientFile(file, msgIndex, file.GoPackage, "")
	if err != nil {
		t.Fatalf("buildGoClientFile: %v", err)
	}

	checks := []string{
		"func (c *LibraryCapi) GetLibraryBookDetailV1(ctx context.Context, req *GetBookReq) (func() (*Book, error), func() (*Library, error), func())",
		"\"application/protobuf-parts\"",
		"reader = NewStreamReader(resp.Body, 0)",
		// A body that ends early is a server-side abort, not an absent part.
		"multipart response ended before part %v",
		"return DecodeBook(payload)",
		"return DecodeLibrary(payload)",
	}
	for _, check := range checks {
		if !strings.Contains(client, check) {
			t.Fatalf("expected generated client to contain %q, got:\n%s", check, client)
		}
	}
	if strings.Contains(client, "DecodeBookDetailRes") {
		t.Fatalf("multipart client must decode parts, not the wrapper message, got:\n%s", client)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "client.gen.go", client, parser.AllErrors); err != nil {
		t.Fatalf("expected generated client source to parse: %v\n%s", err, client)
	}
}

func TestBuildGoMuxFileRejectsMultipartMisuse(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ir.File, map[string]ir.Message)
		wantSub string
	}{
		{
			name: "ScalarPart",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields[1] = ir.Field{Name: "count", ProtoName: "count", Number: 2, Kind: ir.KindInt32}
				idx["example.BookDetailRes"] = out
			},
			wantSub: "must be a message",
		},
		{
			name: "RepeatedPart",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields[1].IsRepeated = true
				idx["example.BookDetailRes"] = out
			},
			wantSub: "cannot be repeated",
		},
		{
			name: "SinglePart",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields = out.Fields[:1]
				idx["example.BookDetailRes"] = out
			},
			wantSub: "at least two fields",
		},
		{
			name: "NonContiguousFieldNumbers",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields[1].Number = 7
				idx["example.BookDetailRes"] = out
			},
			wantSub: "contiguous from 1",
		},
		{
			name: "DuplicatePartType",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields[1].MessageFullName = "example.Book"
				idx["example.BookDetailRes"] = out
			},
			wantSub: "repeats part type",
		},
		{
			name: "GoIgnorePart",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				out := idx["example.BookDetailRes"]
				out.Fields[1].GoIgnore = true
				idx["example.BookDetailRes"] = out
			},
			wantSub: "cannot use cp.go_ignore",
		},
		{
			name: "Streaming",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				f.Services[0].Methods[0].IsStreamingServer = true
			},
			wantSub: "cannot also be streaming",
		},
		{
			name: "GoCustom",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				f.Services[0].Methods[0].GoCustom = true
			},
			wantSub: "cannot also use cp.go_custom",
		},
		{
			name: "Audit",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				f.Services[0].Methods[0].Audit = ir.AuditModeOperation
			},
			wantSub: "cannot also use cp.audit",
		},
		{
			name: "CompressionNotNever",
			mutate: func(f *ir.File, idx map[string]ir.Message) {
				f.Services[0].Methods[0].CompressionMode = 0
			},
			wantSub: "COMPRESSION_MODE_NEVER",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, msgIndex := multipartTestFile()
			// Fields are shared with the slice in file.Messages; copy so a
			// mutation cannot leak between the mux and client checks.
			out := msgIndex["example.BookDetailRes"]
			out.Fields = append([]ir.Field(nil), out.Fields...)
			msgIndex["example.BookDetailRes"] = out
			tc.mutate(&file, msgIndex)

			if _, err := buildGoMuxFile(file, msgIndex, nil, file.GoPackage, ""); err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected mux error containing %q, got %v", tc.wantSub, err)
			}
			if _, err := buildGoClientFile(file, msgIndex, file.GoPackage, ""); err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected client error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestBuildGoMuxUtilSourceEmitsPartsRuntime(t *testing.T) {
	src := string(buildGoMuxUtilSource("apigen", false))
	checks := []string{
		"type PartsWriter struct",
		"func NewPartsWriter(w http.ResponseWriter) *PartsWriter",
		"func (p *PartsWriter) Write(payload []byte) error",
		// Aborting drops the connection so the short body is detectable.
		"func (p *PartsWriter) Abort(ctx context.Context, err error)",
		"panic(http.ErrAbortHandler)",
		"h.Set(\"Content-Type\", \"application/protobuf-parts\")",
	}
	for _, check := range checks {
		if !strings.Contains(src, check) {
			t.Fatalf("expected mux util source to contain %q", check)
		}
	}
}

func TestLoadUtilSourceSortsMapKeys(t *testing.T) {
	src, err := loadUtilSource("example")
	if err != nil {
		t.Fatalf("loadUtilSource: %v", err)
	}
	source := string(src)
	if !strings.Contains(source, "sort.Slice(keys, func(i, j int) bool { return lessMapKey(keys[i], keys[j]) })") {
		t.Fatalf("expected AppendMap to sort map keys, got:\n%s", source)
	}
	if !strings.Contains(source, "\t\"sort\"\n") {
		t.Fatalf("expected util source to import sort")
	}
	if !strings.Contains(source, "func lessMapKey[K comparable](a, b K) bool {") {
		t.Fatalf("expected util source to define lessMapKey")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "util.gen.go", source, parser.AllErrors); err != nil {
		t.Fatalf("expected generated util source to parse: %v\n%s", err, source)
	}
}

// proto3 optional means explicit presence: the non-nil pointer is the
// presence bit, so an explicit 0, "" or false must reach the decoder as a set
// field, exactly as the TS writer already encodes optional fields. The Opt
// append helpers therefore guard on nil alone and never skip zero values.
func TestLoadUtilSourceOptHelpersKeepExplicitZero(t *testing.T) {
	src, err := loadUtilSource("example")
	if err != nil {
		t.Fatalf("loadUtilSource: %v", err)
	}
	source := string(src)
	for _, stale := range []string{
		"if v == nil || *v == 0",
		"if v == nil || *v == \"\"",
		"if v == nil || !*v",
	} {
		if strings.Contains(source, stale) {
			t.Fatalf("Opt append helper skips explicit zero values: %q", stale)
		}
	}
	if !strings.Contains(source, "func AppendInt64FieldOpt") {
		t.Fatalf("expected util source to define AppendInt64FieldOpt")
	}
}

// Optional enum, bytes, timestamp and duration fields bypass the Opt append
// helpers, so their encode lines must pair a plain nil guard with the
// unconditional Elem appenders - no IsZero or != 0 checks that would erase
// presence for explicitly set zero values.
func TestBuildGoFileDataOptionalFieldsKeepExplicitZero(t *testing.T) {
	file := ir.File{
		GoPackage: "example",
		Enums: []ir.Enum{{
			Name:     "Status",
			FullName: "example.Status",
			Values:   []ir.EnumValue{{Name: "STATUS_UNKNOWN", Number: 0}},
		}},
		Messages: []ir.Message{{
			Name:     "Row",
			FullName: "example.Row",
			Fields: []ir.Field{
				{Name: "count", Number: 1, Kind: ir.KindInt64, GoEncode: true, IsOptional: true},
				{Name: "status", Number: 2, Kind: ir.KindEnum, EnumFullName: "example.Status", GoEncode: true, IsOptional: true},
				{Name: "blob", Number: 3, Kind: ir.KindBytes, GoEncode: true, IsOptional: true},
				{Name: "seen_at", Number: 4, Kind: ir.KindMessage, IsTimestamp: true, GoEncode: true, IsOptional: true},
				{Name: "wait", Number: 5, Kind: ir.KindMessage, IsDuration: true, GoEncode: true, IsOptional: true},
			},
		}},
	}
	msgIndex := map[string]ir.Message{}
	for _, msg := range file.Messages {
		msgIndex[msg.FullName] = msg
	}
	enumIndex := map[string]ir.Enum{}
	for _, enum := range file.Enums {
		enumIndex[enum.FullName] = enum
	}

	data, err := buildGoFileData(file, msgIndex, enumIndex, file.GoPackage, "", false, nil, nil)
	if err != nil {
		t.Fatalf("buildGoFileData: %v", err)
	}
	encode := strings.Join(data.Messages[0].EncodeLines, "\n")
	encodeChecks := []string{
		"b = AppendInt64FieldOpt(b, m.Count, 1)",
		"b = AppendInt32Elem(b, int32(*m.Status), 2)",
		"b = AppendBytesElem(b, *m.Blob, 3)",
		"b = AppendBytesElem(b, EncodeTimestamp(*m.SeenAt), 4)",
		"b = AppendBytesElem(b, EncodeDuration(*m.Wait), 5)",
	}
	for _, check := range encodeChecks {
		if !strings.Contains(encode, check) {
			t.Fatalf("expected optional encode to contain %q, got:\n%s", check, encode)
		}
	}
	for _, stale := range []string{".IsZero()", "!= 0 {"} {
		if strings.Contains(encode, stale) {
			t.Fatalf("optional encode must guard on nil alone, found %q in:\n%s", stale, encode)
		}
	}
}
