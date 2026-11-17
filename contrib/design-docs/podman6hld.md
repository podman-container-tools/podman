# Podman 6 High Level Design

#

The following document describes the high level design of Podman 6. Podman 6 will be generally available in Spring of 2026.
In this document, I try to summarize:

* Breaking changes in API or CLI for Podman
* New functionality that is non-trivial work.
* Impacts to other components (like netavark, common)
* Deprecations

The content in this document are proposals generally. If a proposal is objectionable to a considerable number of people or
stakeholders, then it will be reconsidered and decisions will be made consensually.  Also, new content certainly may be
added during review.  Moreover, some of the features described below may have their own design documents where more detail
will be provided.

[**Branching, Schedules, Podman 5**](#Branching-Schedules-Podman-5)

[**Deprecations**](#deprecations)

[**Configuration files**](#configuration-files)

[**Changes in Defaults for Podman**](#changes-in-defaults-for-podman)

[**Libkrun**](#libkrun)

[**New Conmon**](#new-conmon)

[**Pasta**](#pasta)

[**Machine**](#machine)

[**Netavark**](#netavark)

[**Podman machine updates**](#podman-machine-updates)

[**Issues in containers/podman with "6.0" Label**](#issues-in-containers/podman-with-"6.0"-label)


# Branching, Schedules, Podman 5

We have spent considerable time discussing when the Podman main branch should be open for Podman 6.  To understand this,
we must understand the schedule for the upcoming months.  We still have active development occurring for Podman 5 series
releases.  We have the following Podman 5 minor released scheduled:

* Podman 5.7 - November 2025 (first week)
* Podman 5.8 - February 2026

With Podman 5.7 releasing in early November, we will create a 5.7 branch when releasing the first release candidate.  Current
plans project RC1 on the week of October 20th.  Given a short period for things to settle down, we can then branch
Podman main for 6.0 development the week of October 27th.

Podman 5.8 will focus on critical bug fixes and user requirements. As such, it will be derived from the Podman 5.7
branch in February.

Podman 6 is to become generally available in the Spring of 2026.  The exact release date is still to be determined,
however, the intent is to release it in Fedora 44 so likely completion date is in the March 2026 timeframe depending on
the allowances Fedora permits us.

# Deprecations

The following previously announced deprecations will be made final.  Some have already been communicated to users,
whereas some will be new to Podman users.

## **Slirp4netns**

[slirp4netns](https://github.com/rootless-containers/slirp4netns) was the original network utility for rootless Podman and has been replaced by [Pasta](https://passt.top/passt/about/).  Podman
has been able to use Pasta since version 4.4.  In Podman 5.0, Pasta became the default rootless option and a deprecation
notice for slirp4netns was made in Podman 5.

## **CNI plugins**

[CNI plugins](https://github.com/containernetworking/plugins) were the original network backend for Podman.  It has been replaced by netavark.  In Podman 4.0, new
instances of Podman used netavark while existing still used CNI.  In Podman 5, CNI was deprecated and not available for
most.  This work will remove the actual code from the Podman project.  CNI configuration options must also be removed.

## **CGroups V1**

In Podman 6, we will no longer support cgroupsv1 and relevant code shall be removed. Additionally, we need to remove
system and e2e tests that were implemented specifically for cgroupsv1 and cgroupsv1 functionality.

## **BoltDB**

[BoltDB](https://github.com/boltdb/bolt) was the primary database used by Podman to store container information.  Due to lack of upstream upkeep
and an ugly bug, we added SQLite3 support.  SQLite has been the default since 4.8.  We need to add a
[deprecation notice for Podman 5.7](https://issues.redhat.com/browse/RUN-3343), and remove this as an option for 6.0.

We must also remove code in Podman’s test suite that specifically tests BoltDB.  There will be a small amount of work,
presumably cleanup, in [container-libs](https://github.com/containers/container-libs) as well.

## **--network-cmd-path**

Only used for slirp4netns so should be removed as well.

## **Windows 10**

Microsoft has [announced an EOL date for Windows 10 to be October 2025](https://www.microsoft.com/en-us/windows/end-of-support?r=1).  Because we are so dependent on Microsoft’s
work and support of WSL, it makes sense that we do not support Windows 10 with Podman 6.  It should be noted that we
will do nothing to stop it from working.

## **Intel-based Macs** {#intel-based-macs}

In Podman 6, Podman will no longer support Intel based Macs.  You can read a [wider description and justification](#drop-intel-based-mac-support) below.

# Configuration files {#configuration-files}

Podman 6 will address a long standing problem with our configuration files largely centered around the remote client.
When the remote client was introduced, [container-lib/common](https://github.com/containers/container-libs/tree/main/common) already existed. While we did introduce remote client
related content to the containers.conf files, we did not take an overall look at how the remote client would impact our
configuration files.  With the popularity of our Windows and Mac clients, this became more exacerbated and the
introduction of the “machine” function only made that worse.  In Podman 6, we must make containers-common more aware
of our environment.

Users are rightfully confused by which files they should edit and for what function.  Lets look at two examples:
*additional_stores* and the (machine) *provider* setting.  Today, both of these settings are valid in the
*containers.conf* files.  But if you were to specify the *additional_stores* in the *containers.conf* that resides
on the Mac client, it would have no impact.  It needs to be specified in the virtual machine where Podman actually
runs.  And conversely, specifying the machine provider in the virtual machine will have absolutely no impact on
provider selection on the client.

We are still working on proposals on how to handle this and are using real input from users.  The most likely action
we take is making a client/server approach to our configuration files.  This does present problems with migration of
current users and perhaps knowing what should go into each file.  Perhaps our configuration file compiler can warn when
finding things that are not applicable on the client machine as an example.

We will also need to plan on how this new change impacts the use of the recently introduced “global rootless”
configuration file.

We should also look into unifying the parsing of the `containers.conf` and `storage.conf` files. Our configuration
files behave very differently in sometimes surprising ways:

* defaults
* root and rootless
* layering of configuration files.

Resolving this will require breaking changes in how `storage.conf` is parsed but should significantly reduce customer
confusion about configuration file handling. Ideally, the parsing logic adopted for `containers.conf` as part of this
rework can be structured as a library that can easily be applied to `storage.conf` and any other configuration file we
feel the need to add. (policy.json, registries.conf, registries.d, certs.d, mounts.conf, auth.json)

We must also normalize inheritance of configuration files–in particular their order.

And finally, in talking with Enterprise users, I am frequently asked about deploying Podman in a customized fashion.  For
example, suppose a company wants to ensure all podman users use the corporate image registry as a first source.  This
quickly results in a conversation around enforcement versus customization that can be overridden by the user.
We should continue to weigh these problems as we implement our new configuration file approach.

## **Buildah, Skopeo**

The changes in our configuration files will most likely also impact [Buildah](https://github.com/containers/buildah), [Skopeo](https://github.com/containers/skopeo), and to that extent
Podman-Py (see below).  These changes may be breaking in nature and would likely result in a major version bump.

## **Podman-py**

[Podman-py](https://github.com/containers/podman-py) may parse configuration files for some features. This will need to be confirmed.   Generally, it would
be ideal that podman-py not read configuration files and if this can be avoided, the team would prefer it.  If reading
the configuration file is mandatory, we should investigate how we plan to keep common from breaking podman-py in the
future.  Should the reading of the configuration files be done in common so it can be tested for regressions?

# Changes in Defaults for Podman

## **Libkrun**

Podman Desktop (PD) has recently [changed the default provider](https://github.com/podman-desktop/podman-desktop/pull/12786) from `applehv` to [`libkrun`](https://github.com/containers/libkrun).  This change
will be (has been) part of the Podman-Desktop-1.21 release.  The primary reason for the change was to embrace the
strategy around AI and assumption that PD users will want GPU access to run AI workloads.  This seems a reasonable
assumption.

Having a different default provider between Podman and Podman Desktop is not ideal.  But Podman avoids the changing of
defaults in minor releases (*Y versions* ) due to semantic versioning.  Podman 6.0 will follow PD and change the
default to `libkrun`.  This change will only impact “new” machine usages.

# New Conmon

We are currently evaluating the development of a new [conmon](https://github.com/containers/conmon) to address several user enhancements and
improvements.  Addressing the gaps of the current conmon is  key to eliminating where we need to be in the Podman 6
timeframe.  This document also proposes the adoption and requirements of conmon within Podman versions.

# Pasta

Port forwarding in customer networks requires changes in podman and pasta
[https://github.com/containers/podman/issues/8193](https://github.com/containers/podman/issues/8193)

# Machine

In addition to the change in the default provider for Macs, there are some other initiatives we should consider.

## **Obfuscate Machine “Provider”**

The strict adherence to interacting with machines by provider can be relaxed.  On platforms like Windows and Mac, users
have a choice of providers.  They may well have legitimate reasons to use machines in either provider.  This is most
true on the Mac where `libkrun` and `applehv` don’t require special installations like WSL and HyperV.

In Podman 6, we will no longer operate strictly by provider.  And for Macs, we are going to set the default provider
for new machines to be libkrun.  Now suppose the user has two defined machines, `myapplehv` and `mylibkrun`
respectively and with respective providers.  The user should no longer have to make a configuration file change or an
environment variable definition to interact with the machines.  Consider the following (where libkrun is the defined
provider):

`$ podman machine stop myapplehvmachine`

This command should work.

## **Stop, start, remove, inspect, cp, os, set ssh**

These commands should work regardless of provider.  If the user provides a name, Podman should resolve that name to a
provider and take action accordingly.

## **List**

Podman should list all machines regardless of provider. We have this function today but it is triggered with a flag.
This should now become the default.

## **Info**

The `info` command should be altered as well. It should likely report available/used providers and perhaps machine
counts.

## **Init**

The `init` command should now have a `--provider` switch allowing the user to override the defaults.

## **Machine image cache**

Today, a provider will only look into its own cache storage for the machine image.  Mac providers are the only two that
can share a cached image.  And there is no guarantee this remains so in the future.  As such, each provider will need
to pull its own image and in the case of Macs, that (today) will be the same image in two locations.

##

## **Drop Intel-based Mac support**

Podman 6 will no longer support Intel based Macs.  The last Intel Macs were essentially released side by side with
Apple Silicon Macs in 2020.  Apple has stated they will be EOL in 2028, but for the latest models, Tahoe will be the
last supported operating system.

According to Podman Desktop telemetry data, the Mac Intel numbers are already small and dwindling steadily.  As of
late August 2025, Intel Macs users account for 3.7% of Mac users. This is down from 4.1% in May 2025, which was down
from 6.1% in February 2025.  While these numbers do not reflect all of Podman users, we believe they would mirror
Podman usage as well.

Dwindling user bases are a significant factor in deciding to no longer support Intel Mac use.  Among other compelling
reasons, we also considered:

* *No CI* system available to us has Intel Mac support; therefore, end-to-end and system tests cannot be run on them.
* This also includes [podman-machine-os](https://github.com/containers/podman-machine-os).
* No Podman maintainer has *access to an Intel Mac* for development or debug activities.
* Intel Macs *do not support many of our strategies*, specifically GPU driven AI.
* Switching to `libkrun` as the default provider will require *maintainer effort to implement code* dealing with architectures within platforms. The `libkrun` project does not support Intel Macs.
* We assume *Apple will not be interested in fixing problems* on Intel Macs, where it may also be on older operating system versions.

# Netavark

It’s been two major Podman releases since [netavark](https://github.com/containers/netavark) became available.  While it has had many features and
enhancements introduced in that time frame, netavark needs some TLC.

## **Consolidate network create**

Firstly, we have information about Podman networks kind of spread between netavark and Podman.  Here we intended to
push the network creation to netavark instead of Podman directly.

Netavark would accept a new cli command “create” which accepts the config json on stdin and validates + adds any
missing options and then returns the final json on stdout. Podman still manages storing the files.

## **Remove iptables support**

We also intend to remove IP tables support as the community has frankly moved on to nftables.
Fedora/RHEL and Debian have already switched to nftables as default without any major problem.

We might want to check with some other distributions, Arch, openSUSE and let them know ahead of time.

## **Preserve network order**

We want to preserve network order for containers. As such, an internal structure used must change from a map to
slice/array.  Maps do not have a logical way to determine order whereas slices are precisely for this purpose.
This change will impact both netavark and Podman.

It requires both a json schema change for the communication between podman and nv as well as a podman DB change to
store the networks with an order.  While doing all of this work, we may need to keep backwards compatibility for
existing containers in mind.

## **Combine netavark and aardvark repositories**

Having different repositories for each project, especially with their strong reliance on each other, no longer makes
sense.  We will take this opportunity to combine the two repos under netavark.  The aardvark binary will remain as it
does today. And for packaging, at least on Fedora and RHEL, the two RPMS shall remain.  This is proposed as only an
upstream change.  If these projects are ever donated to the CNCF, having a single repository will be easier as well.

# Podman machine updates

Updating machine images has been a problem area for Podman machine for some time.  We implemented
`podman machine os apply` quite some time ago.  That command allows users to point to a valid OCI container image and
update via ostree easily.  Updates are complicated by the fact that we cannot use OCI with WSL due to how WSL is
designed.

Because the WSL machine image is based on a custom Fedora installation, I think allowing `dnf update` to handle
updates will cause too many unpredictable cases.  For example, when a new Podman release is created, a new machine
image is also created.  That machine image is run through our automated (functional and regression) tests.  Now suppose
two weeks later, the user executes updates.  While you may get a new Podman, you will also get new supporting and
operating system updates.  This new combination of updates will most likely differ from whatever that Podman version
was tested with, thereby introducing the possibility of regressions.

## **Future**

Do we process in this manner where all hypervisors but WSL are derived from Fedora CoreOS or can we find a unifying
approach?  And can a unified approach provide us with a similar experience of controlled updates and rollback
possibilities.  The answers to these questions will dictate the future of machine updates.

## **Notifications**

Regardless of our approach, Podman needs to be able to inform users of the following scenarios:

* Podman client and server mismatches.  I recommend only major and minor versions (X.Y) be raised.
* A new patch level version of the machine image is available.

## **Machine Update (Patch level updates)**

We need a machine command that updates the machine image within the minor version.  The command should ensure there is
a new (different) machine version and then updates itself.  My assumption is that a reboot would be required.  If the
command is run and no update is available, the command should result in a zero return code.

## **Machine update (Minor version updates)**

In the case where the Podman client is newer (on the minor version) than the machine image’s Podman version, Podman
should be able to upgrade its machine image successfully.

## **Issues in containers/podman with "6.0" Label**

We have a number of issues in [github.com/containers/podman](http://github.com/containers/podman) with the `6.0` label.  The purpose of the label is to
signify that fixing the bug or implementing a feature would require a major version bump.  We must consider these
issues and determine if they should be included in Podman 6.0.

| Issue # | Title | State | Type | Author |
| :---- | :---- | :---- | :---- | :---- |
| [#26005](https://github.com/containers/podman/issues/26005) | Support default refresh time for podman ps -w when no argument is provided | Open | Feature | dpateriya |
| [#24597](https://github.com/containers/podman/issues/24597) | CLI UX Consistency Pruning Volumes | Open | Bug | damaestro |
| [#23984](https://github.com/containers/podman/issues/23984) | Default route confusion when using multiple `--network` options with `macvlan` and `bridge` networks | Open | Bug | codedump |
| [#23824](https://github.com/containers/podman/issues/23824) | podman inspect should return null on some value instead of 0 | Open | Bug | chisaato |
| [#23353](https://github.com/containers/podman/issues/23353) | "podman machine start" should start default machine "defaultmachine" parameter from "podman machine info" | Open | Feature | kgfathur |
| [#21847](https://github.com/containers/podman/issues/21847) | podman ps --format='{{json .Labels}}' incompatible with docker CLI output (string vs map of strings) | Open | Bug | Romain-Geissler-1A |
| [#19717](https://github.com/containers/podman/issues/19717) | Fail hard instead of pulling non-native image architecture | Open | Feature | praiskup |
| [#14239](https://github.com/containers/podman/issues/14239) | Podman API takes a SpecGenerator rather than CreateOptions, why? | Open | Planning | cdoern |
| [#13383](https://github.com/containers/podman/issues/13383) | looking up image by short name should not fallback to match *any* name | Open | Feature | ianw |
| [#26621](https://github.com/containers/podman/issues/26621) | podman pull for different platforms replaces images | Open | Bug | RonnyPfannschmidt |
