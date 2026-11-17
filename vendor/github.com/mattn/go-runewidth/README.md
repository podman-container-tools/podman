go-runewidth
============

[![Build Status](https://github.com/mattn/go-runewidth/workflows/test/badge.svg?branch=master)](https://github.com/mattn/go-runewidth/actions?query=workflow%3Atest)
[![Codecov](https://codecov.io/gh/mattn/go-runewidth/branch/master/graph/badge.svg)](https://codecov.io/gh/mattn/go-runewidth)
[![GoDoc](https://godoc.org/github.com/mattn/go-runewidth?status.svg)](http://godoc.org/github.com/mattn/go-runewidth)
[![Go Report Card](https://goreportcard.com/badge/github.com/mattn/go-runewidth)](https://goreportcard.com/report/github.com/mattn/go-runewidth)

Provides functions to get fixed width of the character or string.

Usage
-----

```go
runewidth.StringWidth("つのだ☆HIRO") == 12
```

NOTE: Latin letters that Unicode gives ambiguous East Asian width, such as
`ü`, `é` and `ß`, are measured as one cell even when `EastAsianWidth` is
set, as terminals draw them in CJK locales too. Other ambiguous characters
such as `±`, `×` and `○` still take two cells there.


Author
------

Yasuhiro Matsumoto

License
-------

under the MIT License: http://mattn.mit-license.org/2013
