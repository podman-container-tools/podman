# inflect

<!-- Badges: status  -->
[![Tests][test-badge]][test-url] [![Coverage][cov-badge]][cov-url] [![CI vuln scan][vuln-scan-badge]][vuln-scan-url] [![CodeQL][codeql-badge]][codeql-url]
<!-- Badges: release & docker images  -->
<!-- Badges: code quality  -->
<!-- Badges: license & compliance -->
[![Release][release-badge]][release-url] [![Go Report Card][gocard-badge]][gocard-url] [![CodeFactor Grade][codefactor-badge]][codefactor-url] [![License][license-badge]][license-url]
<!-- Badges: documentation & support -->
<!-- Badges: others & stats -->
[![GoDoc][godoc-badge]][godoc-url] [![Slack Channel][slack-logo]![slack-badge]][slack-url] [![go version][goversion-badge]][goversion-url] ![Top language][top-badge] ![Commits since latest release][commits-badge]

---

A package to pluralize words.

Originally forked from https://bitbucket.org/pkg/inflect under a MIT License.

## Status

API is stable.

This library is not used at all by other go-openapi packages and is somewhat redundant with
go-openapi/swag/mangling (for camelcase etc).

Currently we have one single dependency in one place in a go-swagger template (used as a funcmap).

## Import this library in your project

```cmd
go get github.com/go-openapi/inflect
```

## Basic usage

A golang library applying grammar rules to English words.

> This package provides a basic set of functions applying
> grammar rules to inflect English words, modify case style
> (Capitalize, camelCase, snake_case, etc.).
>
> Acronyms are properly handled. A common use case is word pluralization.

## Change log

See <https://github.com/go-openapi/inflect/releases>

<!--
## References
-->

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

See the license [NOTICE](./NOTICE), which recalls the licensing terms of all the pieces of software
on top of which it has been built.

<!--
## Limitations
-->

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines](.github/CONTRIBUTING.md)
<!--
* [Maintainers documentation](docs/MAINTAINERS.md)
* [Code style](docs/STYLE.md)
-->

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/inflect/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/inflect/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/inflect/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/inflect/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/inflect
[vuln-scan-badge]: https://github.com/go-openapi/inflect/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/inflect/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/inflect/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/inflect/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Finflect.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Finflect
[gomod-badge]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Finflect.svg
[gomod-url]: https://badge.fury.io/go/github.com%2Fgo-openapi%2Finflect
<!-- Badges: code quality  -->
[gocard-badge]: https://goreportcard.com/badge/github.com/go-openapi/inflect
[gocard-url]: https://goreportcard.com/report/github.com/go-openapi/inflect
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/inflect
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/inflect
<!-- Badges: documentation & support -->
[doc-badge]: https://img.shields.io/badge/doc-site-blue?link=https%3A%2F%2Fgoswagger.io%2Fgo-openapi%2F
[doc-url]: https://goswagger.io/go-openapi
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/inflect
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/inflect
[slack-logo]: https://a.slack-edge.com/e6a93c1/img/icons/favicon-32.png
[slack-badge]: https://img.shields.io/badge/slack-blue?link=https%3A%2F%2Fgoswagger.slack.com%2Farchives%2FC04R30YM
[slack-url]: https://goswagger.slack.com/archives/C04R30YMU
<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/inflect/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/inflect
[goversion-url]: https://github.com/go-openapi/inflect/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/inflect
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/inflect/latest
