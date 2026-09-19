# Go presence and choices

This is a breaking change to the **generated Go API**. Regenerate a package as a unit and update its callers. Ordinary protobuf tags and message nesting stay unchanged. There is no automatic wrapper flattening, including for messages containing only a oneof.

## Field mapping

| Proto declaration | Generated Go field |
| --- | --- |
| `int32 count = 1` | `Count int32` |
| `optional int32 count = 1` | `Count Maybe[int32]` |
| `optional bytes data = 1` | `Data Maybe[[]byte]` |
| `Book book = 1` | `Book Maybe[Book]` |
| `Book book = 1 [(cp.go_value) = true]` | `Book Book` |
| `repeated Book books = 1` | `Books []Book` |
| `map<string, Book> books = 1` | `Books map[string]Book` |
| `google.protobuf.Timestamp updated = 1` | `Updated Maybe[time.Time]` |
| `google.protobuf.Duration delay = 1` | `Delay Maybe[time.Duration]` |

A protobuf singular message has presence even without the word `optional`. Ordinary proto3 scalars and collections do not. Scalars without explicit presence remain values; a zero number, `false`, or an empty collection is valid unless a separate constraint excludes it.

`Maybe[T]` exposes its data directly. Its zero value is absent:

```go
type Maybe[T any] struct {
    Value   T
    Present bool
}

m.Count = Maybe[int32]{Value: 0, Present: true} // Present zero.
if m.Count.Present {
    fmt.Println(m.Count.Value)
}
m.Count.Present = false // Value is ignored while absent.
m.Count = Maybe[int32]{} // Or clear both fields.
```

There are no constructors, getters, or setters. `Maybe` is emitted once per generated package in `maybe.gen.go`, maintaining standalone output without a cleanproto runtime dependency. Independently generated packages have their own named `Maybe` type; construct it directly using `Value` and `Present`. The `IsZero` method remains for `encoding/json`'s `omitzero` integration. Generated Go JSON uses `omitzero`, requiring Go 1.24 or newer; the generator itself uses the version in `go.mod`.

### Storage and recursion

Replace `[]*Book` literals with `[]Book`, and map message values with `Book`. Explicit `(cp.go_slice_ptr) = true` retains pointer elements when needed for storage; `false` now restates the default. Nil elements in an explicitly requested pointer slice still encode as empty messages.

Singular message cycles are boxed where necessary: `message Node { Node next = 1; }` generates `Next Maybe[*Node]`. Absence still comes from `Maybe`; `Maybe[*Node]{Present: true}` is invalid. A cycle made entirely of value fields also requires pointer storage. `Validate` and `EncodeChecked` reject cyclic **object graphs**; protobuf represents finite trees, not shared object identities. Slices, maps, and oneof pointers already provide indirection. Value fields do not make their slices/maps immutable or deep-copy them.

`cp.go_value` explicitly opts a singular message out of presence tracking: absent input materializes its zero value, and even an empty value is now emitted as a present empty message. It does **not** validate that a field occurred in the original bytes. For required wire presence, use `(buf.validate.field).required = true` on a presence-bearing field; it retains `Maybe` and validation rejects absence. For nonempty strings/collections or nonzero numbers, use the corresponding length, item-count, or numeric constraint. `required` alone no longer rejects zero-valued implicit-presence scalars or empty collections.

## Oneofs

```proto
message CertSource {
  oneof value {
    AcmeCertSource acme = 1;
    SecretCertSource secret = 2;
  }
}
```

Generates:

```go
type CertSource struct {
    Value Maybe[CertSourceValueOneof]
    // Private unknown-field storage omitted here.
}
type CertSourceValueOneof struct {
    Acme   *AcmeCertSource
    Secret *SecretCertSource
}
```

The message and its group are **always separate types**, whether or not the message has other fields. Scalar, enum, bytes, and native alternatives also use pointers. Exactly one non-nil pointer selects a valid choice; there is no stored discriminator.

```go
source := CertSource{
    Value: Maybe[CertSourceValueOneof]{
        Present: true,
        Value: CertSourceValueOneof{
            Acme: &AcmeCertSource{Email: "ops@example.com"},
        },
    },
}
if source.Value.Present && source.Value.Value.Acme != nil {
    fmt.Println(source.Value.Value.Acme.Email)
}
```

There is no `Kind` method or generated kind enum, and no choice constructors or accessors. Inspect the pointer fields directly. A choice's `Validate` counts non-nil alternatives and requires exactly one; its containing message's `Validate` additionally checks constraints throughout the selected payload.

