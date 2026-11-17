# Security Policy

## Supported Versions

Only the latest release on the `v1.14.x` line receives security fixes.

| Version  | Supported          |
| -------- | ------------------ |
| 1.14.x   | :white_check_mark: |
| < 1.14   | :x:                |

## Scope

`go-sqlite3` is a CGo binding that bundles the SQLite amalgamation
(`sqlite3-binding.c` / `sqlite3-binding.h`). Please report issues to the
appropriate project:

- Bugs in the Go binding layer, CGo glue, build tags, or this repository's
  own code: report here.
- Vulnerabilities in SQLite itself: please report them upstream to the
  SQLite developers at <https://www.sqlite.org/>. Once a fix is released
  upstream, this repository will update the bundled amalgamation.

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security problems.

Use GitHub's private vulnerability reporting:
<https://github.com/mattn/go-sqlite3/security/advisories/new>

This project is maintained on a best-effort basis by volunteers, so please
allow reasonable time for investigation and a fix before any public
d
