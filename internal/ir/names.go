package ir

import (
	"strings"
	"unicode"
)

func GoName(protoName string) string {
	parts := splitParts(protoName)
	if len(parts) == 0 {
		return ""
	}
	for i := range parts {
		if i == len(parts)-1 && parts[i] == "id" {
			parts[i] = "ID"
			continue
		}
		parts[i] = title(parts[i])
	}
	return strings.Join(parts, "")
}

func JsName(protoName string) string {
	parts := splitParts(protoName)
	if len(parts) == 0 {
		return ""
	}
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		parts[i] = title(parts[i])
	}
	return strings.Join(parts, "")
}

func splitParts(name string) []string {
	if name == "" {
		return nil
	}
	if strings.ContainsAny(name, "_-") {
		parts := strings.FieldsFunc(name, func(r rune) bool {
			return r == '_' || r == '-'
		})
		for i := range parts {
			parts[i] = strings.ToLower(parts[i])
		}
		return parts
	}
	return []string{name}
}

func title(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
