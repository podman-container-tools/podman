![PODMAN logo](https://raw.githubusercontent.com/containers/container-libs/main/common/logos/podman-logo-full-vert.png)
# Podman Contributing Guide

We'd love to have you join the community!

Please first read our organization wide contributing guide: https://github.com/podman-container-tools/community/blob/main/CONTRIBUTING.md.
It contains all the general recommendations on how to work with issues and how to create proper commits and PRs for the Project.
Below you find helpful advice specific to only the Podman repository.

## Topics

* [Contributing to Podman](#contributing-to-podman)
* [Libraries](#libraries)
* [Codebase structure](#codebase-structure)
* [Testing](#testing)
* [Documentation](#documentation)
* [Continuous Integration](#continuous-integration) [![Build Status](https://github.com/podman-container-tools/podman/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/podman-container-tools/podman/actions/workflows/ci.yml?query=branch%3Amain)

## Contributing to Podman

This section describes how to make a contribution to Podman.
These instructions are geared towards using a Linux development machine, which is required for doing development on the Podman backend.
Development for the Windows and Mac clients can also be done on those operating systems.
Check out these instructions for building the Podman client on [MacOSX](./build_osx.md) or [Windows](./build_windows.md).

> **Note:** Podman 6 dropped support for darwin/amd64 (Intel Mac) binaries. The macOS client is only available for darwin/arm64 (Apple Silicon). Attempting to build with `GOOS=darwin GOARCH=amd64` will fail on any host.

### Prepare your environment

Read the [install documentation to see how to install dependencies](https://podman.io/getting-started/installation#build-and-run-dependencies).

The install documentation will illustrate the following steps:
- Installation of required libraries and tools
- Installing Podman from source

The minimum version of Golang required to build Podman is contained in [go.mod](https://github.com/podman-container-tools/podman/blob/main/go.mod#L5).
You will need to make sure your system's Go compiler is at least this version using the `go version` command.

### Fork and clone Podman

First, you need to fork this project on GitHub.
Then clone your fork locally:
```shell
$ git clone git@github.com:<you>/podman
$ cd ./podman/
```

### Using the Makefile

Podman uses a Makefile for common actions such as compiling Podman, building the documentation, and linting.

You can list available actions by using:
```shell
$ make help
Usage: make <target>
...output...
```

### Install required tools

Makefile allow you to install needed development tools (e.g. the linter):
```shell
$ make install.tools
```

### Building binaries

To build Podman binaries, you can run `make binaries`.
Built binaries will be placed in the `bin/` directory.
You can manually test to verify that Podman is working by running the binaries.

### Building docs

To build Podman's manpages, you can run `make docs`.
Built documentation will be placed in the `docs/build/man` directory.
Markdown versions can be viewed in the `docs/source/markdown` directory.
Files suffixed with `.in` are preliminary versions that are compiled into the final markdown files.

## Libraries

Podman uses a large amount of vendored library code, contained in the `vendor/` directory.
This code is included in the Podman repository, but is actually maintained elsewhere.
Pull requests that change the vendor/ directory directly will not be accepted.
Instead, changes should be submitted to the original package (defined by the path in `vendor/`; for example, `vendor/go.podman.io/storage/` is the [container-libs storage library](https://github.com/podman-container-tools/container-libs/tree/main/storage).
Once the changes have been merged into the original package, Podman's vendor directory can be updated by using `go get` on the appropriate version of the package, then running `make vendor` or `make vendor-in-container`.

## Codebase structure

Description about important directories in our repository is found [here](./docs/CODE_STRUCTURE.md).

## Testing

Podman provides an extensive suite of regression tests in the `test/` directory.
There is a [readme](https://github.com/podman-container-tools/podman/blob/main/test/README.md) file available with details about the tests and how to run them.
All pull requests should be accompanied by test changes covering the changes in the PR.
Pull requests without tests will receive additional scrutiny from maintainers and may be blocked from merging unless tests are added.
Maintainers will decide if tests are not necessary during review.

### Types of Tests

There are several types of tests run by Podman's upstream CI.
* Build testing (including cross-build tests, and testing to verify each commit in a PR builds on its own)
* Go format/lint checking
* Unit testing
* Integration testing (run on several operating systems, both root and rootless)
* System testing (again, run on several operating systems, root and rootless)
* API testing (validates the Podman REST API)
* Machine testing (validates `podman machine` on Windows and Mac hosts)

Changes will usually only need to be tested in one of these.
For example, if you were to make a change to `podman run`, you could test this in either the system tests or the integration tests.
It is not necessary to test a single change in multiple places.

### Go Format and lint

All code changes must pass `make validatepr`.
We are using the [`gofumpt`](https://github.com/mvdan/gofumpt) formatter for our go code, you can either use it directly or format via `golangci-lint fmt`. The `validatepr`/`validate` make targets will fail if the code is not formatted correctly.

### Integration Tests

Our primary means of performing integration testing for Podman is with the [Ginkgo](https://github.com/onsi/ginkgo) BDD testing framework.
This allows us to use native Golang to perform our tests and there is a strong affiliation between Ginkgo and the Go test framework.
Adequate test cases are expected to be provided with PRs.

For details on how to run the tests for Podman in your test environment, see the testing [README.md](test/README.md).

The integration tests are located in the `test/e2e/` directory.

### System Tests

The system tests are written in Bash using the BATS framework.
They provide less comprehensive coverage than the integration tests.
They are intended to validate Podman builds before they are shipped by distributions.

The system tests are located in the `test/system/` directory.

## Documentation

Make sure to update the documentation if needed.
Podman is primarily documented via its manpages, which are located under `docs/source/markdown`.
There are a number of automated tests to make sure the manpages are up to date.
These tests run on all submitted pull requests.
Full details on working with the manpages can be found in the [README](https://github.com/podman-container-tools/podman/blob/main/docs/README.md) for the docs.

Podman also provides Swagger documentation for the REST API.
Swagger is generated from comments on registered handlers located in the `pkg/api/server/` directory.
All API changes should update these Swagger comments to ensure the documentation remains accurate.

## Continuous Integration

All pull requests automatically run Podman's test suite.
The tests have been configured such that only tests relevant to the code changed will be run.
For example, a documentation-only PR with no code changes will run a substantially reduced set of tests.

There is always additional complexity added by automation, and so it sometimes can fail for any number of reasons.
This includes post-merge testing on all branches, which you may occasionally see [failed runs on the workflow history](https://github.com/podman-container-tools/podman/actions/workflows/ci.yml?query=branch%3Amain+is%3Afailure).

Most notably, the tests will occasionally flake.
If you see a single test on your PR has failed, and you do not believe it is caused by your changes, you can rerun the tests.
If you lack permissions to rerun the tests, just wait for a maintainer to rerun them for you. Do not unnecessarily force push
the branch in that case.

If you see multiple test failures, you may wish to check the status graph mentioned above.
When the graph shows mostly green bars on the right, it's a good indication the main branch is currently stable.
Alternating red/green bars is indicative of a testing "flake", and should be examined (anybody can do this):

* *One or a small handful of tests, on a single task, (i.e. specific distro/version)
  where all others ran successfully:*  Frequently the cause is networking or a brief
  external service outage.  The failed tasks may simply be re-run by pressing the
  corresponding button on the task details page.

* *Multiple tasks failing*: Logically this should be due to some shared/common element.
  If that element is identifiable as a networking or external service (e.g. packaging
  repository outage), a re-run should be attempted.

* *All tasks are failing*: If a common element is **not** identifiable as
  temporary (i.e. container registry outage), please try to contact a Podman Maintainer,
  see below for the Matrix channel.

In the (hopefully) rare case there are multiple, contiguous red bars, this is
a ***very bad*** sign.  It means additional merges are occurring despite an uncorrected
or persistently faulty condition.  This risks additional bugs being introduced
and further complication of necessary corrective measures.  Most likely people
are aware and working on this, but it doesn't hurt to confirm and/or try and help
if possible by asking on our Podman developer Matrix channel
[#podman-dev:matrix.org](https://matrix.to/#/#podman-dev:matrix.org).

NOTE: Jobs triggered by Packit are not merge blockers and should be considered of secondary importance.
Contributors and maintainers should feel free to ignore failure status on such jobs.

### PR Approval and Merging

The Podman project uses GitHub's native review system for PR approval and merging.

* **Approving PRs**: Reviewers and maintainers use GitHub's review feature to approve PRs. Select "Approve" when submitting your review to indicate the PR is ready to merge.

* **Merging**: Once a PR has received the required approvals (at least two reviews, with at least one from a maintainer) and CI has passed, a maintainer will merge the PR using GitHub's merge button.
