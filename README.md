# cleanproto

A minimal proto3 generator that generates clean, fast, readable code, supporting natural native types for things like timestamps.

Currently supports Go, JavaScript, and TypeScript.

| Language | Models | Client stubs | Server stubs |
| --- | --- | --- | --- |
| Go | Yes | No | Yes |
| JavaScript | Yes | No | No |
| TypeScript | Yes | No | No |

## Install
```
go install github.com/jptrs93/cleanproto/cmd/cleanproto@latest
```

## Usage
```
cleanproto -proto_path ../protos -go.out ./apigen/go -js.out ./apigen/js -ts.out ./apigen/ts example.proto
```

| Option | Required | Description | Default |
| --- | --- | --- | --- |
| `-proto_path <dir>` | No | Proto import path. Repeatable. | `.` |
| `-go.out <dir>` | One of `-go.out`, `-js.out`, `-ts.out` is required | Output directory for generated Go files. | none |
| `-go.jsontags <style>` | No | Go JSON tags style. Supported: `snake`. | none |
| `-js.out <dir>` | One of `-go.out`, `-js.out`, `-ts.out` is required | Output directory for generated JavaScript files. | none |
| `-ts.out <dir>` | One of `-go.out`, `-js.out`, `-ts.out` is required | Output directory for generated TypeScript files. | none |

Positional args: one or more `.proto` files to generate.

> [!IMPORTANT]
> Generated code relies on `google.golang.org/protobuf/encoding/protowire` for Go and `protobufjs/minimal` for JavaScript/TypeScript. You must add these dependencies to your project.

### Native type support

`cleanproto` provides options so you can direct it to generate more natural language native types for certain field types. This doesn't change the ultimate on wire byte representation but conversion to the native type gets baked into the generated decode/encode functions. For example.

```protobuf
   google.protobuf.Timestamp timestamp = 1 [(cp.js_type) = "Date", (cp.go_type) = "time.Time"];
```

Will generate models where the `timestamp` field has the type `Date` and `time.Time` in javascript and go respectively. 

#### Go

| Native type option | Supported wire types |
| --- | --- |
| `cp.go_type = "time.Time"` | `google.protobuf.Timestamp`, `int32`, `int64` |
| `cp.go_type = "time.Duration"` | `google.protobuf.Duration`, `int32`, `int64` |
| `cp.go_type = "github.com/google/uuid.UUID"` | `bytes` |

#### JavaScript

| Native type option | Supported wire types |
| --- | --- |
| `cp.js_type = "Date"` | `google.protobuf.Timestamp`, `int32`, `int64` |
| `cp.js_type = "number"` | `int32`, `int64`, `google.protobuf.Timestamp`, `google.protobuf.Duration` |
| `cp.js_type = "bigint"` | `int32`, `int64`, `google.protobuf.Timestamp`, `google.protobuf.Duration` |

#### TypeScript

| Native type option | Supported wire types |
| --- | --- |
| `cp.ts_type = "Date"` | `google.protobuf.Timestamp`, `int32`, `int64` |
| `cp.ts_type = "number"` | `int32`, `int64`, `google.protobuf.Timestamp`, `google.protobuf.Duration` |
| `cp.ts_type = "bigint"` | `int32`, `int64`, `google.protobuf.Timestamp`, `google.protobuf.Duration` |

> [!NOTE]
> Native type conversion is standardized and may lose precision when the proto wire type is less precise than the selected native type. For example, if the native JavaScript type is `Date` but the wire type is `int32`, then values are converted to and from epoch seconds to fit `int32` precision.

### Additional options

| Option | Effect |
| --- | --- |
| `cp.go_encode = false` | Keep the field in generated Go models, but skip writing it during Go encoding. |
| `cp.go_ignore = true` | Omit the field completely from generated Go models and their encode/decoding. |
| `cp.js_encode = false` | Keep the field in generated JavaScript models, but skip writing it during JS encoding. |
| `cp.js_ignore = true` | Omit the field completely from generated JavaScript models and their encode/decoding. |
| `cp.ts_encode = false` | Keep the field in generated TypeScript models, but skip writing it during TS encoding. |
| `cp.ts_ignore = true` | Omit the field completely from generated TypeScript models and their encode/decoding. |

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
  -js.out ./gen/js \
  -ts.out ./gen/ts \
  audit.proto
