# cleanproto

A minimal proto3 generator that generates clean, readable code, supporting direct conversion to natural native types.

Currently only supports Go and Javascript.

## Install
```
go install github.com/jptrs93/cleanproto/cmd/cleanproto@latest
```

## Usage
```
cleanproto -proto_path ../protos -go.out ./gen/go -go.pkg api -js.out ./gen/js example.proto
```

## End-to-end example

Proto (`audit.proto`):

```proto
syntax = "proto3";

package demo;

import "cleanproto/options.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

option go_package = "demo";

message AuditEvent {
  int64 occurred_at = 1 [(cp.go.type) = "time.Time"];
  google.protobuf.Duration timeout = 2 [(cp.go.type) = "time.Duration", (cp.js.type) = "bigint"];
  bytes request_id = 3 [(cp.go.type) = "github.com/google/uuid.UUID"];
  int64 actor_id = 4 [(cp.js.type) = "bigint"];
  google.protobuf.Timestamp synced_at = 5 [(cp.js.type) = "number"];
}
```

Generate:

```bash
cleanproto \
  -proto_path ../protos \
  -go.out ./gen/go \
  -go.pkg demo \
  -go.jsontags snake \
  -js.out ./gen/js \
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
    OccurredAt time.Time     `json:"occurred_at"`
    Timeout    time.Duration `json:"timeout"`
    RequestID  uuid.UUID     `json:"request_id"`
    ActorID    int64         `json:"actor_id"`
    SyncedAt   time.Time     `json:"synced_at"`
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
import {Reader, Writer} from "protobufjs/minimal";

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

## Options in .proto
- `option go_package = "module/path;pkg";` for Go package name.
- `option (cp.go.type) = "time.Time" | "time.Duration" | "github.com/google/uuid.UUID";` on fields for Go native type conversion (requires `import "cleanproto/options.proto";`).
- `option (cp.js.type) = "number" | "bigint";` on fields for JS native type conversion (requires `import "cleanproto/options.proto";`).

## CLI args
- `-go.out` output directory for Go.
- `-go.pkg` Go package name for generated code.
- `-go.jsontags snake` Go JSON tag style; omit to generate no JSON tags.
- `-js.out` output directory for JS.

## Notes
- Unknown fields are ignored on decode.
- `oneof` not supported.
- Generated Javascript code uses `protobufjs/minimal`.
- Go output embeds `util.gen.go` which requires `google.golang.org/protobuf/encoding/protowire`.
- When you specify a native type (for example `(cp.go.type) = "time.Duration"` or `(cp.js.type) = "bigint"`), it does not change the wire serialization; it only changes generated API types plus conversion code. For example, `int32 created_at = 1 [(cp.go.type) = "time.Time"];` still encodes on the wire as `int32` varint. It is assumed as epoch seconds. And if you used int64 it would be assumed as epoch milliseconds. These assumed conversions at not configurable. You can only add a native type option on a compatible wire type with a supported conversion. A native type option on an incompatible wire type will result in an error.
