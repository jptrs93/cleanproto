# cleanproto

A minimal proto3 generator that generates clean, readable code, supporting direct conversion to natural native types.

Currently only supports Go and Javascript.

## Install
```
go install github.com/jptrs93/cleanproto/cmd/cleanproto@latest
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
- `oneof` not supported.
- Generated Javascript code uses `protobufjs/minimal`.
- Go output embeds `util.go` which uses `google.golang.org/protobuf/encoding/protowire`.
