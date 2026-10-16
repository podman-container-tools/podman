# codescan

<!-- Badges: status  -->
[![Tests][test-badge]][test-url] [![Coverage][cov-badge]][cov-url] [![CI vuln scan][vuln-scan-badge]][vuln-scan-url] [![CodeQL][codeql-badge]][codeql-url]
<!-- Badges: release & docker images  -->
<!-- Badges: code quality  -->
<!-- Badges: license & compliance -->
[![Release][release-badge]][release-url] [![Go Report Card][gocard-badge]][gocard-url] [![CodeFactor Grade][codefactor-badge]][codefactor-url] [![License][license-badge]][license-url]
<!-- Badges: documentation & support -->
<!-- Badges: others & stats -->
[![GoDoc][godoc-badge]][godoc-url] [![Discord Channel][discord-badge]][discord-url] [![go version][goversion-badge]][goversion-url] ![Top language][top-badge] ![Commits since latest release][commits-badge]

---

A Go source code scanner that produces Swagger 2.0 (OpenAPI 2.0) specifications from annotated Go source files.

Supports Go modules (since go1.11).

## Announcements

* **2026-07-31** : landed a new spec generation TUI tool

* **2026-04-19** : large package layout reshuffle
  * the entire project is being refactored to restore a reasonable level of maintenability
  * the only exposed API is Run() and Options.

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/codescan
```

## Basic usage as a library

```go
import (
  "github.com/go-openapi/codescan"
)

swaggerSpec, err := codescan.Run(&codescan.Options{
  Packages: []string{"./..."},
})
```

## Work with the TUI

This project comes with a terminal UI to quickly render a Swagger spec from source
and navigate your code annotations. It shows diagnostics and you may test the impact
of the various available options.

```cmd
go install github.com/go-openapi/codescan/cmd/genspec-tui@latest
```

```cmd
genspec-tui -workdir [my source location]
```

![tui_screenshot](docs/genspec-tui.png)

A walkthrough of what it is for — scanning, tracking a node back to its source,
diagnostics and spec validation — is on the doc site:
[Usage as a terminal UI][tui-doc-site].

## Generate a spec from the command line

`genspec` is the headless counterpart: it writes the specification to standard output, or to the file
`-output` names, and reports what the scan observed as colored diagnostics on standard error.

```cmd
go install github.com/go-openapi/codescan/cmd/genspec@latest
```

```cmd
genspec -workdir [my source location] ./...
```

Every option the library takes is a flag. Beyond those, it writes YAML as readily as JSON
(`-output swagger.yaml` is enough), merges its discoveries into an existing document with `-input`,
and checks what it produced with `-validate`. Its exit status says which of those went wrong.

It does the same job as go-swagger's `swagger generate spec`, which drives this same library, but is
released on its own — so fixes and enhancements reach it at this project's pace, and it carries only
the dependencies a spec generator needs.

See [cmd/genspec/README.md](cmd/genspec/README.md).

### Where there is no Go toolchain

`genspec-wasi` runs the same scan taking no dependency beyond the library, so it cross-compiles to
WebAssembly and runs under a WASI runtime with no Go toolchain installed and no subprocess.

```cmd
go install github.com/go-openapi/codescan/cmd/genspec-wasi@latest
```

```cmd
genspec-wasi -workdir [my source location] ./...
```

`-format=json` wraps the document with everything the scan observed — diagnostics
and cross-references, each carrying a source position — for a caller that wants
to do something with them rather than read them.

See [cmd/genspec-wasi/README.md](cmd/genspec-wasi/README.md) for the WASI build, what a guest needs mounted,
and how to ship the standard library's types inside the artifact.

## Scan in a browser

**Experimental, and offered for demonstration** — the interface is verified by hand rather than by
tests. [`hack/doc-site/genspec-wasi`](hack/doc-site/genspec-wasi/README.md) is the same
artifact with a front-end around it: open a Go module, watch the specification it produces, edit the
source and watch it change. There is no server and nothing is uploaded — the scanner is codescan
compiled to WebAssembly, running in the tab.

It follows `genspec-tui` closely enough to be judged against it: syntax highlighting on both sides, a
diagnostics gutter, `/` search, and cross-references that answer *which Go code produced this node*
and *what did this field turn into* — by position rather than by guessing at names.

Destined for the documentation site, where a tutorial's example box becomes something you can edit.

## Change log

See <https://github.com/go-openapi/codescan/releases>

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

See the license [NOTICE](./NOTICE), which recalls the licensing terms of all the pieces of software
on top of which it has been built.

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines][contributing-doc-site]
* [Maintainers documentation][maintainers-doc-site]
* [Code style][style-doc-site]

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/codescan/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/codescan/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/codescan/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/codescan/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/codescan
[vuln-scan-badge]: https://github.com/go-openapi/codescan/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/codescan/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/codescan/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/codescan/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Fcodescan.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Fcodescan
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/codescan
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/codescan
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/codescan
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/codescan
<!-- Badges: documentation & support -->
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/codescan
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/codescan
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/FfnFYaC3k5

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/codescan/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/codescan
[goversion-url]: https://github.com/go-openapi/codescan/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/codescan
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/codescan/latest
<!-- Project docs -->
[tui-doc-site]: https://go-openapi.github.io/codescan/getting-started/usage-as-a-tui/
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
