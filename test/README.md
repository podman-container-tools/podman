![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

> **WARNING: System tests are destructive to your local environment.**
> System tests (`make localsystem`/`make remotesystem`/`hack/bats`) wipe `~/.local/share/containers` and `~/.config/containers` before running, removing all local containers, images, pods, volumes, and machine instances. Run them in a dedicated environment or VM, not on a workstation with data you care about.

# Quick Reference

| Command | What it does |
|---------|-------------|
| `make localunit` | Unit tests with coverage |
| `make localintegration` | [Integration tests (`test/e2e/`)](#integration-tests) |
| `make remoteintegration` | [Integration tests, remote client (`test/e2e/`)](#integration-tests) |
| `make localmachine` | Machine tests (`pkg/machine/e2e/`) |
| `make localsystem` | [System tests (`test/system/`)](#system-tests) |
| `make remotesystem` | [System tests, remote client (`test/system/`)](#system-tests) |
| `make test` | Run everything (prefer specific targets, see below) |

## Running specific tests

```bash
# Integration: tests from a specific file
make localintegration FOCUS_FILE=run_test.go

# Integration: tests matching a description (regex)
make localintegration FOCUS="podman run exit"

# Integration: combine file + description, no parallelism
make localintegration FOCUS_FILE=run_test.go FOCUS="podman run exit" GINKGO_PARALLEL=n

# Machine: specific file or description
make localmachine FOCUS_FILE=basic_test.go
make localmachine FOCUS="podman machine init"

# System: using hack/bats for fine-grained control
hack/bats 500                      # file matching "500" (networking)
hack/bats 220:"restart cleans up"  # specific test in a file
hack/bats --root 160:"ps -f"      # root-only, filtered
hack/bats --remote 500             # test with podman-remote

# Unit: specific package or function
go test -v ./libpod/
go test -v -run TestMyFunction ./pkg/specgen/
```

# Test utils
Test utils provide common functions and structs for testing. It includes two structs:
* `PodmanTest`: Handle the *podman* command and other global resources like temporary
directory. It provides basic methods, like checking podman image and pod status. Test
suites should create their owner test *struct* as a composite of `PodmanTest`, and their
owner PodmanMakeOptions().

* `PodmanSession`: Store execution session data and related *methods*. Such like get command
output and so on. It can be used directly in the test suite, only embed it to your owner
session struct if you need expend it.

## Unittest for test/utils
To ensure neither *tests* nor *utils* break, There are unit-tests for each *functions* and
*structs* in `test/utils`. When you adding functions or structs to this *package*, please
update both unit-tests for it and this documentation.

### Run unit test for test/utils
Run unit test for test/utils.

```
make localunit
```

## Structure of the test utils and test suites
The test *utils* package is at the same level of test suites. Each test suites also have their
owner common functions and structs stored in `libpod_suite_test.go`.

# Ginkgo test framework
[Ginkgo](https://github.com/onsi/ginkgo) is a BDD testing framework. This allows
us to use native Golang to perform our tests and there is a strong affiliation
between Ginkgo and the Go test framework.

## Installing dependencies
Some external binaries are required to successfully run the tests.
The test currently depend on:
 - normal podman runtime dependencies
 - coreutils
 - ncat
 - gzip
 - xz
 - htpasswd
 - iproute2
 - util-linux
 - tar
 - docker
 - systemd/systemctl
 - yq

Most of these are only required for a few tests so it is not a big issue if not everything is installed. Only a few test should fail then.

### Installing ginkgo
Build ginkgo and install it under ./test/tools/build/ginkgo with the following command:
```
make .install.ginkgo
```

# Integration Tests
Test suite for integration test for podman command line. It has its own structs:
* `PodmanTestIntegration`: Integration test *struct* as a composite of `PodmanTest`. It
set up the global options for *podman* command to ignore the environment influence from
different test system.

* `PodmanSessionIntegration`: This *struct* has it own *methods* for checking command
output with given format JSON by using *structs* defined in inspect package.

## Running the integration tests
You can run the entire suite of integration tests with the following command:

```
make localintegration
```

To run the remote tests use:
```
make remoteintegration
```

### Test variables

Some test only work as rootless while others only work as root. So to test everything
you should make sure to run the make command above as normal user and root.

The following environment variables are supported by the test setup:
 - `PODMAN_BINARY`: path to the podman binary, defaults to `bin/podman` in the repository root.
 - `PODMAN_REMOTE_BINARY`: path to the podman-remote binary, defaults to `bin/podman-remote` in the repository root.
 - `QUADLET_BINARY`: path to the quadlet binary, defaults to `bin/quadlet` in the repository root.
 - `CONMON_BINARY`: path to th conmon binary, defaults to `/usr/libexec/podman/conmon`.
 - `OCI_RUNTIME`: which oci runtime to use, defaults to `crun`.
 - `PODMAN_DB`: the database backend `sqlite` (default) or `boltdb`.
 - `PODMAN_TEST_IMAGE_CACHE_DIR`: path were the container images should be cached, defaults to `/tmp`.

Note: These variables are used by the integration tests (test/e2e) only not the system tests (test/system).

### Running a single file of integration tests
You can run a single file of integration tests using the go test command:

```
make localintegration FOCUS_FILE=your_test.go
```

`FOCUS_FILE` file maps to ginkgo's `--focus-file` option, see the ginkgo
[docs](https://onsi.github.io/ginkgo/#location-based-filtering) for the accepted syntax.

For remote tests use the `remoteintegration` Makefile target instead.

### Running a single integration test
Before running the test suite, you have to declare which test you want run in the test
file itself. Consider the following actual test:
```
It("podman inspect bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "no such pod foobar"))
	})
```

To mark this as the test you want run, you simply change the *It* description to *FIt*. Please note how
both the `F` and `I` are capitalized. Also see the ginkgo [docs](https://onsi.github.io/ginkgo/#focused-specs).

*Note*: Be sure you remove the `F` from the tests before committing your changes or you will skip all tests
in that file except the one with the `FIt` denotation.

Alternatively you can use the `FOCUS` option which maps to `--focus`, again see the ginkgo
[docs](https://onsi.github.io/ginkgo/#description-based-filtering) for more info about the syntax.
```
make localintegration FOCUS="podman inspect bogus pod"
```

### Controlling Ginkgo parameters
You can control the parameters passed to Ginkgo.

The flags already used can be set by their corresponding environment variable:

- Disable parallel tests by setting `GINKGO_PARALLEL=n`
- Set flake retry count (default 0) to one by setting `GINKGO_FLAKE_ATTEMPTS=1`
- Produce colorful tests report by setting `GINKGO_NO_COLOR=n`

For anything else, use `TESTFLAGS`.
For example for failing fast, set `TESTFLAGS=--fail-fast`

# System tests
System tests are used for testing the *podman* CLI in the context of a complete system. It
requires that *podman*, all dependencies, and configurations are in place.  The intention of
system testing is to match as closely as possible with real-world user/developer use-cases
and environments. The orchestration of the environments and tests is left to external
tooling. To test a specific oci runtime it should be set in containers.conf.
(Note, upstream only tests with crun, there is no guarantee that tests will pass with another
runtime. We will accept patches to make the tests work with runc, e.g. mostly error message
checks, but not other runtimes due to the high maintenance overhead.)

Only the following env vars are supported:
 - `PODMAN`: path to the podman binary, defaults to `podman` in $PATH.
 - `QUADLET`: path to the quadlet binary, defaults to `/usr/libexec/podman/quadlet`.
 - `PODMAN_TESTING`: path to the podman-testing binary, defaults to `../../bin/podman-testing` from the system test directory.
   Note, the binary is used for testing purposes only and is not expected to be shipped to end users.

System tests use Bash Automated Testing System (`bats`) as a testing framework.
Install it via your package manager or get latest stable version
[directly from the repository](https://github.com/bats-core/bats-core), e.g.:

```
mkdir -p ~/tools/bats
git clone --single-branch --branch v1.1.0 https://github.com/bats-core/bats-core.git ~/tools/bats
```

Make sure that `bats` binary (`bin/bats` in the repository) is in your `PATH`, if not - add it:

```
PATH=$PATH:~/tools/bats/bin
```

System tests also rely on `jq`, `socat`, `nmap`, and other tools. For a
comprehensive list, see the `%package tests` section in the
[fedora specfile](https://src.fedoraproject.org/rpms/podman/blob/main/f/podman.spec).

## Running system tests
When `bats` is installed and is in your `PATH`, you can run the test suite with following command:

```
make localsystem
```

## Running the system tests in a more controlled way
If you would like to run a subset of the system tests, or configure the environment (e.g. root vs rootless, local vs remote),
use `hack/bats`.

For usage run:
```
hack/bats --help
```

## Reproducing CI failures locally with Lima

CI jobs whose names end in `/ lima` run inside a Lima VM. To reproduce one,
copy the job name before `/ lima` and pass it to `hack/ci/ci.sh`:

```bash
hack/ci/ci.sh TEST MODE PRIV DISTRO
```

For example, `sys remote rootless fedora-current / lima` becomes:

```bash
hack/ci/ci.sh sys remote rootless fedora-current
```

Jobs without the `/ lima` suffix cannot be reproduced this way, including
machine tests and Windows and macOS jobs.

`TEST` is the suite, one of:

- `build` builds Podman and its documentation
- `apiv2` runs API v2 tests
- `bindings` runs bindings tests
- `bud` runs Buildah bud tests
- `compose_v2` runs Docker Compose v2 tests
- `docker_py` runs Docker SDK for Python tests
- `unit` runs unit tests
- `upgrade` runs upgrade tests
- `int` runs integration tests
- `sys` runs system tests

`MODE` is `local` or `remote` and defaults to `local`. `PRIV` is `root` or
`rootless` and defaults to `rootless`. `DISTRO` is `fedora-current`,
`fedora-prior`, `fedora-rawhide`, or `debian-sid`.

For the `upgrade` suite, `MODE` is the version to upgrade from (for example,
`v5.3.1`) instead of `local` or `remote`.

The script requires [Lima](https://lima-vm.io/docs/installation/). It starts a
nested-virtualization VM with 8 GB of memory, copies this repository into it,
and runs `hack/ci/runner.sh`, so the host distribution does not matter.
