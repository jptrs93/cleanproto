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
 go install github.com/jptrs93/cleanproto/cmd/cleanproto@v1.2.11
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
| `-go.ctxtype <type>` | No | Go server auth context type for handler interface, verifyAuth return, and audit callback when server stubs are generated. | `context.Context` |
| `-js.out <dir>` | One of `-go.out`, `-js.out`, `-ts.out` is required | Output directory for generated JavaScript files. | none |
| `-ts.out <dir>` | One of `-go.out`, `-js.out`, `-ts.out` is required | Output directory for generated TypeScript files. | none |

Positional args: one or more `.proto` files to generate.

> [!IMPORTANT]
> Generated code relies on `google.golang.org/protobuf/encoding/protowire` for Go and `protobufjs/minimal` for JavaScript/TypeScript. You must add these dependencies to your project.

### Native type support

`cleanproto` provides options so you can direct it to generate more natural native types for certain field types. This doesn't change the on-wire byte representation, but conversion to the native type gets baked into the generated decode/encode functions. For example.

```protobuf
   google.protobuf.Timestamp timestamp = 1 [(cp.js_type) = "Date", (cp.go_type) = "time.Time"];
```

This generates models where the `timestamp` field has the type `Date` and `time.Time` in JavaScript and Go respectively.

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
> Native type conversion is standardized and may lose precision when the proto wire type is less precise than the selected native type. For example, if the native JavaScript type is `Date` but the wire type is `int32`, then values are converted to and from epoch seconds to fit `int32` precision. With `int64`, `Date`/`time.Time` values are converted to and from epoch milliseconds.

### Additional options

| Option | Effect |
| --- | --- |
| `cp.go_encode = false` | Keep the field in generated Go models, but skip writing it during Go encoding. |
| `cp.go_ignore = true` | Omit the field completely from generated Go models and their encode/decoding. |
| `cp.js_encode = false` | Keep the field in generated JavaScript models, but skip writing it during JS encoding. |
| `cp.js_ignore = true` | Omit the field completely from generated JavaScript models and their encode/decoding. |
| `cp.ts_encode = false` | Keep the field in generated TypeScript models, but skip writing it during TS encoding. |
| `cp.ts_ignore = true` | Omit the field completely from generated TypeScript models and their encode/decoding. |
| `cp.json_ignore = true` | Keep the field in generated Go models but force a `json:"-"` struct tag so it is omitted by JSON marshalling. |



## Example (models only)

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
  google.protobuf.Duration timeout = 2 [(cp.go_type) = "time.Duration", (cp.js_type) = "bigint", (cp.ts_type) = "number"];
  bytes request_id = 3 [(cp.go_type) = "github.com/google/uuid.UUID"];
  int64 actor_id = 4 [(cp.js_type) = "bigint"];
  google.protobuf.Timestamp synced_at = 5 [(cp.js_type) = "number"];
}
```

Generate:

```bash
cleanproto \
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

```

</details>

## Example (server stubs)

`cleanproto` also generates Go server stubs and JavaScript client calling code when your proto includes a `service`.

Proto (`example/library.proto`):

```proto
syntax = "proto3";

package example;

import "options.proto";

option go_package = "example";

message Book {
  string id = 1;
  string title = 2;
  string author = 3;
}

message Library {
  string id = 1;
  string name = 2;
  repeated Book books = 3;
}

message GetBookReq {
  string id = 1;
}

message CheckoutBookReq {
  string library_id = 1;
  string book_id = 2;
}

service LibraryService {
  rpc GetLibraryV1(cp.Empty) returns (Library);
  rpc GetLibraryBookV1(GetBookReq) returns (Book);
  rpc PostLibraryBook_CheckoutV1(CheckoutBookReq) returns (cp.Empty);
}
```

- HTTP verb mapping comes from the RPC name prefix: `Get`, `Post`, `Put`, `Patch`, and `Delete` map to `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
- A trailing `V1` becomes the `/v1` URL prefix.
- The first CamelCase segment after the verb becomes slash-separated path parts, so `GetLibraryBookV1` maps to `/v1/library/book`.
- Additional `_`-separated suffix segments become kebab-case suffixes, so `PostLibraryBook_CheckoutV1` maps to `/v1/library/book-checkout`.

Generate:

```bash
cleanproto \
  -go.out ./gen/go \
  -js.out ./gen/js \
  example/library.proto
```

This writes `model.gen.go`, `mux.gen.go`, `mux_util.gen.go`, and `util.gen.go` for Go, plus `model.js` and `capi.js` for JavaScript. The checked-in example outputs in this repo live under `example/testdata/gen`.

<details>
<summary>Show Go mux output</summary>

