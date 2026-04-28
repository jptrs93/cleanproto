package parser

import (
	"log"
	"strconv"
	"sync"

	"github.com/jptrs93/cleanproto/internal/ir"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const bufValidateExtensionNumber = 1159

type validateContext struct {
	mu     sync.Mutex
	warned map[string]struct{}
}

func newValidateContext() *validateContext {
	return &validateContext{warned: map[string]struct{}{}}
}

func (vc *validateContext) warn(scope, format string, args ...any) {
	key := scope + "|" + format
	vc.mu.Lock()
	if _, ok := vc.warned[key]; ok {
		vc.mu.Unlock()
		return
	}
	vc.warned[key] = struct{}{}
	vc.mu.Unlock()
	log.Printf("cleanproto: %s: "+format, append([]any{scope}, args...)...)
}

func (vc *validateContext) parseFieldOptions(field protoreflect.FieldDescriptor) (ir.FieldConstraints, error) {
	var c ir.FieldConstraints
	if vc == nil {
		return c, nil
	}
	opts, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return c, nil
	}
	rules := findExtensionMessage(opts.ProtoReflect(), bufValidateExtensionNumber, "buf.validate.field")
	if rules == nil {
		return c, nil
	}
	if err := vc.fillFieldConstraints(string(field.FullName()), rules, &c); err != nil {
		return c, err
	}
	return c, nil
}

func (vc *validateContext) warnMessageOptions(msg protoreflect.MessageDescriptor) error {
	if vc == nil {
		return nil
	}
	opts, ok := msg.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil
	}
	rules := findExtensionMessage(opts.ProtoReflect(), bufValidateExtensionNumber, "buf.validate.message")
	if rules == nil {
		return nil
	}
	scope := string(msg.FullName())
	rules.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		switch fd.Name() {
		case "cel", "cel_expression":
			vc.warn(scope, "buf.validate.message.%s not supported (skipped)", fd.Name())
		case "oneof":
			vc.warn(scope, "buf.validate.message.oneof not supported (skipped)")
		}
		return true
	})
	return nil
}

// findExtensionMessage locates an embedded-message extension by number+full-name on the
// given options message. protocompile already decodes buf.validate extensions into the
// typed protoreflect surface, so they show up via Range rather than as unknown bytes.
func findExtensionMessage(rm protoreflect.Message, fieldNum int32, fullName string) protoreflect.Message {
	var found protoreflect.Message
	rm.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !fd.IsExtension() || int32(fd.Number()) != fieldNum {
			return true
		}
		if string(fd.FullName()) != fullName {
			return true
		}
		found = v.Message()
		return false
	})
	return found
}

func (vc *validateContext) fillFieldConstraints(scope string, rules protoreflect.Message, c *ir.FieldConstraints) error {
	var rangeErr error
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "required":
			c.Required = v.Bool()
		case "ignore":
			n := int32(v.Enum())
			switch n {
			case 0:
				c.Ignore = ir.IgnoreUnspecified
			case 1:
				c.Ignore = ir.IgnoreIfZeroValue
			case 3:
				c.Ignore = ir.IgnoreAlways
			default:
				vc.warn(scope, "buf.validate ignore=%d not supported (skipped)", n)
			}
		case "cel", "cel_expression":
			vc.warn(scope, "buf.validate.field.%s not supported (skipped)", fd.Name())
		case "float", "double", "int32", "int64", "uint32", "uint64",
			"sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64":
			c.Numeric = vc.parseNumericRules(scope, v.Message())
		case "bool":
			c.Bool = parseBoolRules(v.Message())
		case "string":
			c.String = vc.parseStringRules(scope, v.Message())
		case "bytes":
			c.Bytes = vc.parseBytesRules(scope, v.Message())
		case "enum":
			c.Enum = vc.parseEnumRules(scope, v.Message())
		case "repeated":
			r, err := vc.parseRepeatedRules(scope, v.Message())
			if err != nil {
				rangeErr = err
				return false
			}
			c.Repeated = r
		case "map":
			m, err := vc.parseMapRules(scope, v.Message())
			if err != nil {
				rangeErr = err
				return false
			}
			c.Map = m
		case "any", "duration", "field_mask", "timestamp":
			vc.warn(scope, "buf.validate.field.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	return rangeErr
}

func numericLiteral(kind protoreflect.Kind, v protoreflect.Value) (string, bool) {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return strconv.FormatInt(v.Int(), 10), true
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return strconv.FormatUint(v.Uint(), 10), true
	case protoreflect.FloatKind:
		return strconv.FormatFloat(v.Float(), 'g', -1, 32), true
	case protoreflect.DoubleKind:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), true
	}
	return "", false
}

