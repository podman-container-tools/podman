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

* **2025-04-19** : large package layout reshuffle
  * the entire project is being refactored to restore a reasonable level of maintenability
  * the only exposed API is Run() and Options.

## Status

API is stable.

## Import this library in your project

```cmd
go get github.com/go-openapi/codescan
```

## Basic usage

```go
import (
  "github.com/go-openapi/codescan"
)

swaggerSpec, err := codescan.Run(&codescan.Options{
  Packages: []string{"./..."},
})
```

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
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
