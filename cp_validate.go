package cp

import _ "embed"

const ValidateProtoPath = "buf/validate/validate.proto"

//go:embed buf/validate/validate.proto
var ValidateProtoSource string
