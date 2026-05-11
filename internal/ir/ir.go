package ir

type File struct {
	Path      string
	Package   string
	GoPackage string
	Enums     []Enum
	Messages  []Message
	Services  []Service
}

type Service struct {
	Name    string
	Methods []Method
}

type Method struct {
	Name              string
	InputFullName     string
	OutputFullName    string
	GoCustom          bool
	OperationID       string
	Audit             bool
	IsStreamingServer bool
	PolicyType        int32
	PolicyScopes      []string
	CompressionMode   int32
}

type Enum struct {
	Name     string
	FullName string
	Values   []EnumValue
}

type EnumValue struct {
	Name   string
	Number int32
}

type Message struct {
	Name     string
	FullName string
	Fields   []Field
}

type Field struct {
	Name            string
	ProtoName       string
	Number          int
	Kind            Kind
	IsRepeated      bool
	IsOptional      bool
	IsPacked        bool
	IsMap           bool
	IsTimestamp     bool
	IsDuration      bool
	GoType          string
	JSType          string
	TSType          string
	GoEncode        bool
	GoIgnore        bool
	GoSlicePtr      *bool
	JsEncode        bool
	JsIgnore        bool
	TsEncode        bool
	TsIgnore        bool
	JSONIgnore      bool
	AuditIgnore     bool
	MapKeyKind      Kind
	MapValueKind    Kind
	MapValueMessage string
	MapValueEnum    string
	MessageFullName string
	EnumFullName    string
	Constraints     FieldConstraints
}

type IgnoreMode int

const (
	IgnoreUnspecified IgnoreMode = iota
	IgnoreIfZeroValue
	IgnoreAlways
)

type FieldConstraints struct {
	Required bool
	Ignore   IgnoreMode
	Bool     *BoolRules
	Numeric  *NumericRules
	String   *StringRules
	Bytes    *BytesRules
	Enum     *EnumRules
	Repeated *RepeatedRules
	Map      *MapRules
}

type NumericRules struct {
	Const *string
	Gt    *string
	Gte   *string
	Lt    *string
	Lte   *string
	In    []string
	NotIn []string
}

type StringRules struct {
	Const       *string
	Len         *uint64
	MinLen      *uint64
	MaxLen      *uint64
	Pattern     string
	Prefix      string
	Suffix      string
	Contains    string
	NotContains string
	Email       bool
	UUID        bool
	In          []string
	NotIn       []string
}

type BytesRules struct {
	HasConst    bool
	Const       []byte
	Len         *uint64
	MinLen      *uint64
	MaxLen      *uint64
	Pattern     string
	HasPrefix   bool
	Prefix      []byte
	HasSuffix   bool
	Suffix      []byte
	HasContains bool
	Contains    []byte
}

type BoolRules struct {
	Const *bool
}

type EnumRules struct {
	Const       *int32
	DefinedOnly bool
	In          []int32
	NotIn       []int32
}

type RepeatedRules struct {
	MinItems *uint64
	MaxItems *uint64
	Unique   bool
	Items    *FieldConstraints
}

type MapRules struct {
	MinPairs *uint64
	MaxPairs *uint64
	Keys     *FieldConstraints
	Values   *FieldConstraints
}

func (c FieldConstraints) IsEmpty() bool {
	return !c.Required && c.Ignore == IgnoreUnspecified &&
		c.Bool == nil && c.Numeric == nil && c.String == nil && c.Bytes == nil &&
		c.Enum == nil && c.Repeated == nil && c.Map == nil
}

type Kind int

const (
	KindBool Kind = iota
	KindInt32
	KindInt64
	KindUint32
	KindUint64
	KindSint32
	KindSint64
	KindFixed32
	KindFixed64
	KindSfixed32
	KindSfixed64
	KindFloat
	KindDouble
	KindString
	KindBytes
	KindMessage
	KindEnum
)
