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

## CRIT

The `crit` package provides bindings to decode, encode, and manipulate
CRIU image files natively within Go. It also provides a CLI tool similar
to the original CRIT Python tool. To get started with this, see the docs
at [CRIT (Go library)](https://criu.org/CRIT_%28Go_library%29).

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
| v8             | 8.2.0          | 4.2          |
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