func (vc *validateContext) parseNumericRules(scope string, rules protoreflect.Message) *ir.NumericRules {
	out := &ir.NumericRules{}
	any := false
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "const":
			if s, ok := numericLiteral(fd.Kind(), v); ok {
				out.Const = &s
				any = true
			}
		case "lt":
			if s, ok := numericLiteral(fd.Kind(), v); ok {
				out.Lt = &s
				any = true
			}
		case "lte":
			if s, ok := numericLiteral(fd.Kind(), v); ok {
				out.Lte = &s
				any = true
			}
		case "gt":
			if s, ok := numericLiteral(fd.Kind(), v); ok {
				out.Gt = &s
				any = true
			}
		case "gte":
			if s, ok := numericLiteral(fd.Kind(), v); ok {
				out.Gte = &s
				any = true
			}
		case "in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				if s, ok := numericLiteral(fd.Kind(), list.Get(i)); ok {
					out.In = append(out.In, s)
					any = true
				}
			}
		case "not_in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				if s, ok := numericLiteral(fd.Kind(), list.Get(i)); ok {
					out.NotIn = append(out.NotIn, s)
					any = true
				}
			}
		case "example", "finite":
			vc.warn(scope, "buf.validate numeric.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	if !any {
		return nil
	}
	return out
}

func parseBoolRules(rules protoreflect.Message) *ir.BoolRules {
	out := &ir.BoolRules{}
	any := false
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.Name() == "const" {
			b := v.Bool()
			out.Const = &b
			any = true
		}
		return true
	})
	if !any {
		return nil
	}
	return out
}

func (vc *validateContext) parseStringRules(scope string, rules protoreflect.Message) *ir.StringRules {
	out := &ir.StringRules{}
	any := false
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "const":
			s := v.String()
			out.Const = &s
			any = true
		case "len":
			n := v.Uint()
			out.Len = &n
			any = true
		case "min_len":
			n := v.Uint()
			out.MinLen = &n
			any = true
		case "max_len":
			n := v.Uint()
			out.MaxLen = &n
			any = true
		case "pattern":
			out.Pattern = v.String()
			any = true
		case "prefix":
			out.Prefix = v.String()
			any = true
		case "suffix":
			out.Suffix = v.String()
			any = true
		case "contains":
			out.Contains = v.String()
			any = true
		case "not_contains":
			out.NotContains = v.String()
			any = true
		case "email":
			if v.Bool() {
				out.Email = true
				any = true
			}
		case "uuid":
			if v.Bool() {
				out.UUID = true
				any = true
			}
		case "in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				out.In = append(out.In, list.Get(i).String())
				any = true
			}
		case "not_in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				out.NotIn = append(out.NotIn, list.Get(i).String())
				any = true
			}
		case "len_bytes", "min_bytes", "max_bytes",
			"strict", "example",
			"hostname", "ip", "ipv4", "ipv6", "uri", "uri_ref", "address",
			"tuuid", "ip_with_prefixlen", "ipv4_with_prefixlen", "ipv6_with_prefixlen",
			"ip_prefix", "ipv4_prefix", "ipv6_prefix", "host_and_port", "ulid",
			"protobuf_fqn", "protobuf_dot_fqn", "well_known_regex":
			vc.warn(scope, "buf.validate string.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	if !any {
		return nil
	}
	return out
}

