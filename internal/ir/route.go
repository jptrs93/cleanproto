package ir

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

type HTTPMethod int32

const (
	HTTPMethodUnspecified HTTPMethod = 0
	HTTPMethodAny         HTTPMethod = 1
	HTTPMethodGet         HTTPMethod = 2
	HTTPMethodPost        HTTPMethod = 3
	HTTPMethodPut         HTTPMethod = 4
	HTTPMethodPatch       HTTPMethod = 5
	HTTPMethodDelete      HTTPMethod = 6
)

func (h HTTPMethod) String() string {
	switch h {
	case HTTPMethodGet:
		return "GET"
	case HTTPMethodPost:
		return "POST"
	case HTTPMethodPut:
		return "PUT"
	case HTTPMethodPatch:
		return "PATCH"
	case HTTPMethodDelete:
		return "DELETE"
	}
	return ""
}

type Route struct {
	Method   string
	Path     string
	Wildcard bool
}

func (r Route) Pattern() string {
	if r.Method == "" {
		return r.Path
	}
	return r.Method + " " + r.Path
}

var verbPrefixes = []string{"Get", "Post", "Put", "Patch", "Delete"}

func splitVerb(name string) (verb string, rest string) {
	for _, p := range verbPrefixes {
		if strings.HasPrefix(name, p) {
			return strings.ToUpper(p), strings.TrimPrefix(name, p)
		}
	}
	return "", name
}

func (m Method) Route() (Route, error) {
	verb, rest := splitVerb(m.Name)
	method := verb
	switch m.HTTPMethod {
	case HTTPMethodUnspecified:
		if verb == "" {
			return Route{}, fmt.Errorf("rpc %s: name must start with Get, Post, Put, Patch or Delete, or set cp.http_method", m.Name)
		}
	case HTTPMethodAny:
		method = ""
	default:
		method = m.HTTPMethod.String()
		if method == "" {
			return Route{}, fmt.Errorf("rpc %s: unknown cp.http_method value %d", m.Name, m.HTTPMethod)
		}
	}
	path := m.URL
	if path == "" {
		source := rest
		if verb == "" {
			source = m.Name
		}
		derived, ok := derivePath(source)
		if !ok {
			return Route{}, fmt.Errorf("rpc %s: cannot derive a URL path from the name; set cp.url", m.Name)
		}
		path = derived
	}
	return Route{Method: method, Path: path, Wildcard: isWildcardPath(path)}, nil
}

func derivePath(rest string) (string, bool) {
	if rest == "" || rest == "Root" {
		return "/", true
	}
	version := ""
	if strings.HasSuffix(rest, "V1") {
		rest = strings.TrimSuffix(rest, "V1")
		version = "v1"
	}
	parts := strings.Split(rest, "_")
	first := camelWords(parts[0])
	if len(first) == 0 {
		return "", false
	}
	base := "/"
	if version != "" {
		base += version + "/"
	}
	base += strings.Join(first, "/")
	for _, seg := range parts[1:] {
		words := camelWords(seg)
		if len(words) == 0 {
			continue
		}
		base += "-" + strings.Join(words, "-")
	}
	return base, true
}

func camelWords(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower) || unicode.IsDigit(prev)) {
				out = append(out, strings.ToLower(b.String()))
				b.Reset()
			}
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, strings.ToLower(b.String()))
	}
	return out
}

func isWildcardPath(path string) bool {
	if strings.HasSuffix(path, "{$}") {
		return strings.Contains(strings.TrimSuffix(path, "{$}"), "{")
	}
	return strings.Contains(path, "{") || strings.HasSuffix(path, "/")
}

func (m Method) inputEmpty() bool {
	return strings.HasSuffix(m.InputFullName, ".Empty")
}

func ValidateRoutes(svc Service) error {
	mux := http.NewServeMux()
	for _, m := range svc.Methods {
		route, err := m.Route()
		if err != nil {
			return err
		}
		if route.Method == "" && !m.GoCustom {
			return fmt.Errorf("rpc %s: cp.http_method HTTP_METHOD_ANY requires cp.go_custom", m.Name)
		}
		if route.Method == "" && !m.inputEmpty() {
			return fmt.Errorf("rpc %s: cp.http_method HTTP_METHOD_ANY requires cp.Empty input", m.Name)
		}
		if route.Wildcard && !m.GoCustom {
			return fmt.Errorf("rpc %s: wildcard route %q requires cp.go_custom", m.Name, route.Path)
		}
		if err := registerPattern(mux, route.Pattern()); err != nil {
			return fmt.Errorf("rpc %s: %s", m.Name, err)
		}
	}
	return nil
}

var registeredAt = regexp.MustCompile(` \(registered at [^)]*\)`)

func registerPattern(mux *http.ServeMux, pattern string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := strings.TrimPrefix(fmt.Sprint(r), "http: ")
			err = fmt.Errorf("%s", registeredAt.ReplaceAllString(msg, ""))
		}
	}()
	mux.Handle(pattern, http.NotFoundHandler())
	return nil
}
