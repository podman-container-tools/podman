Quick overview of podman system tests. The idea is to use BATS,
but with a framework for making it easy to add new tests and to
debug failures.

Quick Start
===========

Look at [000-TEMPLATE](000-TEMPLATE) for a simple starting point.
This introduces the basic set of helper functions:

* `setup` (implicit) - establishes a test environment.

* `parse_table` - you can define tables of inputs and expected results,
then read those in a `while` loop. This makes it easy to add new tests.
Because bash is not a programming language, the caller of `parse_table`
sometimes needs to massage the returned values; `030-run.bats` offers
examples of how to deal with the more typical such issues.

* `run_podman` - runs command defined in `$PODMAN` (default: 'podman'
but could also be './bin/podman' or 'podman-remote'), with a timeout.
Checks its exit status.

* `assert` - compare actual vs expected output. Emits a useful diagnostic
on failure.

* `die` - output a properly-formatted message to stderr, and fail test

* `skip_if_rootless` - if rootless, skip this test with a helpful message.

* `skip_if_remote` - like the above, but skip if testing `podman-remote`

* `safename` - generates a pseudorandom lower-case string suitable
for use in names for containers, images, volumes, any object. String
includes the BATS test number, making it possible to identify the
source of leaks (failure to clean up) at the end of tests.

* `random_string` - returns a pseudorandom alphanumeric string suitable
for verifying I/O.

Test files are of the form `NNN-name.bats` where NNN is a three-digit
number. Please preserve this convention, it simplifies viewing the
directory and understanding test order. In particular, `00x` tests
should be reserved for a first-pass fail-fast subset of tests:

    bats test/system/00*.bats || exit 1
    bats test/system

...the goal being to provide quick feedback on catastrophic failures
without having to wait for the entire test suite.


Running tests
=============
To run the tests locally in your sandbox using `hack/bats` is recommend, check `hack/bats --help` for info about usage.

To run the entire suite use `make localsystem` or `make remotesystem` for podman-remote testing.

Analyzing test failures
=======================

The top priority for this scheme is to make it easy to diagnose
what went wrong. To that end, `podman_run` always logs all invoked
commands, their output and exit codes. In a normal run you will never
see this, but BATS will display it on failure. The goal here is to
give you everything you need to diagnose without having to rerun tests.

The `assert` comparison function is designed to emit useful diagnostics,
in particular, the actual and expected strings. Please do not use
the horrible BATS standard of `[ x = y ]`; that's nearly useless
for tracking down failures.

If the above are not enough to help you track down a failure:


Debugging tests
---------------

Some functions have `dprint` statements. To see the output of these,
set `PODMAN_TEST_DEBUG="funcname"` where `funcname` is the name of
the function or perhaps just a substring.


Requirements
============

- bats
- jq
- skopeo
- httpd-tools
- openssl
- socat
- buildah
- gnupg
- xfsprogs


Further Details
===============

TBD. For now, look in [helpers.bash](helpers.bash); each helper function
has (what are intended to be) helpful header comments. For even more
examples, see and/or run `helpers.t`; that's a regression test
and provides a thorough set of examples of how the helpers work.
