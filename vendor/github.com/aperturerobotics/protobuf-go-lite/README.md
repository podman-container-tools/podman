# protobuf-go-lite

[![GoDoc Widget]][GoDoc] [![DeepWiki Widget]][DeepWiki]

[GoDoc]: https://godoc.org/github.com/aperturerobotics/protobuf-go-lite
[GoDoc Widget]: https://godoc.org/github.com/aperturerobotics/protobuf-go-lite?status.svg
[DeepWiki Widget]: https://img.shields.io/badge/DeepWiki-aperturerobotics%2Fprotobuf--go--lite-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==
[DeepWiki]: https://deepwiki.com/aperturerobotics/protobuf-go-lite

**protobuf-go-lite** is a stripped-down version of the [protobuf-go] code
generator modified to work without reflection and merged with [vtprotobuf] to
provide modular features with static code generation for marshal/unmarshal,
size, clone, equal, text, and JSON. JSON support is derived from a fork of
[protoc-gen-go-json].

[protobuf-go]: https://github.com/protocolbuffers/protobuf-go
[vtprotobuf]: https://github.com/planetscale/vtprotobuf
[protoc-gen-go-json]: https://github.com/TheThingsIndustries/protoc-gen-go-json

Static code generation without reflection is more efficient at runtime and
results in smaller code binaries. It also provides better support for [tinygo]
which has limited reflection support.

[tinygo]: https://github.com/tinygo-org/tinygo

protobuf-go-lite supports Edition 2024 schemas that resolve to the open Go API
and static, reflect-free generated output. The default `features=all` path
supports explicit and implicit presence, legacy required fields, packed
encoding, delimited message encoding, oneofs, maps, clone/equal, text,
unmarshal, unsafe unmarshal, and JSON when the resolved JSON format is
`ALLOW`.

Generated output is static Go code. The default `codegen=helper` mode emits
message methods that call small concrete helpers from the root
`protobuf-go-lite` runtime package to keep generated `.pb.go` files smaller.
The fallback `codegen=unrolled` mode keeps the older inline method-body shape
for helper-converted method families when callers need to inspect or compare
that output. Neither mode relies on Go reflection, descriptors, struct tags, or
runtime type metadata for generated marshal, unmarshal, size, clone, equal,
text, or JSON behavior.

protobuf-go-lite rejects Edition schemas that require closed enum semantics,
`LEGACY_BEST_EFFORT` JSON, or explicit hybrid/opaque Go APIs. It does not
support fieldmasks and extensions.

### Ecosystem

Lightweight Protobuf 3 RPCs are implemented in [StaRPC] for Go and TypeScript.

[StaRPC]: https://github.com/aperturerobotics/starpc

[protoc-gen-doc] is recommended for generating documentation.

[protoc-gen-doc]: https://github.com/pseudomuto/protoc-gen-doc

[protobuf-es-lite] is recommended for lightweight TypeScript protobufs.

[protobuf-es-lite]: https://github.com/aperturerobotics/protobuf-es-lite

## Protobuf

