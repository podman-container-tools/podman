# cupwriter

[![GoDoc](https://pkg.go.dev/badge/github.com/vbauerster/cupwriter)](https://pkg.go.dev/github.com/vbauerster/cupwriter)
[![Test status](https://github.com/vbauerster/cupwriter/actions/workflows/test.yml/badge.svg)](https://github.com/vbauerster/cupwriter/actions/workflows/test.yml)
[![Lint status](https://github.com/vbauerster/cupwriter/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/vbauerster/cupwriter/actions/workflows/golangci-lint.yml)

cupwriter is a cross platform buffered `io.Writer` which abstracts writing multi lines to a fixed position within a terminal.
It's used by [mpb](https://github.com/vbauerster/mpb) under the hood which means it's quite battle tested in spite of `v0.0.x`.
