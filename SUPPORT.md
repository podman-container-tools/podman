# Upstream support of Podman

This Github repository is for the upstream development of Podman and the latest version
of Podman.

The term "latest version" refers to our mainline development tree or the
[latest release](https://github.com/containers/podman/releases/latest).

## Expectations on support

The Podman maintainers provide a "best effort" for the support of Podman.  If are using
Podman from a Linux distribution, please use the Linux distribution's mechanism as support
unless you are willing to reproduce problems on the main branch of our upstream code.

## Operating System and Hardware

Podman is run on a bevy of operating systems and hardware.  Upstream development cannot
possibly support all the combinations and custom environments of our users.

All pull requests (new code) to Podman go through automated testing on the following
combinations:

### Native Podman

| Architecture | Operating System | Distribution |
| :--- | :--- | :--- |
| x86_64 | Linux | Debian (latest) |
| x86_64 | Linux | Fedora (latest) |
| ARM64 | Linux | Fedora (latest) |

### Podman Machine

| Architecture | Operating System | Machine Provider |
| :--- | :--- | :--- |
| x86_64 | Windows 2022 | WSL |
| x86_64 | Windows 2022 | HyperV |
| ARM64 | MacOS | AppleHV |
| ARM64 | MacOS | Libkrun |


For Linux, we test the latest versions of Fedora and Debian.

Operating systems and hardware outside our automated testing is considered "best effort".
In many cases, we are unable to test, triage, and develop for combinations outside what
our automated testing covers.

As of Podman 6, we no longer support Windows 10 nor Intel Macs.  While no code was removed
to drop support of Windows 10, code for Intel Macs was removed and will no longer compile
for that platform.
