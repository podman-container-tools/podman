# Podman Container Tools Support

Please read our support policy here: https://github.com/podman-container-tools/community/blob/main/SUPPORT.md.

# Upstream support of Podman

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

### Podman Machine

| Architecture | Operating System | Machine Provider |
| :--- | :--- | :--- |
| x86_64 | Windows 2025 | WSL |
| x86_64 | Windows 2025 | HyperV |
| ARM64 | MacOS | AppleHV |
| ARM64 | MacOS | Libkrun |


For Linux, we test the latest versions of Fedora and Debian.

Operating systems and hardware outside our automated testing is considered "best effort".
In many cases, we are unable to test, triage, and develop for combinations outside what
our automated testing covers. We are however willing to accept PRs to fix issues for
specific platforms that exists in the upstream code as long as support for them was not
removed intentionally.

As of Podman 6, we no longer support Windows 10 nor Intel Macs.  While no code was removed
to drop support of Windows 10, code for Intel Macs was removed and will no longer compile
for that platform.
