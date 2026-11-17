# Issue reporting on Podman

The Podman team cares for our users and our communities.  When someone has a problem with
Podman and takes the time to report an [issue](https://github.com/containers/podman/issues)
on our Github, we deeply appreciate the effort.  We want to help. Consider reading
our [support](SUPPORT.md) document prior to submitting an issue as well.

## Considerations when reporting an issue upstream

### Where to report your issue

If you are running Podman acquired from a Linux distribution and that Linux distribution has a
bug reporting mechanism, then please report the bug there.  To report an issue on our
github repository, use the issue tab and click [New Issue](https://github.com/containers/podman/issues/new/choose)

### Development or latest version

We view this Github repository as an upstream repository for development of the latest
of Podman in the main branch.

When reporting an issue, it should ideally be for the main branch of our repository. Please make
an effort to reproduce the issue using the main branch whenever possible.  An issue can also
be written against the latest released version of Podman.

The term "latest version" refers to our mainline development tree or the
[latest release](https://github.com/containers/podman/releases/latest).

### Bugs vs features

A bug is when something in Podman is not working as it should or has been described.  A
feature or enhancement is when you would like Podman to behave differently.

### Use the issue template when reporting bugs
When you report an issue on the upstream repository, be sure to fill out the entire template.
You must provide the required `podman info` wherever possible as it helps us diagnose
your report.  If possible, always provide a _reliable reproducer_.  This is extremely
helpful for us during triage and bug fixing.  Good examples of a reliable reproducer are:

* Provide precise Podman commands
* Use generic images (like fedora/alpine/debian) where possible to reduce the chance your
container images was a contributor. Abstracting away from the functional purpose helps
diagnoses and reduces noise for us.
* If using the RESTFUL API, providing curl commands as a reproducer is preferred.  Be sure
to provide the same data (or sample data) for things like POST.
* Not requiring the use of a third party tool to reproduce the problem


### Look through existing issues before reporting a new issue
Managing issues is a time consuming processes for maintainers.  You can save us time by
making sure the issue you want to report has not already been reported.  It is appropriate
to comment on the existing issue with relevant information.


### Why was my issue report closed

Issues filed upstream may be closed by a maintainer for the following reasons:

* A fix for the issue has been merged into the main branch of our upstream
repository.  It is possible that the bug was already fixed upstream as well.
* The reported issue is a duplicate.
* The issue is reported against a [distribution that has a bug reporting mechanism](#where-to-report-your-issue)
or paid support.
* The issue was reported using an [older version](#development-or-latest-version) of Podman.
* A maintainer determines the code is working as designed.
* The issue has become [stale](#-stale) and reporters are not responding.
* We were unable to reproduce the issue, or there was insufficient information to reproduce the issue.
* One or more maintainers have decided a feature will not be implemented or an issue will not be fixed.


#### Definitions

[**stale**](https://github.com/containers/podman/issues?q=is%3Aopen+is%3Aissue+sort%3Acreated-asc+label%3Astale-issue): open, but no activity in the last thirty days.

**crickets**: closed due to lack of response from reporting party.

[**jetsam**](https://github.com/containers/podman/issues?q=is%3Aissue+label%3Ajetsam+is%3Aclosed): closed without being implemented. A deliberate decision made in recognition of human limitations.



#### Process

In order of judgment, from least to most.

##### &rarr; stale

Issues are marked with the label *stale-issue* by a [github action](https://github.com/containers/podman/blob/main/.github/workflows/stale.yml) that runs daily at 00:00 UTC. This also triggers an email alert to subscribers on that issue.

Judgment: typically a team member will skim the issue, then decide whether to:

* remove the label; or
* close the issue (see below); or
* do nothing.

This is informal: there is no guarantee that anyone will actually do this.

##### &rarr; crickets

Typically done by a team member after receiving a *stale-issue* email.

Judgment:

* there is not enough information to act on the issue; and
* someone on the team has asked the reporter for more details (like NEEDINFO); and
* the reporter has not responded.

There is no actual *crickets* label. There is no automated way to
find issues that have been closed for this reason.

##### &rarr; jetsam

Last-resort closing of an issue that will not be worked on.

Factors:

* issue has remained open for over sixty days; and
* reporter is responsive, and still wishes to have the issue addressed (as does the team).

Judgment:

* the issue is too difficult or complicated or hard to track down.
* decision should be made by two or more team members, with discussion in the issue/PR itself.

When such an issue is closed, team member should apply the *jetsam* label.