func (vc *validateContext) parseBytesRules(scope string, rules protoreflect.Message) *ir.BytesRules {
	out := &ir.BytesRules{}
	any := false
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "const":
			out.HasConst = true
			out.Const = append([]byte(nil), v.Bytes()...)
			any = true
		case "len":
			n := v.Uint()
			out.Len = &n
			any = true
		case "min_len":
			n := v.Uint()
			out.MinLen = &n
			any = true
		case "max_len":
			n := v.Uint()
			out.MaxLen = &n
			any = true
		case "pattern":
			out.Pattern = v.String()
			any = true
		case "prefix":
			out.HasPrefix = true
			out.Prefix = append([]byte(nil), v.Bytes()...)
			any = true
		case "suffix":
			out.HasSuffix = true
			out.Suffix = append([]byte(nil), v.Bytes()...)
			any = true
		case "contains":
			out.HasContains = true
			out.Contains = append([]byte(nil), v.Bytes()...)
			any = true
		case "in", "not_in", "example",
			"ip", "ipv4", "ipv6", "ip_with_prefixlen", "ipv4_with_prefixlen", "ipv6_with_prefixlen":
			vc.warn(scope, "buf.validate bytes.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	if !any {
		return nil
	}
	return out
}

func (vc *validateContext) parseEnumRules(scope string, rules protoreflect.Message) *ir.EnumRules {
	out := &ir.EnumRules{}
	any := false
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "const":
			n := int32(v.Int())
			out.Const = &n
			any = true
		case "defined_only":
			if v.Bool() {
				out.DefinedOnly = true
				any = true
			}
		case "in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				out.In = append(out.In, int32(list.Get(i).Int()))
				any = true
			}
		case "not_in":
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				out.NotIn = append(out.NotIn, int32(list.Get(i).Int()))
				any = true
			}
		case "example":
			vc.warn(scope, "buf.validate enum.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	if !any {
		return nil
	}
	return out
}

func (vc *validateContext) parseRepeatedRules(scope string, rules protoreflect.Message) (*ir.RepeatedRules, error) {
	out := &ir.RepeatedRules{}
	any := false
	var rangeErr error
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "min_items":
			n := v.Uint()
			out.MinItems = &n
			any = true
		case "max_items":
			n := v.Uint()
			out.MaxItems = &n
			any = true
		case "unique":
			if v.Bool() {
				out.Unique = true
				any = true
			}
		case "items":
			ic := &ir.FieldConstraints{}
			if err := vc.fillFieldConstraints(scope+".items", v.Message(), ic); err != nil {
				rangeErr = err
				return false
			}
			if !ic.IsEmpty() {
				out.Items = ic
				any = true
			}
		case "example":
			vc.warn(scope, "buf.validate repeated.%s not supported (skipped)", fd.Name())
		}
		return true
	})
	if rangeErr != nil {
		return nil, rangeErr
	}
	if !any {
		return nil, nil
	}
	return out, nil
}

func (vc *validateContext) parseMapRules(scope string, rules protoreflect.Message) (*ir.MapRules, error) {
	out := &ir.MapRules{}
	any := false
	var rangeErr error
	rules.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "min_pairs":
			n := v.Uint()
			out.MinPairs = &n
			any = true
		case "max_pairs":
			n := v.Uint()
			out.MaxPairs = &n
			any = true
		case "keys":
			kc := &ir.FieldConstraints{}
			if err := vc.fillFieldConstraints(scope+".keys", v.Message(), kc); err != nil {
				rangeErr = err
				return false
			}
			if !kc.IsEmpty() {
				out.Keys = kc
				any = true
			}
		case "values":
			vcst := &ir.FieldConstraints{}
			if err := vc.fillFieldConstraints(scope+".values", v.Message(), vcst); err != nil {
				rangeErr = err
				return false
			}
			if !vcst.IsEmpty() {
				out.Values = vcst
				any = true
			}
		}
		return true
	})
	if rangeErr != nil {
		return nil, rangeErr
	}
	if !any {
		return nil, nil
	}
	return out, nil
}
