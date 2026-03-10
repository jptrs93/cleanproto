# cleanproto

A minimal proto3 generator that generates clean, fast, readable code, supporting natural native types for things like timestamps.

Currently supports Go, JavaScript, and TypeScript.

## Install
```
go install github.com/jptrs93/cleanproto/cmd/cleanproto@latest
```

## Usage
```
cleanproto -proto_path ../protos -go.out ./gen/go -go.pkg api -js.out ./gen/js -ts.out ./gen/ts example.proto
```

## End-to-end example

Proto (`audit.proto`):

```proto
syntax = "proto3";

package demo;

import "options.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

option go_package = "demo";

message AuditEvent {
  int64 occurred_at = 1 [(cp.go_type) = "time.Time"];
  google.protobuf.Duration timeout = 2 [(cp.go_type) = "time.Duration", (cp.js_type) = "bigint"];
  bytes request_id = 3 [(cp.go_type) = "github.com/google/uuid.UUID"];
  int64 actor_id = 4 [(cp.js_type) = "bigint"];
  google.protobuf.Timestamp synced_at = 5 [(cp.js_type) = "number"];
}
```

Generate:

```bash
cleanproto \
  -proto_path ../protos \
  -go.out ./gen/go \
  -go.pkg demo \
  -js.out ./gen/js \
  -ts.out ./gen/ts \
  audit.proto
```

Example Go output (`gen/go/model.gen.go`):

```go
package demo

import (
    "github.com/google/uuid"
    "google.golang.org/protobuf/encoding/protowire"
    "time"
)

type AuditEvent struct {
    OccurredAt time.Time
    Timeout    time.Duration
    RequestID  uuid.UUID
    ActorID    int64
    SyncedAt   time.Time
}

func (m *AuditEvent) Encode() []byte {
    var b []byte
    b = AppendInt64FromTime(b, m.OccurredAt, 1)
    b = AppendDurationFromDuration(b, m.Timeout, 2)
    b = AppendBytesFromUUID(b, m.RequestID, 3)
    b = AppendInt64Field(b, m.ActorID, 4)
    b = AppendTimestampFromTime(b, m.SyncedAt, 5)
    return b
}

func DecodeAuditEvent(b []byte) (*AuditEvent, error) {
    var m AuditEvent
    var num protowire.Number
    var typ protowire.Type
    var err error
    for len(b) > 0 {
        b, num, typ, err = ConsumeTag(b)
        if err != nil {
            return nil, err
        }
        switch num {
        case 1:
            var item time.Time
            b, item, err = ConsumeTimeFromInt64(b, typ)
            if err == nil {
                m.OccurredAt = item
            }
        case 2:
            var item time.Duration
            b, item, err = ConsumeDurationFromDuration(b, typ)
            if err == nil {
                m.Timeout = item
            }
        case 3:
            var item uuid.UUID
            b, item, err = ConsumeUUIDFromBytes(b, typ)
            if err == nil {
                m.RequestID = item
            }
        case 4:
            b, m.ActorID, err = ConsumeVarInt64(b, typ)
        case 5:
            var item time.Time
            b, item, err = ConsumeTimeFromTimestamp(b, typ)
            if err == nil {
                m.SyncedAt = item
            }
        default:
            b, err = SkipFieldValue(b, num, typ)
        }
        if err != nil {
            return nil, err
        }
    }
    return &m, nil
}
```

Example JS output (`gen/js/model.js`):

```js
import protobufjsm from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

/**
 * @typedef {Object} AuditEvent
 * @property {number} occurredAt
 * @property {bigint} timeout
 * @property {Uint8Array} requestId
 * @property {bigint} actorId
 * @property {number} syncedAt
 */

export function writeAuditEvent(message, writer) {
    if (message.occurredAt !== undefined && message.occurredAt !== null && message.occurredAt !== 0) {
        writer.uint32(tag(1, WIRE.VARINT)).int64(message.occurredAt);
    }
    if (message.timeout !== undefined && message.timeout !== null && message.timeout !== 0n) {
        writer.uint32(tag(2, WIRE.LDELIM)).fork();
        writeDurationFromBigInt(message.timeout, writer);
        writer.ldelim();
    }
    if (message.requestId && message.requestId.length > 0) {
        writer.uint32(tag(3, WIRE.LDELIM)).bytes(message.requestId);
    }
    if (message.actorId !== undefined && message.actorId !== null && message.actorId !== 0n) {
        writer.uint32(tag(4, WIRE.VARINT)).int64(message.actorId.toString());
    }
    if (message.syncedAt !== undefined && message.syncedAt !== null && message.syncedAt !== 0) {
        writer.uint32(tag(5, WIRE.LDELIM)).fork();
        writeTimestampFromMillis(message.syncedAt, writer);
        writer.ldelim();
    }
}

export function encodeAuditEvent(message) {
    const writer = Writer.create();
    writeAuditEvent(message, writer);
    return writer.finish();
}

function decodeAuditEventMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {occurredAt: 0, timeout: 0n, requestId: new Uint8Array(0), actorId: 0n, syncedAt: 0 };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1:
                message.occurredAt = readInt64(reader, "int64");
                break;
            case 2:
                message.timeout = decodeDurationBigIntMessage(reader, reader.uint32());
                break;
            case 3:
                message.requestId = reader.bytes();
                break;
            case 4:
                message.actorId = readInt64BigInt(reader, "int64");
                break;
            case 5:
                message.syncedAt = decodeTimestampMillisMessage(reader, reader.uint32());
                break;
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}

export function decodeAuditEvent(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeAuditEventMessage(reader);
}
```

