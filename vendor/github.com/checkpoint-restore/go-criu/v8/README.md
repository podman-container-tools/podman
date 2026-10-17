<!-- markdownlint-configure-file { "no-hard-tabs": { "code_blocks": false } } -->
# go-criu -- Go bindings for CRIU

[![ci](https://github.com/checkpoint-restore/go-criu/actions/workflows/main.yml/badge.svg)](https://github.com/checkpoint-restore/go-criu/actions/workflows/main.yml)
[![verify](https://github.com/checkpoint-restore/go-criu/actions/workflows/verify.yml/badge.svg)](https://github.com/checkpoint-restore/go-criu/actions/workflows/verify.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/checkpoint-restore/go-criu.svg)](https://pkg.go.dev/github.com/checkpoint-restore/go-criu)

This repository provides Go bindings for [CRIU](https://criu.org/).
The code is based on the Go-based PHaul implementation from the CRIU repository.
For easier inclusion into other Go projects, the CRIU Go bindings have been
moved to this repository.

## CRIU

The Go bindings provide an easy way to use the CRIU RPC calls from Go without
the need to set up all the infrastructure to make the actual RPC connection to CRIU.

The following example would print the version of CRIU:

```go
import (
	"log"

	"github.com/checkpoint-restore/go-criu/v8"
)

func main() {
	c := criu.MakeCriu()
	version, err := c.GetCriuVersion()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(version)
}
```

or to just check if at least a certain CRIU version is installed:

```go
	c := criu.MakeCriu()
	result, err := c.IsCriuAtLeast(31100)
```

### Plugin options

Set `rpc.CriuOpts.PluginOptions` to pass options to CRIU plugins for each
request. Each entry uses `PLUGIN.NAME[=VALUE]` syntax, without leading `--`
or the CLI's `--plugin-option=` prefix. For example, add CUDA options to
otherwise configured dump options:

```go
opts.PluginOptions = []string{
	"cuda_plugin.backend=cuda-checkpoint",
	"cuda_plugin.timeout=600",
}
if err := c.Dump(opts, nil); err != nil {
	return err
}
```

For restore, set options on the restore request:

```go
restoreOpts.PluginOptions = []string{
	"cuda_plugin.backend=driver-api",
	"cuda_plugin.device-map=auto",
}
if err := c.Restore(restoreOpts, nil); err != nil {
	return err
}
```

CUDA supports these options in CRIU builds that provide the corresponding
plugin functionality:

- `cuda_plugin.backend`: `auto` (the default), `driver-api`, or
  `cuda-checkpoint`.
- `cuda_plugin.timeout`: a nonnegative number of seconds, with `0` (the
  default) meaning unlimited. This limits CLI helper invocations; it does
  not impose a deadline on in-process Driver API calls. It is separate from
  `CriuOpts.Timeout`, which controls task freezing and the native CUDA lock
  timeout.
- `cuda_plugin.device-map`: `auto` or explicit mappings such as `0=1,1=0`.
  This option is valid only for restore.

The slice preserves option order, including repeated keys. CRIU validates
option syntax, and each plugin interprets its recognized settings. Options
belong to the supplied `CriuOpts`; use separate options for dump and restore,
or replace the slice when reusing an options object.

The CRIU executable must support the `plugin_options` RPC field and have the
required plugin installed and available. Older CRIU builds may silently
ignore the unknown protobuf field, so request success alone does not prove
that the options were applied. `FeatureCheck` currently has no capability
flag for plugin options. Verify the CRIU build and plugin support used by
your application.

## CRIT

The `crit` package provides bindings to decode, encode, compress, and decompress
CRIU image files natively within Go. It also provides a CLI tool similar
to the original CRIT Python tool. To get started with this, see the docs
at [CRIT (Go library)](https://criu.org/CRIT_%28Go_library%29).

Compression commands process one checkpoint directory at a time. For an
incremental chain, pass its newest checkpoint: the directory that no other
checkpoint uses as a parent. Compressing an older layer is unsupported because
the command cannot update newer checkpoints that depend on it. A standalone
checkpoint can be compressed normally with `crit compress DIR`.
Compression requires a full checkpoint with `mm-<pid>.img` metadata to identify
mappings that must remain raw. Pre-dumps are unsupported.
By default, every replaced image is kept as a `.bak` file next to it. Remove
those backups before transforming the same directory again, or pass
`--in-place` to skip creating them.

Reading compressed checkpoint images is supported on every platform on which
CRIT builds. The `crit compress` and `crit decompress` transforms preserve
Linux filesystem metadata and transaction guarantees and are therefore
available only on Linux; on other systems their Go APIs return
`errors.ErrUnsupported`.

## Releases

The first go-criu release was 3.11 based on CRIU 3.11. The initial plan
was to follow CRIU so that go-criu would carry the same version number as
CRIU.

As go-criu is imported in other projects and as Go modules are expected
to follow Semantic Versioning go-criu will also follow Semantic Versioning
starting with the 4.0.0 release.

The following table shows the relation between go-criu and criu versions:

| Major version  | Latest release | CRIU version |
| -------------- | -------------- | ------------ |
| v8             | 8.5.0          | 4.3          |
| v8             | 8.4.0          | 4.2          |
| v7             | 7.2.0          | 3.19         |
| v7             | 7.0.0          | 3.18         |
| v6             | 6.3.0          | 3.17         |
| v5             | 5.3.0          | 3.16         |
| v5             | 5.0.0          | 3.15         |
| v4             | 4.1.0          | 3.14         |

## How to contribute

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to contribute to this
project.

## License and copyright

Unless mentioned otherwise in a specific file's header, all code in
this project is released under the Apache 2.0 license.

The author of a change remains the copyright holder of their code
(no copyright assignment). The list of authors and contributors can be
retrieved from the git commit history and in some cases, the file headers.
