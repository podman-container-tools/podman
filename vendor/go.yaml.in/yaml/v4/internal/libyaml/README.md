# internal/libyaml

This package implements the YAML processing pipeline used by the public
`go.yaml.in/yaml/v4` API.
It includes the Go port of libyaml's low-level machinery and the stages that
convert between YAML nodes and Go values.
Applications should import the public package rather than this internal one.

## Processing Stages

| Operation | Stages | Source files |
|---|---|---|
| Load text | Reader, scanner, parser | `reader.go`, `scanner.go`, `parser.go` |
| Build nodes | Composer, resolver | `composer.go`, `resolver.go` |
| Build Go values | Constructor | `constructor.go`, `plugin_merge_key.go` |
| Build output nodes | Representer, desolver | `representer.go`, `desolver.go` |
| Write text | Serializer, emitter, writer | `serializer.go`, `emitter.go`, `writer.go` |

`loader.go` and `dumper.go` coordinate these stages.
`node.go` defines nodes and their conversion methods.
`yaml.go` defines token, event, encoding, style, and mark types.
`options.go` supplies options and default safety checks.
`errors.go` defines structured errors.

The reader, scanner, and parser share the `Parser` state.
`Parser.Parse` pulls tokens from the scanner and returns events.
`Composer.Compose` builds node graphs, including anchor and alias edges.
The resolver infers scalar tags; the constructor converts nodes into the
requested destination types.
On output, the serializer pushes events to the emitter, which writes formatted
YAML through the writer.

See [go-yaml Internals](../../docs/dev/go-yaml-internals.md) for control flow,
comments, safety limits, and plugin boundaries.

## Test Layout

Stage tests live in `*_test.go`, with YAML fixtures in `testdata/`.
Examples include `scanner.yaml`, `parser.yaml`, `composer.yaml`,
`constructor.yaml`, `representer.yaml`, `desolver.yaml`, `serializer.yaml`,
`emitter.yaml`, `reader.yaml`, and `writer.yaml`.
Not every test file has a fixture; several tests construct their inputs in Go.

The shared data-driven framework is in `internal/testutil/datatest`.
This package's `testdata_test.go` supplies `TestCase`, constant and event
converters, and `LoadTestCases` for its fixtures.
Fixture YAML is loaded through `LoadAny`, so ordinary scalar resolution applies
before conversion to test case structs.
Quote fixture strings when they would otherwise resolve to another type.

Test cases use the test type as a mapping key:

```yaml
- scan-tokens:
    name: Simple scalar
    yaml: hello
    want:
    - STREAM_START_TOKEN
    - SCALAR_TOKEN
    - STREAM_END_TOKEN
```

A parser case can inspect event boundaries:

```yaml
- parse-events:
    name: Simple mapping
    yaml: |
      key: value
    want:
    - STREAM_START_EVENT
    - DOCUMENT_START_EVENT
    - MAPPING_START_EVENT
    - SCALAR_EVENT
    - SCALAR_EVENT
    - MAPPING_END_EVENT
    - DOCUMENT_END_EVENT
    - STREAM_END_EVENT
```

`name`, `yaml`, and `want` are common fixture fields.
Emitter tests use `data` for event specifications and `conf` for emitter
configuration.
Detailed scanner and parser tests can specify token or event properties such
as value and style.
Use the corresponding test handler and nearby fixtures as the source of truth
for a test type's accepted fields.
There is no separate scalar-resolution algorithm for test data.

## Running Tests

Run these commands from the repository root:

```bash
make test-internal
make shell CMD='go test ./internal/libyaml -run TestScanner'
make shell CMD='go test ./internal/libyaml -run TestParser'
make shell CMD='go test ./internal/libyaml -run TestEmitter'
make shell CMD='go test -v ./internal/libyaml -run TestScanner/Simple_scalar'
make shell CMD='go test -cover ./internal/libyaml'
```

The test-only module under `testdata/v3hooks` checks real v3 unmarshaler types
without adding those modules to the core dependency graph.
Run it with `make test-v3-hooks`.