Example TS output (`gen/ts/model.ts`):

```ts
export interface AuditEvent {
  occurredAt: bigint;
  timeout: bigint;
  requestId: Uint8Array;
  actorId: bigint;
  syncedAt: number;
}

import protobufjsm from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

export function writeAuditEvent(message: AuditEvent, writer: Writer): void {
  if (message.occurredAt !== undefined && message.occurredAt !== null && message.occurredAt !== 0n) {
    writer.uint32(tag(1, WIRE.VARINT)).int64(message.occurredAt.toString());
  }
  if (message.timeout !== undefined && message.timeout !== null && message.timeout !== 0n) {
    writer.uint32(tag(2, WIRE.LDELIM)).fork();
    writeDurationFromBigInt(message.timeout, writer);
    writer.ldelim();
  }
  if (message.requestId && message.requestId.length > 0) {
    writer.uint32(tag(3, WIRE.LDELIM)).bytes(message.requestId);
  }
  if (message.actorId !== undefined && message.actorId !== null && message.actorId !== 0n) {
    writer.uint32(tag(4, WIRE.VARINT)).int64(message.actorId.toString());
  }
  if (message.syncedAt !== undefined && message.syncedAt !== null && message.syncedAt !== 0) {
    writer.uint32(tag(5, WIRE.LDELIM)).fork();
    writeTimestampFromMillis(message.syncedAt, writer);
    writer.ldelim();
  }
}

export function encodeAuditEvent(message: AuditEvent): Uint8Array {
  const writer = Writer.create();
  writeAuditEvent(message, writer);
  return writer.finish();
}

function decodeAuditEventMessage(reader: Reader, length?: number): AuditEvent {
  const end = length === undefined ? reader.len : reader.pos + length;
  const message: AuditEvent = { occurredAt: 0n, timeout: 0n, requestId: new Uint8Array(0), actorId: 0n, syncedAt: 0 };
  while (reader.pos < end) {
    const tag = reader.uint32();
    switch (tag >>> 3) {
      case 1:
        message.occurredAt = readInt64BigInt(reader, 'int64');
        break;
      case 2:
        message.timeout = decodeDurationBigIntMessage(reader, reader.uint32());
        break;
      case 3:
        message.requestId = reader.bytes();
        break;
      case 4:
        message.actorId = readInt64BigInt(reader, 'int64');
        break;
      case 5:
        message.syncedAt = decodeTimestampMillisMessage(reader, reader.uint32());
        break;
      default:
        reader.skipType(tag & 7);
    }
  }
  return message;
}

export function decodeAuditEvent(buffer: ArrayBuffer): AuditEvent {
  const reader = Reader.create(new Uint8Array(buffer));
  return decodeAuditEventMessage(reader);
}
```

## Options in .proto
- `option go_package = "module/path;pkg";` for Go package name.
- `option (cp.go_type) = "time.Time" | "time.Duration" | "github.com/google/uuid.UUID";` on fields for Go native type conversion (requires `import "options.proto";`).
- `option (cp.go_encode) = false;` on fields to keep the field in generated Go type definitions but skip writing it during Go encoding.
- `option (cp.go_ignore) = true;` on fields to omit the field from generated Go type definitions and Go encode/decode.
- `option (cp.js_type) = "number" | "bigint" | "Date";` on fields for JS native type conversion (requires `import "options.proto";`).
- `option (cp.js_encode) = false;` on fields to keep the field in generated JS typedefs but skip writing it during JS encoding.
- `option (cp.js_ignore) = true;` on fields to omit the field from generated JS typedefs and JS encode/decode.
- `option (cp.ts_type) = "number" | "bigint" | "Date";` on fields for TS native type conversion (requires `import "options.proto";`). In TS output, 64-bit integer wire types default to `bigint` when no explicit `cp.ts_type` is set.
- `option (cp.ts_encode) = false;` on fields to keep the field in generated TS type definitions but skip writing it during TS encoding.
- `option (cp.ts_ignore) = true;` on fields to omit the field from generated TS type definitions and TS encode/decode.

## CLI args
- `-go.out` output directory for Go.
- `-go.pkg` Go package name for generated code.
- `-go.jsontags snake` Go JSON tag style; omit to generate no JSON tags.
- `-js.out` output directory for JS.
- `-ts.out` output directory for TS.

## Notes
- Unknown fields are ignored on decode.
- `oneof` not supported.
- Generated Javascript code uses `protobufjs/minimal`.
- Go output embeds `util.gen.go` which requires `google.golang.org/protobuf/encoding/protowire`.
- `cp.<lang>.ignore = true` takes precedence over `cp.<lang>.encode = false` for that language, since ignored fields are omitted entirely.
- For `cp.js_type = "Date"`, supported wire types are `google.protobuf.Timestamp`, `int32` (assumed epoch seconds), and `int64` (assumed epoch seconds).
- When you specify a native type (for example `(cp.go_type) = "time.Duration"` or `(cp.js_type) = "bigint"`), it does not change the wire serialization; it only changes generated API types plus conversion code. For example, `int32 created_at = 1 [(cp.go_type) = "time.Time"];` still encodes on the wire as `int32` varint. It is assumed as epoch seconds. And if you used int64 it would be assumed as epoch milliseconds. These assumed conversions at not configurable. You can only add a native type option on a compatible wire type with a supported conversion. A native type option on an incompatible wire type will result in an error.
