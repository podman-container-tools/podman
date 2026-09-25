# Security Policy

This policy outlines the commitment and practices of the go-openapi maintainers regarding security.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.36.x  | :white_check_mark: |

## Threat model

codescan is a local developer tool that runs as the invoking user.

**The command line, the terminal and the process environment are the user's own**,
and are not treated as untrusted input: a flag that writes a file is doing what it was asked to.

Usage as a library is intended to serve similar applications, such as [go-swagger][go-swagger-url],
and **not** to serve functionality over a network.

What **is** untrusted is the content that originates from a scanned repository:
its Go source, including the doc comments the output specification is built from,
as well as the `.codescan.yaml` configuration found by searching upwards on the developer's machine.

Scanning a repository exposes the user to code they may not have read, and which may not be theirs,
whatever directory it happens to sit in.

A few consequences worth stating:

- Library: **the specification it emits carries text the scanned repository wrote.**

  > Descriptions, titles and examples all come from that repository's doc comments, so a caller that
  > renders, serves or stores the specification is passing on untrusted content and has to treat it
  > as such. Checking what goes in, and what comes out, stays with the caller.

- TUI: **What a repository writes must not reach the terminal as instructions.**

  > Control characters in source, file names and diagnostics are encoded before they are drawn,
  > so a crafted file cannot repaint the screen or retitle the window of whoever opens it.

- CLI & TUI: **A repository must not choose where the tool reads or writes.**

  > The paths are settable on the command line only, never from a configuration file: the directory
  > the scan runs from, the generated OpenAPI specification, and performance profiles.
  >
  > A configuration file is found by searching upwards, so it belongs to the tree being scanned
  > rather than to the person scanning it. What it may still set shapes the document, not the
  > filesystem; `-no-config` skips it altogether.

- Playground: **the analysis happens entirely in the visitor's browser.**

  > Code is loaded and analysed in the browser, and never sent to a server.
  >
  > That is a property of the playground rather than of `genspec-wasi` itself: run standalone under a
  > WASI runtime, the command reads and writes whatever the host mounts and the command line names.

Reports that do not cross these boundaries — a flag the user typed, a path the user passed — are not treated
as vulnerabilities, however a scanner classifies them.

### The documentation site

Assets fetched at build time are consumed to build the documentation site published to visitors. They do not
ship as a library, so a compromised asset reaches the site's readers and nobody running codescan: a different
boundary with a different audience, which is why it is stated apart from the tool's own.

## Vulnerability checks in place

This repository uses automated vulnerability scans, at every merged commit and at least once a week.

We use:

- [`GitHub CodeQL`][codeql-url]
- [`trivy`][trivy-url]
- [`govulncheck`][govulncheck-url]

Reports are centralized in github security reports and visible only to the maintainers.

## Reporting a vulnerability

If you become aware of a security vulnerability that affects the current repository,
**please report it privately to the maintainers**
rather than opening a publicly visible GitHub issue.

Please follow the instructions provided by github to [Privately report a security vulnerability][github-guidance-url].

> [!NOTE]
> On Github, navigate to the project's "Security" tab then click on "Report a vulnerability".

[codeql-url]: https://github.com/github/codeql
[trivy-url]: https://trivy.dev/docs/latest/getting-started
[govulncheck-url]: https://go.dev/blog/govulncheck
[github-guidance-url]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability#privately-reporting-a-security-vulnerability
[go-swagger-url]: https://github.com/go-swagger/go-swagger
