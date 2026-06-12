![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)
# Contributing to Podman

We'd love to have you join the community!
Below summarizes the processes that we follow.

## Topics

* [LLM ("AI") Policy](#llm-ai-policy)
* [Reporting Issues](#reporting-issues)
* [Working On Issues](#working-on-issues)
* [Contributing to Podman](#contributing-to-podman)
* [Continuous Integration](#continuous-integration) [![Build Status](https://github.com/containers/podman/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/containers/podman/actions/workflows/ci.yml?query=branch%3Amain)
* [Submitting Pull Requests](#submitting-pull-requests)
* [Communications](#communications)

## LLM ("AI") Policy

If your contribution is aided by LLMs or other AI tools, please read the [LLM Policy](LLM_POLICY.md).
This includes comments, issues, PRs and any other interactions with the Podman team.

## Reporting Issues

Before reporting an issue, check our backlog of [open issues](https://github.com/containers/podman/issues) to see if someone else has already reported it.
If so, feel free to add your scenario, or additional information, to the discussion.
Or simply "subscribe" to it to be notified when it is updated.
Please do not add comments like "+1" or "I have this issue as well" without adding any new information.
Instead, please add a thumbs-up emoji to the original report.

Note: Older closed issues/PRs are automatically locked.
If you have a similar problem please open a new issue instead of commenting.

If you find a new issue with the project we'd love to hear about it!
The most important aspect of a bug report is that it includes enough information for us to reproduce it.
To make this easier, there are three types of issue templates you can use.
* If you have a bug to report, please use *Bug Report* template.
* If you have an idea to propose, please use the *Feature Request* template.
* If your issue is something else, please use the default *Blank issue* template.
Please include as much detail as possible, including all requested fields in the template.
Not having all requested information - for example, a full `podman info` - makes it much harder to find and fix issues.
A reproducer is the best thing you can include.
Reproducers make finding and fixing issues much easier for maintainers.
The easier it is for us to reproduce a bug, the faster it'll be fixed!

Please don't include any private/sensitive information in your issue!
Security issues should NOT be reported via Github and should instead be reported via the process described [here](https://github.com/containers/container-libs/blob/main/SECURITY.md).

## Working On Issues

Once you have decided to contribute to Podman by working on an issue, check our backlog of [open issues](https://github.com/containers/podman/issues) looking for any that are unassigned.
If you want to work on a specific issue that is already assigned but does not appear to be actively being worked on, please ping the assignee in the issue and ask if you can take over.
If they do not respond after several days, you can notify a maintainer to have the issue reassigned.
When working on an issue, please assign it to yourself.
You can use the `/assign` bot command in a comment on an issue to assign it to yourself.
If you lack permissions to do so, you can ping the `@containers/podman-maintainers` group to have a maintainer set you as assignee.
If you are a maintainer of Podman project, please following the [instructions](https://github.com/containers/podman/blob/main/TRIAGE.md) to triage new issues.

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

The minimum version of Golang required to build Podman is contained in [go.mod](https://github.com/containers/podman/blob/main/go.mod#L5).
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

For further reading about branching [you can read this document](https://herve.beraud.io/containers/linux/podman/isolate/environment/2019/02/06/how-to-hack-on-podman.html).

### Building docs

To build Podman's manpages, you can run `make docs`.
Built documentation will be placed in the `docs/build/man` directory.
Markdown versions can be viewed in the `docs/source/markdown` directory.
Files suffixed with `.in` are preliminary versions that are compiled into the final markdown files.

## Libraries

Podman uses a large amount of vendored library code, contained in the `vendor/` directory.
This code is included in the Podman repository, but is actually maintained elsewhere.
Pull requests that change the vendor/ directory directly will not be accepted.
Instead, changes should be submitted to the original package (defined by the path in `vendor/`; for example, `vendor/github.com/containers/storage` is the [containers/storage library](https://github.com/containers/storage/).
Once the changes have been merged into the original package, Podman's vendor directory can be updated by using `go get` on the appropriate version of the package, then running `make vendor` or `make vendor-in-container`.

## Codebase structure

Description about important directories in our repository is found [here](./docs/CODE_STRUCTURE.md).

## Testing

Podman provides an extensive suite of regression tests in the `test/` directory.
There is a [readme](https://github.com/containers/podman/blob/main/test/README.md) file available with details about the tests and how to run them.
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
Full details on working with the manpages can be found in the [README](https://github.com/containers/podman/blob/main/docs/README.md) for the docs.

Podman also provides Swagger documentation for the REST API.
Swagger is generated from comments on registered handlers located in the `pkg/api/server/` directory.
All API changes should update these Swagger comments to ensure the documentation remains accurate.

## Submitting Pull Requests

No Pull Request (PR) is too small!
Typos, additional comments in the code, new test cases, bug fixes, new features, more documentation, ... it's all welcome!

While bug fixes can first be identified via an "issue" in Github, that is not required.
It's ok to just open up a PR with the fix, but make sure you include the same information you would have included in an issue - like how to reproduce it.

PRs for new features should include some background on what use cases the new code is trying to address.
When possible and when it makes sense, try to break-up larger PRs into smaller ones - it's easier to review smaller code changes.
But only if those smaller ones make sense as stand-alone PRs.

Pull requests should be submitted to the main branch of the Podman repository. Bug fixes may be cherry-picked or back-ported
to Podman release branches but must first be merged upstream. Maintainers reserve the right to not accept any pull requests
to any release branches.

Regardless of the type of PR, all PRs should include:
* Well-documented code changes, both through comments in the code itself and high-quality commit messages.
* Additional tests. Ideally, they should fail w/o your code change applied.
  (With a few exceptions, CI hooks will block your PR unless your change
  includes files named `*_test.go` or under the `test/` subdirectory. Repo
  admins may bypass this restriction by setting the 'No New Tests' GitHub
  label on the PR).
* Documentation updates to reflect the changes made in the pull request.

Squash your commits into logical pieces of work that might want to be reviewed separate from the rest of the PRs.
Squashing down to just one commit is also acceptable since in the end the entire PR will be reviewed anyway.
When in doubt, squash.

When your PR fixes an issue, please note that by including `Fixes: #00000` in the commit description.
More details on this are below, in the "Describe your changes in Commit Messages" section.

The Podman repo follows a two-ack policy for merges.
PRs will be approved by an [approver][owners] listed in [`OWNERS`](OWNERS).
They will then be merged by a repo owner.
Two reviews are required for a pull request to merge.

### Describe your Changes in Commit Messages

Describe your problem.
Whether your patch is a one-line bug fix or 5000 lines of a new feature, there must be an underlying problem that motivated you to do this work.
Convince the reviewer that there is a problem worth fixing and that it makes sense for them to read past the first paragraph.

Describe user-visible impact.
Straight up crashes and lockups are pretty convincing, but not all bugs are that blatant.
Even if the problem was spotted during code review, describe the impact you think it can have on users.
Keep in mind that the majority of users run packages provided by distributions, so include anything that could help route your change downstream.

Quantify optimizations and trade-offs.
If you claim improvements in performance, memory consumption, stack footprint, or binary size, include
numbers that back them up.
But also describe non-obvious costs.
Optimizations usually aren’t free but trade-offs between CPU, memory, and readability; or, when it comes to heuristics, between different workloads.
Describe the expected downsides of your optimization so that the reviewer can weigh costs against
benefits.

Once the problem is established, describe what you are actually doing about it in technical detail.
It’s important to describe the change in plain English for the reviewer to verify that the code is behaving as you intend it to.

Solve only one problem per patch.
If your description starts to get long, that’s a sign that you probably need to split up your patch.

If the patch fixes a logged bug entry, refer to that bug entry by number or URL.
If the patch follows from a mailing list discussion, give a URL to the mailing list archive.
Please format these lines as `Fixes:` followed by the URL or, for Github bugs, the bug number preceded by a #.
For example:

```
Fixes: #00000
Fixes: https://github.com/containers/container-libs/issues/00000
Fixes: https://issues.redhat.com/browse/RHEL-00000
Fixes: RHEL-00000
```

However, try to make your explanation understandable without external resources.
In addition to giving a URL to a mailing list archive or bug, summarize the relevant points of the discussion that led to the patch as submitted.

If you want to refer to a specific commit, don’t just refer to the SHA-1 ID of the commit.
Please also include the oneline summary of the commit, to make it easier for reviewers to know what it is about. If the commit was merged in Github, referring to a Github PR number is also a good option, as that will retain all discussion from development, and makes including a summary less critical.
Examples:

```
Commit f641c2d9384e ("fix bug in rm -fa parallel deletes") [...]
PR #00000
```

When referring to a commit by SHA, you should also be sure to use at least the first twelve characters of the SHA-1 ID.
The Podman repository holds a lot of objects, making collisions with shorter IDs a real possibility.
Bear in mind that, even if there is no collision with your six-character ID now, that condition may change five years from now.

The following git config settings can be used to add a pretty format for outputting the above style in the git log or git show commands:

```
[core]
        abbrev = 12
[pretty]
        fixes = Fixes: %h (\"%s\")
```

### Sign your PRs

The sign-off is a line at the end of the explanation for the patch.
Your signature certifies that you wrote the patch or otherwise have the right to pass it on as an open-source patch.
The rules are simple: if you can certify the below (from [developercertificate.org](https://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions).

If you set your `user.name` and `user.email` git configs, you can sign your commit automatically with `git commit -s`.

#### Automatic DCO Signing (Recommended)

To automatically sign all your commits and avoid DCO check failures, configure Git to use the repository's hooks:

```shell
$ make init-git-hooks
```

This will configure Git to automatically add the `Signed-off-by` trailer to all your commit messages.

### Reviewing PRs

If you are a maintainer of Podman project, please following the [guidelines](https://github.com/containers/podman/blob/main/REVIEWING.md) on how to review a PR.

### Continuous Integration

All pull requests automatically run Podman's test suite.
The tests have been configured such that only tests relevant to the code changed will be run.
For example, a documentation-only PR with no code changes will run a substantially reduced set of tests.
To force a PR to run all tests, you can include the string `[CI:ALL]` in the PR title, but this is almost never necessary.

There is always additional complexity added by automation, and so it sometimes can fail for any number of reasons.
This includes post-merge testing on all branches, which you may occasionally see [failed runs on the workflow history](https://github.com/containers/podman/actions/workflows/ci.yml?query=branch%3Amain+is%3Afailure).

Most notably, the tests will occasionally flake.
If you see a single test on your PR has failed, and you do not believe it is caused by your changes, you can rerun the tests.
If you lack permissions to rerun the tests, please ping the maintainers using the `@containers/podman-maintainers` group and request that the failing test be rerun.

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
  temporary (i.e. container registry outage), please seek assistance via
  [the methods below](#communications) as this may be early indication of
  a more serious problem.

In the (hopefully) rare case there are multiple, contiguous red bars, this is
a ***very bad*** sign.  It means additional merges are occurring despite an uncorrected
or persistently faulty condition.  This risks additional bugs being introduced
and further complication of necessary corrective measures.  Most likely people
are aware and working on this, but it doesn't hurt [to confirm and/or try and help
if possible.](#communications).

NOTE: Jobs triggered by Packit are not merge blockers and should be considered of secondary importance.
Contributors and maintainers should feel free to ignore failure status on such jobs.

## Communications

If you need help, you can contact the maintainers using the channels mentioned in Podman's [communications](https://github.com/containers/podman/blob/main/README.md#communications) document.

For discussions around issues/bugs and features, you can use the GitHub
[issues](https://github.com/containers/podman/issues)
and
[PRs](https://github.com/containers/podman/pulls)
tracking system.

### PR Approval and Merging

The Podman project uses GitHub's native review system for PR approval and merging.

* **Approving PRs**: Reviewers and maintainers use GitHub's review feature to approve PRs. Select "Approve" when submitting your review to indicate the PR is ready to merge.

* **Merging**: Once a PR has received the required approvals (at least two reviews, with at least one from a maintainer) and CI has passed, a maintainer will merge the PR using GitHub's merge button.