An ordinary protobuf oneof may be unset: `Maybe[CertSourceValueOneof]{}` is valid. `Maybe[CertSourceValueOneof]{Present: true}` is invalid because its choice has no selected alternative. When `Present` is false, any leftover `Value` is ignored during validation, encoding, and JSON serialization. To make the group mandatory:

```proto
import "buf/validate/validate.proto";

message CertSource {
  oneof value {
    option (buf.validate.oneof).required = true;
    AcmeCertSource acme = 1;
    SecretCertSource secret = 2;
  }
}
```

Now `CertSource.Value` is `CertSourceValueOneof` directly. The empty message is decodable, but fails validation because no alternative is selected. A parent `CertSource cert_source = 7` remains `Maybe[CertSource]`: absent parent is distinct from present-but-invalid empty wrapper.

The group's alternatives keep their parent message's protobuf tag namespace. A DSL with separate field and alternative tag scopes must still emit a wrapper message. There is no flattening heuristic or DSL-specific inference in cleanproto.

### Optional payloads and collections

Protobuf has no directly optional oneof alternative, repeated/map alternative, optional collection, or optional list element. Represent these with explicit wrapper messages. For example, selecting `NullableBook { Book value = 1; }` distinguishes no choice from a selected payload whose `Value` is absent. The wrapper stays a Go struct; it is not implicitly converted into `Maybe[Book]`. Similarly, use a message containing a repeated field for an optional list or list-valued choice. Nested Go `Maybe` forms are not inferred from arbitrary wrapper shapes.

## Encoding, decoding, validation and JSON

- Every generated message has `Validate() error` and `EncodeChecked() ([]byte, error)`. Use checked encoding at input boundaries. `Encode() []byte` remains as a convenience and **panics** on invalid data, rather than silently selecting one branch.
- `DecodeX` parses and merges protobuf data; it does not reject missing required domain fields. Call `Validate` afterwards. Generated server handlers validate inputs, and generated clients/servers propagate encoding failures through their error paths.
- Different alternatives use protobuf's last-one-wins rule. Repeated occurrences of the same message alternative merge, including native Timestamp/Duration messages. Present scalar zero and empty message/bytes alternatives are emitted.
- Unknown fields are retained in ordinary generated messages and re-emitted. `UnknownFields()` returns a copy. Fields explicitly marked `cp.go_ignore` are discarded instead. Unknown fields inside native Timestamp/Duration conversions and map-entry envelopes are not retained.
- Unknown numeric enum values survive by default. `(buf.validate.field).enum.defined_only = true` opts into rejecting them.
- An unknown field cannot safely be assigned to an arbitrary oneof. Cleanproto preserves its bytes without guessing a selection. A required group containing only a future, unknown alternative therefore fails validation. Unknown bytes are appended after known fields on re-encoding; ordering relative to known fields is not preserved. This matters if a newer schema interprets an unknown tag as a competing oneof alternative.
- `Maybe` and choice JSON marshalers validate generated payloads before encoding, including detecting cyclic references.
- Go JSON uses ordinary `encoding/json`, **not ProtoJSON**. Absent `Maybe` fields are omitted with snake tags; JSON `null` means absent. Present zero values are included. Present nil slices/maps are normalized to empty collections, including a selected bytes alternative. Arbitrary nested `Maybe`/nullable-pointer JSON values need an explicit wrapper to preserve distinct null states.
- Audit projections preserve optionality and choices while removing marked fields. Unknown wire bytes are private and excluded from JSON and audit output.
- Native numeric time conversions retain their existing units/range/precision limits. A present zero `time.Time` in a protobuf Timestamp is now preserved as year 1, rather than changing to the Unix epoch.

## Generation boundaries

Go supports the new oneof mapping. JS and TS retain their existing field representations and **explicitly reject oneof schemas** until their own mappings are implemented. The CLI generates all requested outputs in memory before writing, so a rejected target does not leave a partial generation.

Supply related input protos together when generating one Go package. Inputs with the same Go package are combined into one set of output files; different Go packages sharing an output directory are rejected. Cross-package Go import generation is not added by this change. Conflicting generated type/helper/member names produce errors instead of invalid or overwritten code.

Unsupported CEL/law expressions are still outside this iteration; existing warnings remain. This change adds no DSL-to-protobuf generator, database mapping, API redesign, or automatic wrapper flattening.

See [`example/choices.proto`](example/choices.proto) for the HTTPS certificate example and coverage of native/scalar choices, recursion, and audit projections. Generate it with:

```sh
go run ./cmd/cleanproto -proto_path example -proto_path . \
  -go.out .build/choices -go.jsontags snake choices.proto
```