```

<details>
<summary>Show Go output</summary>

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
	if !m.SyncedAt.IsZero() {
		b = AppendBytesField(b, EncodeTimestamp(m.SyncedAt), 5)
	}
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
			b, m.OccurredAt, err = ConsumeTimeFromInt64(b, typ)
		case 2:
			b, m.Timeout, err = ConsumeDurationFromDuration(b, typ)
		case 3:
			b, m.RequestID, err = ConsumeUUIDFromBytes(b, typ)
		case 4:
			b, m.ActorID, err = ConsumeVarInt64(b, typ)
		case 5:
			b, m.SyncedAt, err = ConsumeTimeFromTimestamp(b, typ)
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

</details>

<details>
<summary>Show JavaScript output</summary>

```js
/**
 * @typedef {Object} AuditEvent
 * @property {number} occurredAt
 * @property {bigint} timeout
 * @property {Uint8Array} requestId
 * @property {bigint} actorId
 * @property {number} syncedAt
 */
/**
 * @typedef {Object} ApiErr
 * @property {number} code
 * @property {string} displayErr
 * @property {string} internalErr
 */
import protobufjsm from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

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
            case 1: {
                message.occurredAt = readInt64(reader, "int64");
                break;
            }
            case 2: {
                message.timeout = decodeDurationBigIntMessage(reader, reader.uint32());
                break;
            }
            case 3: {
                message.requestId = reader.bytes();
                break;
            }
            case 4: {
                message.actorId = readInt64BigInt(reader, "int64");
                break;
            }
            case 5: {
                message.syncedAt = decodeTimestampMillisMessage(reader, reader.uint32());
                break;
            }
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

export function writeApiErr(message, writer) {
    if (message.code !== undefined && message.code !== null && message.code !== 0) {
        writer.uint32(tag(1, WIRE.VARINT)).int32(message.code);
    }
    if (message.displayErr !== undefined && message.displayErr !== null && message.displayErr !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.displayErr);
    }
    if (message.internalErr !== undefined && message.internalErr !== null && message.internalErr !== "") {
        writer.uint32(tag(3, WIRE.LDELIM)).string(message.internalErr);
    }
}
```

</details>

<details>
<summary>Show TypeScript output</summary>

```ts
import protobufjsm from 'protobufjs/minimal';
import type { Reader, Writer } from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

export interface AuditEvent {
  occurredAt: bigint;
  timeout: number;
  requestId: Uint8Array;
  actorId: bigint;
  syncedAt: Date;
}
export interface ApiErr {
  code: number;
  displayErr: string;
  internalErr: string;
}

export function writeAuditEvent(message: AuditEvent, writer: Writer): void {
  if (message.occurredAt !== undefined && message.occurredAt !== null && message.occurredAt !== 0n) {
    writer.uint32(tag(1, WIRE.VARINT)).int64(message.occurredAt.toString());
  }
  if (message.timeout !== undefined && message.timeout !== null) {
    writer.uint32(tag(2, WIRE.LDELIM)).fork();
    writeDuration(message.timeout, writer);
    writer.ldelim();
  }
  if (message.requestId && message.requestId.length > 0) {
    writer.uint32(tag(3, WIRE.LDELIM)).bytes(message.requestId);
  }
  if (message.actorId !== undefined && message.actorId !== null && message.actorId !== 0n) {
    writer.uint32(tag(4, WIRE.VARINT)).int64(message.actorId.toString());
  }
  if (message.syncedAt !== undefined && message.syncedAt !== null) {
    writer.uint32(tag(5, WIRE.LDELIM)).fork();
    writeTimestamp(message.syncedAt, writer);
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
  const message: AuditEvent = {occurredAt: 0n, timeout: 0, requestId: new Uint8Array(0), actorId: 0n, syncedAt: new Date(0) };
  while (reader.pos < end) {
    const tag = reader.uint32();
    switch (tag >>> 3) {
      case 1: {
        message.occurredAt = readInt64BigInt(reader, "int64");
        break;
      }
      case 2: {
        message.timeout = decodeDurationMessage(reader, reader.uint32());
        break;
      }
      case 3: {
        message.requestId = reader.bytes();
        break;
      }
      case 4: {
        message.actorId = readInt64BigInt(reader, "int64");
        break;
      }
      case 5: {
        message.syncedAt = decodeTimestampMessage(reader, reader.uint32());
        break;
      }
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

export function writeApiErr(message: ApiErr, writer: Writer): void {
  if (message.code !== undefined && message.code !== null && message.code !== 0) {
    writer.uint32(tag(1, WIRE.VARINT)).int32(message.code);
  }
  if (message.displayErr !== undefined && message.displayErr !== null && message.displayErr !== "") {
    writer.uint32(tag(2, WIRE.LDELIM)).string(message.displayErr);
  }
  if (message.internalErr !== undefined && message.internalErr !== null && message.internalErr !== "") {
    writer.uint32(tag(3, WIRE.LDELIM)).string(message.internalErr);
  }
}
```

</details>

## Notes
- Unknown fields are ignored on decode.
- `oneof` not supported.
- `cp.<lang>_ignore = true` takes precedence over `cp.<lang>_encode = false` for that language, since ignored fields are omitted entirely.
