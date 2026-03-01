#!/bin/sh
set -e

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)

mkdir -p "$ROOT_DIR/.build/go" "$ROOT_DIR/.build/js"

go run ./cmd/cleanproto \
  -proto_path "$ROOT_DIR/example" \
  -go.out "$ROOT_DIR/.build/go" \
  -go.jsontags "snake" \
  -js.out "$ROOT_DIR/.build/js" \
  "example.proto"
