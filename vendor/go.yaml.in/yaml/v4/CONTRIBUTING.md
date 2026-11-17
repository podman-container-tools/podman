Contributing to go-yaml
=======================

Thank you for your interest in contributing to go-yaml!

This document provides guidelines and instructions for contributing to this
project.


## Code of Conduct

By participating in this project, you agree to follow our Code of Conduct.

We expect all contributors to:

- Be respectful and inclusive
- Use welcoming and inclusive language
- Be collaborative and constructive
- Focus on what is best for both the Go and YAML communities


## How to Contribute


### Reporting Issues

Before submitting an issue, please:

- Check if the issue already exists in our issue tracker
- Use a clear and descriptive title
- Provide detailed steps to reproduce the issue
- Include relevant code samples and error messages
- Specify your Go version and operating system
- Use the `go-yaml` CLI tool described below


### Using the `go-yaml` CLI Tool

This tool can be used to inspect both the internal stages and final results of
YAML processing with the go-yaml library.
It should be used when reporting most bugs.

The `go-yaml` CLI tool uses the `go.yaml.in/yaml/v4` library to decode and
encode YAML.
Decoding YAML is a multi-stage process that involves tokens, events, and nodes.
The `go-yaml` CLI tool lets you see all of these intermediate stages of the
decoding process.
This is crucial for understanding what go-yaml is doing internally.

The `go-yaml` CLI tool can be built with the `make go-yaml` command or installed
with the `go install go.yaml.in/yaml/v4/cmd/go-yaml@latest` command.

You can learn about all of its options with the `go-yaml -h` command.

Here is an example of using it on a small piece of YAML:

```bash
./go-yaml -t <<< '
foo: &a1 bar
*a1: baz
'
```


### Coding Conventions

- Follow standard Go coding conventions
- Use `make fmt` to format your code
- Write descriptive comments for non-obvious code
- Add tests for your work
- Keep line length to 80 characters
- Use meaningful variable and function names
- Start doc and comment sentences on a new line
- Test your changes with the `go-yaml` CLI tool when working on parsing logic


### Commit Conventions

- No merge commits
- Commit subject line should:
  - Start with a capital letter
  - Not end with a period
  - Be no more than 50 characters


### Pull Requests

1. Fork the repository
1. Create a new branch for your changes
1. Make your changes following our coding conventions
   - If you are not sure about the coding conventions, please ask
   - Look at the existing code for examples
1. Write clear commit messages
1. Update tests and documentation
1. Submit a pull request


### Testing

- Ensure all tests pass with `make test`
- Add new tests for new functionality
- Update existing tests when modifying functionality


## Development Process

- This project makes use of a GNU makefile (`GNUmakefile`) for many dev tasks
- The makefile doesn't use your locally installed Go commands; it auto-installs
  them, so that all results are deterministic
- Fork and clone the repository
- Make your changes
- Run tests, linters and formatters
  - `make fmt`
  - `make tidy`
  - `make lint`
  - `make lint-yaml`
  - `make test`
  - You can use `make check` to run all of the above
- Submit a [Pull Request](https://github.com/yaml/go-yaml/pulls)


### Using Your Own Go with the Makefile

We ask that you always test with the makefile installed Go before committing,
since it is deterministic and uses the exact same flow as the go-yaml CI.

We also realize that many Go devs need to run their locally installed Go
commands for their development environment, and might want to use them with
the go-yaml makefile.

If you need to use your own Go utils with the makefile, set `GO_YAML_PATH` to
the directory(s) containing them (either by exporting it or passing it to
`make`).

Something like this:

```bash
export GO_YAML_PATH=$(dirname "$(command -v go)")
make test
# or
make test GO_YAML_PATH=$(dirname "$(command -v go)")
```

**Note:** `GO-VERSION` and `GO_YAML_PATH` are mutually exclusive.
When `GO_YAML_PATH` is set, the makefile uses your own Go environment and
ignores any `GO-VERSION` setting.


### Using the Makefile Environment as a Shell

Sometimes you might want to run your own shell commands using the same binaries
that the makefile installs.

To get a subshell with this environment, run one of:

```bash
make shell
make bash
make zsh
make shell GO-VERSION=1.23.4
```



## Makefile Targets

The repository's makefile (`GNUmakefile`) provides a number of useful targets:

- `make test` runs all tests including yaml-test-suite tests
- `make test-main` runs just the unit tests
- `make test-internal` runs just the internal tests
- `make test-yts` runs just the yaml-test-suite tests
- `make test v=1 opts='-count=3'` runs the tests with options
- `make test GO-VERSION=1.23.4` runs the tests with a specific Go version
- `make test GO_YAML_PATH=/path/to/go/bin` uses your own Go installation
- `make shell` opens a shell with the project's dependencies set up
- `make shell GO-VERSION=1.23.4` opens a shell with a specific Go version
- `make fmt` runs `golangci-lint fmt ./...`
- `make lint` runs `golangci-lint run`
- `make lint-yaml` runs a pinned yamllint using uv from Makes
- `make tidy` runs `go mod tidy`
- `make distclean` cleans the project completely


## Preparing a GitHub Release

Write and review the announcement in `docs/releases/vVERSION.md`.
After the annotated tag is available on GitHub, run from the repository root:

```bash
make release v=4.0.0
```

Makes installs the GitHub CLI automatically.
Authenticate it with an account that can create releases in `yaml/go-yaml`.
The target requires an unprefixed stable version, a matching remote tag, and
nonempty release notes.
It creates an unpublished draft and prints its URL.
Review the rendered announcement on GitHub, then publish it as a stable release
marked **Latest**.
Run `make test-release` to check the target using a mock GitHub CLI.


## Getting Help

If you need help, you can:
- Open an [issue](https://github.com/yaml/go-yaml/issues) with your question
- Start a [discussion](https://github.com/yaml/go-yaml/discussions)
- Read through our [documentation](https://pkg.go.dev/go.yaml.in/yaml/v4)
- Check the [migration guide](docs/v3-to-v4-migration.md) if upgrading from v3
- Join our [Slack channel](https://cloud-native.slack.com/archives/C08PPAT8PS7)


## We are a Work in Progress

This project is very much a team effort.
We are just getting things rolling and trying to get the foundations in place.
There are lots of opinions and ideas about how to do things, even within the
core team.

Once our process is more mature, we will likely change the rules here.
We'll make the new rules as a team.
For now, please stick to the rules as they are.

This project is focused on serving the needs of both the Go and YAML
communities.
Sometimes those needs can be in conflict, but we'll try to find common ground.


## Thank You

Thank you for contributing to go-yaml!
