# go-yaml v4

**YAML for Go, with the control your application needs.**

**v4.0.0 is the stable release.**

Read YAML into Go values, work with nodes and comments, and write YAML with
explicit control over formatting and behavior.
Use the same processing pipeline from Go or from the `go-yaml` command.
Maintained by the [YAML organization](https://github.com/yaml).

[Get started](#get-started) · [CLI](#the-go-yaml-cli) ·
[Options](#options-that-fit-your-application) ·
[Plugins](#plugins-extend-go-yamls-capabilities) ·
[Documentation](#documentation) · [Migration](docs/v3-to-v4-migration.md)

## Choose your version

**v2 and v3 remain stable.**
Their APIs are frozen, and their branches receive security fixes only.
New features and routine bug fixes are developed in v4.

| Version | Import path | Status |
|---|---|---|
| [v2](https://github.com/yaml/go-yaml/tree/v2) | `go.yaml.in/yaml/v2` | Stable, frozen API; security fixes only |
| [v3](https://github.com/yaml/go-yaml/tree/v3) | `go.yaml.in/yaml/v3` | Stable, frozen API; security fixes only |
| [v4](https://github.com/yaml/go-yaml/tree/main) | `go.yaml.in/yaml/v4` | Stable; active development |

**v4 is designed to be highly compatible with v3.**
The classic `Unmarshal`, `Marshal`, `NewDecoder`, and `NewEncoder` entry points
remain available.
The new `Load` and `Dump` APIs add options, plugins, and clearer configuration.
Version presets restore v3 defaults, with individual options for finer control.
See the [migration guide](docs/v3-to-v4-migration.md) for formatting, error
handling, and custom unmarshaler details before upgrading.

## Get started

```bash
go get go.yaml.in/yaml/v4
```

Load into structs, maps, slices, or your own types.
Export struct fields and use `yaml` tags to control their names:

```go
package main

import (
    "fmt"
    "log"

    "go.yaml.in/yaml/v4"
)

type Config struct {
    Name string `yaml:"name"`
    Port int    `yaml:"port"`
}

func main() {
    input := []byte("name: api\nport: 8080\n")
    var config Config
    if err := yaml.Load(input, &config, yaml.WithKnownFields()); err != nil {
        log.Fatal(err)
    }

    output, err := yaml.Dump(config, yaml.WithIndent(4))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(string(output))
}
```

Output:

```yaml
name: api
port: 8080
```

Use `NewLoader` and `NewDumper` for streams and `yaml.Node` when you need
access to tags, styles, anchors, comments, or source positions.
Both constructors return an error; check it before using the stream.
See the [runnable examples](example/README.md) and
[API reference](https://pkg.go.dev/go.yaml.in/yaml/v4).

## The go-yaml CLI

**See what YAML is doing at every stage.**
The CLI exposes the library's scanner, parser, node tree, and output formats.
Use it to inspect configuration, explore options, compare behavior, and make
bug reports reproducible.

```bash
go install go.yaml.in/yaml/v4/cmd/go-yaml@latest
```

![CLI demo: JSON conversion, nodes, options, and YAML pipelines.][cli-demo]

[cli-demo]: docs/demo/cli.gif

Use this `config.yaml`:

```yaml
# Service configuration
service:
  name: api
  port: 8080
```

Try the same commands with your own file:

```bash
go-yaml -J config.yaml
go-yaml -n config.yaml
go-yaml -Y -o indent=4 config.yaml
go-yaml -t config.yaml | go-yaml -e | go-yaml -N | go-yaml -Y
```

| Output | Flags | What you can inspect |
|---|---|---|
| YAML | `-y` / `-Y` | Ordinary encoding / node-aware styles and comments |
| JSON | `-j` / `-J` | Compact / pretty-printed JSON |
| Tokens | `-t` / `-T` | Scanner tokens / tokens with source positions |
| Events | `-e` / `-E` | Parser events / events with source positions |
| Nodes | `-n` / `-N` | Compact tree / tags and styles included |

Select an output mode and pass a file, or pipe input on standard input.
Put flags before the file name.
Token, event, and node output can be piped into later stages, as shown above.
Use `-f yaml` to force YAML input when a document resembles an intermediate
stage's representation.
Backward stage conversions are rejected.

The CLI also supports:

- **Options:** `-o indent=4`, repeated `-o` flags, and presets such as `-o v3`.
  Run `go-yaml -o help` to discover available settings.
- **Configuration:** `-C options.yaml` loads a reusable options file.
  Command-line `-o` settings override its individual non-plugin options.
- **Plugins:** `--plugin` selects implementations linked into the executable.
  Build the optional reference parser with `make cli PLUGIN=...`.
  Use `PLUGIN-PACKAGE=package@version` to link other released implementations,
  or `make cli CONFIG=options.yaml` to embed configuration.
  See [plugin builds](cmd/go-yaml/README.md#plugin-builds) for complete commands.
- **Layout:** `-l` selects block layout for stage representations.
- **Classic APIs:** `--unmarshal`, `--decode`, `--marshal`, and `--encode`
  help compare classic library paths with the default Load/Dump pipeline.

Node-aware YAML output retains supported styles and comments; it does not
promise byte-for-byte formatting preservation.
See the [CLI guide](cmd/go-yaml/README.md) for pipelines, plugin builds,
configuration, and processing-mode restrictions.
The [demo source and recording instructions](docs/demo/README.md) are included.

## Options that fit your application

Choose strict loading, indentation, sequence layout, line width, document
markers, Unicode handling, and more without wrapping or forking the library.
Options work with package functions, streaming loaders and dumpers, and node
methods.

```go
err := yaml.Load(input, &config,
    yaml.WithKnownFields(),
    yaml.WithUniqueKeys(),
)
```

`yaml.Load` rejects additional documents by default.
Use `yaml.NewLoader` to process a stream one document at a time.

Start from a version preset and override only what you need.
Options are applied in order:

```go
output, err := yaml.Dump(config,
    yaml.WithV3Defaults(),
    yaml.WithIndent(2),
)
```

`Load` and `Dump` use v4 defaults: two-space indentation and compact sequences.
The classic APIs retain their compatibility settings.
To match `Marshal` formatting when switching to `Dump`, use
`WithV3Defaults()` followed by `WithLineWidth(-1)`.
Presets select defaults; they do not reverse every v4 bug fix or API change.

You can also share configuration between Go code and the CLI:

```yaml
indent: 4
compact-seq-indent: false
line-width: 100
```

In Go, parse that YAML with `yaml.OptsYAML`, check the returned error, and pass
the resulting option to `Load`, `Dump`, or a stream constructor.
On the command line, use `go-yaml -Y -C options.yaml config.yaml`.

See the [options guide](docs/options.md) for the full reference and presets.

## Plugins extend go-yaml's capabilities

**go-yaml's plugin APIs let you adapt YAML processing to your application.**
Use the supplied implementations or implement the public interfaces in your
own Go package.

These are the current plugin APIs:

| Plugin API | Purpose | Supplied implementations |
|---|---|---|
| `loader-limits` | Control nesting and alias limits | Core loader-limits plugin |
| `alias-data` | Control anchor sources and scope | Core alias-data plugin |
| `merge-key` | Control YAML merge keys and mapping conflicts | Core merge-key plugin |
| `error-format` | Choose how structured errors are presented | Core v3 and v4 formatters |
| `yaml-parser` | Replace the parser behind the node pipeline | Built-in Go parser; optional reference parser module |
| `tab-indent` | Enable and configure tab-indented input | Core opt-in tab-indent plugin |

For example, set a nesting-depth limit directly in Go:

```go
import (
    "go.yaml.in/yaml/v4"
    "go.yaml.in/yaml/v4/plugin/loaderlimits"
)

// Inside your loading function:
err := yaml.Load(input, &config,
    yaml.WithPlugin(loaderlimits.New(loaderlimits.DepthValue(50))),
)
```

The core module includes the built-in parser, loader limits, merge-key and
alias-data policies, error formatters, and tab-indentation support.
JSON-comments and reference-parser implementations are separate modules, so
applications acquire those dependencies only when they choose them.

Plugins can be passed directly with `WithPlugin` or registered by name for
YAML configuration.
The CLI selects already-linked implementations at runtime; it does not
download plugins.
See the [plugin guide](docs/plugin.md) for interfaces, registration,
configuration, dependencies, and limitations of each processing path.

## Documentation

| Start here | Learn how to |
|---|---|
| [API reference](https://pkg.go.dev/go.yaml.in/yaml/v4) | Use Go types, functions, and interfaces |
| [v3 to v4 migration](docs/v3-to-v4-migration.md) | Upgrade while preserving compatibility |
| [Options guide](docs/options.md) | Control loading and output |
| [Plugin guide](docs/plugin.md) | Configure or write extensions |
| [CLI guide](cmd/go-yaml/README.md) | Inspect stages and build custom commands |
| [Examples](example/README.md) | Run complete Go programs |
| [Contributor guide](CONTRIBUTING.md) | Build, test, and contribute |

## Compatibility

Custom `UnmarshalYAML(*yaml.Node) error` hooks depend on the exact package
that defines `Node`.
Update those hooks and their imports when moving from v3 to v4.
V4 reports a migration error for recognized v3 node hooks; the legacy
`UnmarshalYAML(func(any) error) error` callback remains supported across
versions.
See the [migration guide](docs/v3-to-v4-migration.md#custom-unmarshalers-across-versions)
for details.

The library supports most of YAML 1.2 and retains selected YAML 1.1 behavior
for compatibility, including legacy octal notation and `yes`/`no` booleans
when loading into typed Go bools.
Consult the API reference and test behavior against your application's data.

## Report bugs and propose ideas

**Please use the CLI when reporting bugs or suggesting behavior.**
A small example that others can run helps us understand both failures and
proposals.
Include:

1. Minimal input as text and the exact command or Go program you ran.
2. Actual output and expected output, including errors.
3. Relevant options, configuration, and plugin selections.
4. Your Go version, operating system, and installed module revision.

For CLI reports, useful commands include:

```bash
go-yaml -T config.yaml
go-yaml -E config.yaml
go-yaml -N config.yaml
go version
go version -m "$(command -v go-yaml)"
```

If the problem involves Go types, struct tags, or custom marshalers, include
a small Go reproducer as well.
For ideas, show the input and output you would like the CLI to support.
Open an [issue](https://github.com/yaml/go-yaml/issues/new) or join the
[maintainers on Slack](https://cloud-native.slack.com/archives/C08PPAT8PS7).

Contributors can build the CLI with `make cli`, run `make test`, and enter the
project's managed development environment with `make shell`.
The Makefile installs its tools under `.cache/`.
See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## Acknowledgments and license

This project builds on [the original go-yaml repository](
https://github.com/go-yaml/yaml) by Gustavo Niemeyer and its contributors.
It began at Canonical as part of Juju, using a Go port of Kirill Simonov's
[libyaml](https://pyyaml.org/wiki/LibYAML).
The YAML organization continues its maintenance and development.

See [LICENSE](LICENSE) and [NOTICE](NOTICE).
The libyaml-derived files listed in NOTICE retain their original MIT license;
the remaining project files are covered by Apache License 2.0.
