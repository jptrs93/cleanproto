# cleanproto

Minimal proto3 generator for Go and JS.

## Features
- Generates Go structs with `Encode()` and `DecodeX` functions.
- Generates JS encode/decode functions using `protobufjs/minimal`.
- Supports proto3 only (no `oneof`, no services).
- Optional fields map to pointers in Go and `undefined` in JS.

## Install
```
go build ./cmd/cleanproto
```

## Usage
```
cleanproto -proto_path ../protos -go_out ./gen/go -go_pkg api -js_out ./gen/js example.proto
```

## Options in .proto
- `option go_package = "module/path;pkg";` for Go package name.
- `option (cleanproto.go_out) = "./gen/go";` for Go output path (requires `import "cleanproto/options.proto";`).
- `option (cleanproto.js_out) = "./gen/js";` for JS output path (requires `import "cleanproto/options.proto";`).

## Notes
- Unknown fields are ignored on decode.
- Go output embeds `util.go` sourced from `../jnotes/app/protowireu/protowireu.go` and uses `google.golang.org/protobuf/encoding/protowire`.
```