[protocol buffers](https://protobuf.dev) are a cross-platform cross-language
message serialization format. Protobuf is a language for specifying the schema
for structured data. This schema is compiled into language specific bindings.
This project provides both a tool to generate Go code for the protocol buffer
language, and also the runtime implementation to handle serialization of
messages in Go.

See the [protocol buffer developer guide](https://protobuf.dev/overview) for
more information about protocol buffers themselves.

## Example

See the [protobuf-project](https://github.com/aperturerobotics/protobuf-project)
template for an example of how to use this package and vtprotobuf together with
protowrap to generate protobufs for your project.

This package is available at **github.com/aperturerobotics/protobuf-go-lite**.

## Package index

Summary of the packages provided by this module:

*   [`compiler/protogen`](https://pkg.go.dev/github.com/aperturerobotics/protobuf-go-lite/compiler/protogen):
    Package `protogen` provides support for writing protoc plugins.
*   [`cmd/protoc-gen-go-lite`](https://pkg.go.dev/github.com/aperturerobotics/protobuf-go-lite/cmd/protoc-gen-go-lite):
    The `protoc-gen-go-lite` binary is a protoc plugin to generate a Go protocol
    buffer package.
*   [`registry`](https://pkg.go.dev/github.com/aperturerobotics/protobuf-go-lite/registry):
    Package `registry` indexes generated message constructors and custom
    message options without runtime reflection.

## Usage

1. Install `protoc-gen-go-lite`:

    ```
    go install github.com/aperturerobotics/protobuf-go-lite/cmd/protoc-gen-go-lite@latest
    ```

2. Update your `protoc` generator to use the new plug-in.

    ```
    for name in $(PROTO_SRC_NAMES); do \
        protoc \
          --plugin protoc-gen-go-lite="${GOBIN}/protoc-gen-go-lite" \
          --go-lite_out=.  \
          --go-lite_opt=features=marshal+unmarshal+size+equal+clone+text \
        proto/$${name}.proto; \
    done
    ```

`protobuf-go-lite` replaces `protoc-gen-go` and `protoc-gen-go-vtprotobuf` and should not be used with those generators.

Check out the [template](https://github.com/aperturerobotics/template) for a quick start!

### Code generation modes

`protoc-gen-go-lite` accepts `codegen=helper` and `codegen=unrolled`:

- `codegen=helper` is the default. It emits static message methods that call
  concrete runtime helpers such as encode, decode, clone/equal, size, and text
  helpers.
- `codegen=unrolled` emits the older inline method-body style for
  helper-converted method families. Select it by adding `codegen=unrolled` to
  `--go-lite_opt`.

Both modes are selected at generation time and produce normal Go packages for
callers. They do not add a reflection registry, descriptor builder, struct tag
interpreter, or runtime type-metadata dependency to generated fast paths.

### Opt-in message registry

Generated packages do not register message types by default. Add `registry=true`
to `--go-lite_opt` when an application must discover linked message types at
runtime:

```
protoc --go-lite_out=. --go-lite_opt=features=all,registry=true example.proto
```

The registry requires the `size`, `marshal`, and `unmarshal` features. Each
generated package registers constructors by fully qualified protobuf name and
`google.protobuf.Any` type URL during package initialization. The generator
also flattens custom message options into fully qualified scalar paths:

```go
registry.Range(func(entry registry.Entry) bool {
	tenant, tenantOK := entry.Option("annotations.msg_opts.tenant")
	table, tableOK := entry.Option("annotations.msg_opts.table")
	if tenantOK && tableOK {
		messagesByKey[tenant+"."+table] = entry
	}
	return true
})
```

Repeated option values retain declaration order. Message-valued options use
dot-separated field paths, and enum values use their protobuf names. The
registry copies option metadata on registration and returns sorted snapshots to
callers.

### Generated output

Generated `.pb.go` files are checked in for this repository's fixtures and
well-known type packages. After changing the generator or runtime helpers,
regenerate and verify them with the GNU Make targets:

```
gmake gengo
gmake check-gengo
```

On systems where `make` is GNU Make, `make check-gengo` is equivalent. On macOS,
use `gmake` so the same GNU Make behavior is used locally and in Linux CI.

## Available features

The following additional features can be enabled:

- `size`: generates a `func (p *YourProto) SizeVT() int` helper that behaves identically to calling `proto.Size(p)` on the message, except the size calculation is static generated code and does not use reflection. This helper function can be used directly, and it'll also be used by the `marshal` codegen to ensure the destination buffer is properly sized before ProtoBuf objects are marshalled to it.

- `equal`: generates the following helper methods

    - `func (this *YourProto) EqualVT(that *YourProto) bool`: this function behaves almost identically to calling `proto.Equal(this, that)` on messages, except the equality calculation is static generated code and does not use reflection. This helper function can be used directly.

    - `func (this *YourProto) EqualMessageVT(thatMsg any) bool`: this function behaves like the above `this.EqualVT(that)`, but allows comparing against arbitrary proto messages. If `thatMsg` is not of type `*YourProto`, false is returned. The uniform signature provided by this method allows accessing this method via type assertions even if the message type is not known at compile time. This allows implementing a generic `func EqualVT(proto.Message, proto.Message) bool` without reflection.

- `marshal`: generates the following helper methods

    - `func (p *YourProto) MarshalVT() ([]byte, error)`: this function behaves identically to calling `proto.Marshal(p)`, except the actual marshalling is static generated code and does not use reflection or allocate memory. This function simply allocates a properly sized buffer by calling `SizeVT` on the message and then uses `MarshalToSizedBufferVT` to marshal to it.

    - `func (p *YourProto) MarshalToVT(data []byte) (int, error)`: this function can be used to marshal a message to an existing buffer. The buffer must be large enough to hold the marshalled message, otherwise this function will panic. It returns the number of bytes marshalled. This function is useful e.g. when using memory pooling to re-use serialization buffers.

    - `func (p *YourProto) MarshalToSizedBufferVT(data []byte) (int, error)`: this function behaves like `MarshalTo` but expects that the input buffer has the exact size required to hold the message, otherwise it will panic.

- `marshal_strict`: generates the following helper methods

    - `func (p *YourProto) MarshalVTStrict() ([]byte, error)`: this function behaves like `MarshalVT`, except fields are marshalled in a strict order by field's numbers they were declared in .proto file.

    - `func (p *YourProto) MarshalToVTStrict(data []byte) (int, error)`: this function behaves like `MarshalToVT`, except fields are marshalled in a strict order by field's numbers they were declared in .proto file.

    - `func (p *YourProto) MarshalToSizedBufferVTStrict(data []byte) (int, error)`: this function behaves like `MarshalToSizedBufferVT`, except fields are marshalled in a strict order by field's numbers they were declared in .proto file.


- `unmarshal`: generates a `func (p *YourProto) UnmarshalVT(data []byte)` that behaves similarly to calling `proto.Unmarshal(data, p)` on the message, except the unmarshalling is performed by static generated code without using reflection and allocating as little memory as possible. If the receiver `p` is **not** fully zeroed-out, the unmarshal call will actually behave like `proto.Merge(data, p)`. This is because the `proto.Unmarshal` in the ProtoBuf API is implemented by resetting the destination message and then calling `proto.Merge` on it. To ensure proper `Unmarshal` semantics, ensure you've called `proto.Reset` on your message before calling `UnmarshalVT`, or that your message has been newly allocated.

- `unmarshal_unsafe` generates a `func (p *YourProto) UnmarshalVTUnsafe(data []byte)` that behaves like `UnmarshalVT`, except it unsafely casts slices of data to `bytes` and `string` fields instead of copying them to newly allocated arrays, so that it performs less allocations. **Data received from the wire has to be left untouched for the lifetime of the message.** Otherwise, the message's `bytes` and `string` fields can be corrupted.

- `clone`: generates the following helper methods

    - `func (p *YourProto) CloneVT() *YourProto`: this function behaves similarly to calling `proto.Clone(p)` on the message, except the cloning is performed by static generated code without using reflection. If the receiver `p` is `nil` a typed `nil` is returned.

    - `func (p *YourProto) CloneMessageVT() any`: this function behaves like the above `p.CloneVT()`, but provides a uniform signature in order to be accessible via type assertions even if the type is not known at compile time. This allows implementing a generic `func CloneMessageVT() any` without reflection. If the receiver `p` is `nil`, a typed `nil` pointer of the message type will be returned inside a `any` interface.

- `json`: generates the following helper methods

    - `func (p *YourProto) UnmarshalJSON(data []byte) error` behaves similarly to calling `protojson.Unmarshal(data, p)` on the message, except the unmarshalling is performed by static generated code without using reflection and allocating as little memory as possible. If the receiver `p` is **not** fully zeroed-out, the unmarshal call will actually behave like `proto.Merge(data, p)`. To ensure proper `Unmarshal` semantics, ensure you've called `proto.Reset` on your message before calling `UnmarshalJSON`, or that your message has been newly allocated.

    - `func (p *YourProto) UnmarshalJSONValue(val *fastjson.Value) error` unmarshals a `*fastjson.Value`.

    - `func (p *YourProto) MarshalJSON() ([]byte, error)` behaves similarly to calling `protojson.Marshal(p)` on the message, except the marshalling is performed by static generated code without using reflection and allocating as little memory as possible.

    - Adding a `//protobuf-go-lite:disable-json` comment before a message or enum will disable the json marshaler / unmarshaler.

- `text`: generates `MarshalProtoText() string` and `String() string` methods
  that emit protobuf text-format-style output using static generated code
  without reflection. Adding a `//protobuf-go-lite:disable-text` comment before
  a message disables text generation for that message.

## License

BSD-3
