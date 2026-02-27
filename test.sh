#!/bin/sh
set -e

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)

mkdir -p "$ROOT_DIR/.build/go" "$ROOT_DIR/.build/js"

go run ./cmd/cleanproto \
  -proto_path "$ROOT_DIR/example" \
  "example.proto"
