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
