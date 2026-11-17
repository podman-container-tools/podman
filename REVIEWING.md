# Reviewing Pull Requests

This document contains general principles for how to perform code reviews in the Podman repository.
It does not aim to be a complete guide to how to perform code review, but rather to provide general guidance on how code reviews should be performed.

This document is aimed at Reviewers, Maintainers, and Core Maintainers (see [GOVERNANCE.md](./GOVERNANCE.md) for definitions of these roles), but these guidelines should be followed by all who wish to review code in the Podman project's GitHub repositories, even those who are not currently a maintainer.

## How are reviews performed

The Podman project aims to ensure that all PRs are reviewed by at least 2 people prior to merge, at least one of which must be a repository Maintainer.
There are some exceptions to this: Updates to libraries (including Go vendor updates) that pass CI cleanly and require no code changes may be merged by a maintainer without further review.

All code merged must pass CI.

## What should you review?

We encourage review of all PRs, even those not in the area of expertise of the reviewer.
Even if you are not fully familiar with the code in question, you can still notice basic mistakes.
Feel free to ask questions about how areas of the code work to help familiarize yourself during reviews.
If you finish a review and do not feel like you adequately understood the code to approve it for merge, tag an expert in that area of the code to perform a further review.

Timely PR reviews are important - contributors can become discouraged if a PR is neglected by maintainers.
All Maintainers and Reviewers should try to review new pull requests in their repositories once a day to ensure this.
When you review a PR with failing tests, please check to see if those tests failed due to known flakes.
If so, please restart the failed tests.
Many repositories in the Podman project can only have their tests restarted by project members, not the submitter, so regular attention to test failures by reviewers is important.

## Things to Check

### Breaking Changes

The Podman project aims to present a stable API for its users.
Breaking changes to the project's Command Line Interfaces or public APIs (the Podman REST API and its associated bindings) must only be made in approved breaking change releases - Podman 6.0, 7.0, etc.
Individual repositories should identify what parts of the repository are considered to be their stable API.
Breaking changes can include renaming an option without retaining the original name as an alias, removing an option entirely, or changing how an option works in a way that does not ensure backwards compatibility.

Periods when it is acceptable to merge breaking changes will be widely announced.
If it is not one of those periods, reviewers should be on the lookout for breaking changes.
PRs with breaking changes should not be merged.
Please guide the contributor in how to make the change in a non-breaking fashion.
If this is not possible, deferring the PR to the next breaking change window or closing it entirely is appropriate.

### Commit Messages and Hygiene

Good commit messages are essential for understanding why a change was made in the future.
Reviewers should check each commit of a PR to ensure that it has an appropriate commit message which fully and accurately explains the change and why it is being made.
This should remain true after changes to the PR due to code review, so please re-review commit messages before merging code, to ensure they are still accurate.
Pull requests fixing a specific issue must include a `Fixes: #xxxxx` line in the commit message.
Full details on the `Fixes` line can be found in the [Contributor's Guide](./CONTRIBUTING.md).
Reviewers are responsible for enforcing the guidelines in that document.

Each commit in a PR should be self-contained and have a clear and distinct purpose.
If this is not true, please encourage the contributor to squash their commits using `git rebase -i` until it is true.

### Disagreements between reviewers

We do not expect all reviewers to be of the same opinion during code review.
If you see another reviewer requesting changes to a PR you do not agree with, it is perfectly acceptable to comment to that effect.
Disagreements between maintainers can generally be worked out during PR review through comments.
If this is not possible, other maintainers can be called on to give their opinions and determine a way forward.
We encourage such disagreements and discussions, so long as they remain respectful and are done with the goal of resolving the dispute.

If a decision cannot be reached, the issue may be put to a vote, in which all Maintainers of the repository in question and all Core Maintainers can vote.

Being respectful includes respect for the contributor's time.
If there is a disagreement, please do not make the contributor make repeated changes until an agreement on how to proceed is reached.
Also, please attempt to reach an agreement quickly, so the PR can be merged in a timely fashion.

### Language Version Updates

Most repositories in the Podman project are written in Go, and as such target a specific version of Go in their `go.mod`.
For example:
```
$ cat go.mod | grep 'go 1.'
go 1.22.8
```

Changing this value affects what versions of Go can build the project, and as such increasing it can prevent some distributions with older Golang versions from building Podman.

For all branches except the main branch, Go version should remain static unless there is an extremely good reason to change it (for example, a CVE fix requires pulling in a new version of a library that needs a newer Golang version).
PRs into a non-main branch which change the supported Go version should be modified to not require such a change, or rejected if that is not possible.
Changing supported Go version in the main branch is allowed, but not encouraged.

### Tests

Please check tests added by the PR.
If a PR has no new tests, determine if this is actually appropriate - has new functionality been added which should have been tested?
If the PR does have tests, check to see if they are reasonably comprehensive.
The Podman Project does not have code coverage standards at present, but we aim to ensure that all new functionality is tested.
Tests for bugs and new functionality should, generally speaking, fail when run against Podman without the patch applied.
Reviewers are encouraged to check this when reviewing a pull request.

### Documentation

All changes to public-facing APIs (e.g. the Podman REST API) and CLI should be appropriately documented.
API changes require Swagger documentation.
CLI changes should be documented in the Manpages.

Please validate that PRs making such changes include appropriate documentation.
Many repositories in the Podman project will enforce this via CI check, but you should still review the contents of the documentation to make sure they are appropriate and complete.

## Things to Avoid

### Bikeshedding and excessively critical reviews

Please avoid bikeshedding during reviews.

Trivial changes that do not affect the ultimate functionality of the PR - for example, unnecessary renaming of variables, or small changes to code style or formatting - should not block merge of a PR.
It is acceptable to make such comments, but they should be marked as nice to have changes.
Exceptions can be made with documentation, as ensuring correctness and clarity in documentation is very important.
Ensuring our documentation is free of typos and obvious grammatical issues is not bikeshedding and is allowed.

### Asking for unrelated changes

Please do not request that a contributor make significant changes to code their PR did not touch.
If you are reviewing and find problems in pre-existing code in a file that the PR changed, you should not require that the contributor change this code as well.
Asking if they are willing to do so is fine, but do not block the merge of the PR if they are unwilling.

Some changes are OK to ask for - for example, asking a contributor to refactor existing code very similar to something being added to prevent code duplication.
However, larger changes that would substantially increase the size of the PR should be avoided.
Reviewers should use their best judgement to balance respect for the contributor's time and the code hygiene of the project.
If a change is too large to be reasonably asked for, consider asking the contributor to add a comment with a "TODO" or "FIXME" to the area that needs changing (or making a PR yourself with such a comment).
