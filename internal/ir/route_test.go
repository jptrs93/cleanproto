package ir

import (
	"strings"
	"testing"
)

func TestRouteDerivation(t *testing.T) {
	tests := []struct {
		method   Method
		pattern  string
		wildcard bool
	}{
		{method: Method{Name: "Get"}, pattern: "GET /", wildcard: true},
		{method: Method{Name: "GetRoot"}, pattern: "GET /", wildcard: true},
		{method: Method{Name: "Post"}, pattern: "POST /", wildcard: true},
		{method: Method{Name: "GetLibraryV1"}, pattern: "GET /v1/library"},
		{method: Method{Name: "GetV1Healthz"}, pattern: "GET /v1/healthz"},
		{method: Method{Name: "PostLibraryBook_CheckoutV1"}, pattern: "POST /v1/library/book-checkout"},
		{method: Method{Name: "DeleteThing"}, pattern: "DELETE /thing"},
		{method: Method{Name: "GetBooksV1", URL: "/v1/custom"}, pattern: "GET /v1/custom"},
		{method: Method{Name: "GetThingV1", URL: "/v1/thing/{id}"}, pattern: "GET /v1/thing/{id}", wildcard: true},
		{method: Method{Name: "GetThingV1", URL: "/v1/thing/{$}"}, pattern: "GET /v1/thing/{$}"},
		{method: Method{Name: "GetThingV1", URL: "/v1/thing/{id}/{$}"}, pattern: "GET /v1/thing/{id}/{$}", wildcard: true},
		{method: Method{Name: "GetThingV1", URL: "/v1/thing/"}, pattern: "GET /v1/thing/", wildcard: true},
		{method: Method{Name: "Rill", HTTPMethod: HTTPMethodAny, URL: "/rill/{path...}"}, pattern: "/rill/{path...}", wildcard: true},
		{method: Method{Name: "Rill", HTTPMethod: HTTPMethodAny}, pattern: "/rill"},
		{method: Method{Name: "AssetsV1", HTTPMethod: HTTPMethodGet}, pattern: "GET /v1/assets"},
		{method: Method{Name: "PostThing", HTTPMethod: HTTPMethodPut}, pattern: "PUT /thing"},
		{method: Method{Name: "GetThing", HTTPMethod: HTTPMethodAny}, pattern: "/thing"},
	}
	for _, tc := range tests {
		route, err := tc.method.Route()
		if err != nil {
			t.Fatalf("Route(%+v): %v", tc.method, err)
		}
		if route.Pattern() != tc.pattern {
			t.Fatalf("Route(%+v).Pattern() = %q, want %q", tc.method, route.Pattern(), tc.pattern)
		}
		if route.Wildcard != tc.wildcard {
			t.Fatalf("Route(%+v).Wildcard = %v, want %v", tc.method, route.Wildcard, tc.wildcard)
		}
	}
}

func TestRouteDerivationErrors(t *testing.T) {
	tests := []struct {
		method Method
		want   string
	}{
		{method: Method{Name: "Rill"}, want: "cp.http_method"},
		{method: Method{Name: "Rill", URL: "/rill"}, want: "cp.http_method"},
		{method: Method{Name: "Get_"}, want: "set cp.url"},
		{method: Method{Name: "Rill", HTTPMethod: HTTPMethod(9)}, want: "unknown cp.http_method"},
	}
	for _, tc := range tests {
		_, err := tc.method.Route()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Route(%+v) error = %v, want containing %q", tc.method, err, tc.want)
		}
	}
}

func TestValidateRoutes(t *testing.T) {
	root := Method{Name: "Static", HTTPMethod: HTTPMethodAny, URL: "/", GoCustom: true, InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}
	rill := Method{Name: "Rill", HTTPMethod: HTTPMethodAny, URL: "/rill/{path...}", GoCustom: true, InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}
	health := Method{Name: "GetV1Healthz", InputFullName: "cp.Empty", OutputFullName: "demo.Health"}
	if err := ValidateRoutes(Service{Name: "Demo", Methods: []Method{root, rill, health}}); err != nil {
		t.Fatalf("ValidateRoutes: %v", err)
	}
	tests := []struct {
		name    string
		methods []Method
		want    string
	}{
		{name: "any without custom", methods: []Method{{Name: "Rill", HTTPMethod: HTTPMethodAny, URL: "/rill/{path...}", InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}}, want: "requires cp.go_custom"},
		{name: "any with body", methods: []Method{{Name: "Rill", HTTPMethod: HTTPMethodAny, URL: "/rill/{path...}", GoCustom: true, InputFullName: "demo.Req", OutputFullName: "cp.Empty"}}, want: "requires cp.Empty input"},
		{name: "wildcard without custom", methods: []Method{{Name: "GetThingV1", URL: "/v1/thing/{id}", InputFullName: "cp.Empty", OutputFullName: "demo.Thing"}}, want: "requires cp.go_custom"},
		{name: "root without custom", methods: []Method{{Name: "Get", InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}}, want: "requires cp.go_custom"},
		{name: "verbless", methods: []Method{{Name: "Rill", InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}}, want: "cp.http_method"},
		{name: "method conflict", methods: []Method{{Name: "Get", GoCustom: true, InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}, rill}, want: "conflicts with pattern \"GET /\""},
		{name: "same requests", methods: []Method{rill, {Name: "Subtree", HTTPMethod: HTTPMethodAny, URL: "/rill/", GoCustom: true, InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}}, want: "matches the same requests"},
		{name: "duplicate", methods: []Method{health, health}, want: "conflicts"},
		{name: "bad pattern", methods: []Method{{Name: "GetThingV1", URL: "/v1/thing/{", GoCustom: true, InputFullName: "cp.Empty", OutputFullName: "cp.Empty"}}, want: "rpc GetThingV1: "},
	}
	for _, tc := range tests {
		err := ValidateRoutes(Service{Name: "Demo", Methods: tc.methods})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error = %v, want containing %q", tc.name, err, tc.want)
		}
		if strings.Contains(err.Error(), "registered at") {
			t.Fatalf("%s: error should not carry registration sites: %v", tc.name, err)
		}
	}
}