```go
package example

import (
    "context"
    "net/http"
)

type HandlerFunc = func(context.Context, http.ResponseWriter, *http.Request)
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type VerifyAuthFunc func(context.Context, *http.Request, AccessPolicy) (context.Context, error)

type MuxOptions struct {
    MaxRequestBodySize *int
}

func ApplyMiddlewares(h HandlerFunc, middlewares ...MiddlewareFunc) http.HandlerFunc {
    for _, m := range middlewares {
        h = m(h)
    }
    return func(w http.ResponseWriter, r *http.Request) {
        h(r.Context(), w, r)
    }
}

type ServerHandler interface {
    GetLibraryV1(context.Context) (*Library, error)
    GetLibraryBookV1(context.Context, *GetBookReq) (*Book, error)
    PostLibraryBookCheckoutV1(context.Context, *CheckoutBookReq) error
}

func CreateMux(h ServerHandler, verifyAuth VerifyAuthFunc, options *MuxOptions, middlewares ...MiddlewareFunc) *http.ServeMux {
    if verifyAuth == nil {
        verifyAuth = func(ctx context.Context, _ *http.Request, _ AccessPolicy) (context.Context, error) {
            return ctx, nil
        }
    }
    if options == nil {
        options = &MuxOptions{}
    }
    m := http.NewServeMux()
    m.HandleFunc("GET /v1/library", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
        ctx, err := verifyAuth(ctx, r, AccessPolicy{})
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        res, err := h.GetLibraryV1(ctx)
        Respond(ctx, r, w, res, err)
    }, middlewares...))
    m.HandleFunc("GET /v1/library/book", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
        ctx, err := verifyAuth(ctx, r, AccessPolicy{})
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        req, err := decodeWithMaxBodySize(r, options.MaxRequestBodySize, DecodeGetBookReq)
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        res, err := h.GetLibraryBookV1(ctx, req)
        Respond(ctx, r, w, res, err)
    }, middlewares...))
    m.HandleFunc("POST /v1/library/book-checkout", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
        ctx, err := verifyAuth(ctx, r, AccessPolicy{})
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        req, err := decodeWithMaxBodySize(r, options.MaxRequestBodySize, DecodeCheckoutBookReq)
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        err = h.PostLibraryBookCheckoutV1(ctx, req)
        if err != nil {
            HandleReqErr(ctx, err, r, w)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }, middlewares...))
    return m
}
```

</details>

- To use the Go stubs, you just implement the generated handler interface and pass it to `CreateMux`.
- A minimal `main.go` can look like this:

```go
package main

import (
    "context"
    "log"
    "net/http"

    "your/module/gen/go/example"
)

type server struct{}

func (server) GetLibraryV1(context.Context) (*example.Library, error) {
    return &example.Library{Name: "City Library"}, nil
}

func (server) GetLibraryBookV1(_ context.Context, req *example.GetBookReq) (*example.Book, error) {
    return &example.Book{ID: req.ID, Title: "Example Book", Author: "A. Writer"}, nil
}

func (server) PostLibraryBookCheckoutV1(context.Context, *example.CheckoutBookReq) error {
    return nil
}

func main() {
    mux := example.CreateMux(server{}, nil, nil)
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

- `cp.Empty` from `options.proto` is the special empty message type; when used as a request or response it becomes no request body or no response body in the generated server/client surface.
- `cp.go_custom = true` switches a Go handler into custom mode so the generated interface method receives `*http.Request` and `http.ResponseWriter` directly, while still optionally decoding the protobuf request first.

<details>
<summary>Show JavaScript output</summary>

```js
// model.js

/**
 * @typedef {Object} Book
 * @property {string} id
 * @property {string} title
 * @property {string} author
 */
/**
 * @typedef {Object} Library
 * @property {string} id
 * @property {string} name
 * @property {Book[]} books
 */
/**
 * @typedef {Object} GetBookReq
 * @property {string} id
 */
/**
 * @typedef {Object} CheckoutBookReq
 * @property {string} libraryId
 * @property {string} bookId
 */
import protobufjsm from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

export function encodeGetBookReq(message) {
    const writer = Writer.create();
    writeGetBookReq(message, writer);
    return writer.finish();
}

export function decodeBook(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeBookMessage(reader);
}

export function decodeLibrary(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeLibraryMessage(reader);
}

export function encodeCheckoutBookReq(message) {
    const writer = Writer.create();
    writeCheckoutBookReq(message, writer);
    return writer.finish();
}

// capi.js

import {
  decodeBook,
  decodeLibrary,
  encodeCheckoutBookReq,
  encodeGetBookReq,
} from './model.js';

export class Capi {
  constructor(baseURL = '', headerProvider = null, errorHandler = null) {
    this.baseURL = baseURL;
    this.headerProvider = headerProvider == null ? () => ({}) : headerProvider;
    this.errorHandler = errorHandler == null ? async (response) => { throw new Error(`HTTP ${response.status}`); } : errorHandler;
  }

  async #request(path, { method = 'GET', body } = {}) {
    const headers = this.headerProvider() || {};
    headers['Accept'] = 'application/x-protobuf';
    if (body !== undefined) {
      headers['Content-Type'] = 'application/x-protobuf';
    }
    return fetch(`${this.baseURL}${path}`, { method, headers, body, credentials: 'include' });
  }

  async getLibraryV1() {
    const response = await this.#request('/library/v1', { method: 'GET' });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeLibrary(await response.arrayBuffer());
  }

  async getLibraryBookV1(payload) {
    const response = await this.#request('/library/book/v1', { method: 'GET', body: encodeGetBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeBook(await response.arrayBuffer());
  }

  async postLibraryBook_CheckoutV1(payload) {
    const response = await this.#request('/library/book-checkout-v1', { method: 'POST', body: encodeCheckoutBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    await response.arrayBuffer();
  }
}
```

</details>

## Notes
- Unknown fields are ignored on decode.
- `oneof` not supported.
- `cp.<lang>_ignore = true` takes precedence over `cp.<lang>_encode = false` for that language, since ignored fields are omitted entirely.

## Todo
- Add options to indicate server or client side like `cp.go_server_only`.
- Only generate `Decode`/`Encode` methods actually used by a side. For example, a server does not need to decode its response types, only encode them.
- Update generated client calling code so it only adds auth headers in alignment with access policies when defined.
- In the utils code added to support generated code, strip out any helper functions that are not actually used.
