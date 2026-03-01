package ir

type File struct {
	Path      string
	Package   string
	GoPackage string
	GoOut     string
	JsOut     string
	Enums     []Enum
	Messages  []Message
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
