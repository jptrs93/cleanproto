#!/bin/sh
set -e

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)

mkdir -p "$ROOT_DIR/.build/go" "$ROOT_DIR/.build/js"

go run ./cmd/cleanproto \
  -proto_path "$ROOT_DIR/example" \
  -go_out "$ROOT_DIR/.build/go" \
  -go_pkg example \
  -js_out "$ROOT_DIR/.build/js" \
  "$ROOT_DIR/example/example.proto"
