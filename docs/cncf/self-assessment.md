# Podman Self-assessment

## Table of contents

* [Metadata](#metadata)
  * [Security links](#security-links)
* [Overview](#overview)
  * [Actors](#actors)
  * [Actions](#actions)
  * [Background](#background)
  * [Goals](#goals)
  * [Non-goals](#non-goals)
* [Self-assessment use](#self-assessment-use)
* [Security functions and features](#security-functions-and-features)
* [Project compliance](#project-compliance)
* [Secure development practices](#secure-development-practices)
* [Security issue resolution](#security-issue-resolution)
* [Appendix](#appendix)

## Metadata

|||| | \-- | \-- | | Assessment Stage | Incomplete | | Software | [https://github.com/containers/podman](https://github.com/containers/podman) | | Security Provider | No | | Languages | Go | | SBOM | [https://github.com/containers/podman/blob/main/go.mod](https://github.com/containers/podman/blob/main/go.mod) |

### Security links

| Doc | url |
| :---- | :---- |
| Security file | [https://github.com/containers/podman/blob/main/SECURITY.md](https://github.com/containers/podman/blob/main/SECURITY.md) |

## Overview

Podman (the POD MANager) is a daemonless container engine for developing, managing, and running OCI containers and pods. Podman emphasizes security by enabling rootless containers, providing fine-grained security controls, and operating without a daemon process.

### Background

Podman is a container management tool that provides a command-line interface for managing containers, images, and pods. Podman runs without a daemon and supports rootless containers, providing an additional layer of security for many use cases.

Key characteristics:

- **Daemonless**: No background daemon process, reducing attack surface
- **Rootless**: Containers can run without root privileges
- **Pod support**: Native support for Kubernetes-style pods
- **Security-focused**: Built with security as a primary concern

Podman is part of the containers ecosystem and integrates with other tools like Buildah, Skopeo, and CRI-O.

### Actors

* **Podman CLI**: The main command-line interface that users interact with. It parses commands and coordinates with other components.

* **libpod library**: The core library that provides container lifecycle management APIs. It handles container creation, execution, and management.

* **Container runtime**: Interfaces with OCI-compliant runtimes (runc, crun) to actually run containers.

* **Image store**: Manages container images and their metadata. Images are stored in a local registry and can be verified for integrity.

* **Container storage**: Manages container filesystems and layers.

* **Network configuration**: Handles container networking, including rootless networking and port forwarding.

* **Systemd integration**: Provides systemd services and pod management.

* **Rootless mode**: Podman can be run in rootless mode, protecting against possible attacks like container escapes allowing control of the entire system.

### Actions

* **Container creation**:

  - Validates container configuration and security options
  - Sets up namespaces and cgroups for isolation
  - Configures security policies (seccomp, SELinux, capabilities)
  - Creates rootless user namespace mapping


* **Image pulling**:

  - Verifies image signatures and checksums
  - Validates image layers and metadata
  - Stores images in a secure local registry


* **Container execution**:

  - Applies security policies (seccomp, SELinux, capabilities)
  - Sets up proper user namespaces for rootless operation
  - Monitors container process and resource usage


* **Pod management**:

  - Creates shared network namespace for pod containers
  - Manages pod-level security policies
  - Coordinates container lifecycle within pods


* **Volume management**:

  - Creates and mounts volumes with appropriate permissions
  - Handles rootless volume mounting
  - Applies SELinux labels to volumes

### Goals

* **Rootless operation**: Enable users to run containers without root privileges, reducing the attack surface and potential for privilege escalation.

* **Daemonless architecture**: Eliminate the daemon process to reduce attack surface and improve security posture.

* **Security by default**: Provide secure defaults for container execution, including appropriate seccomp profiles, SELinux policies, and capability restrictions.

* **OCI compliance**: Maintain compatibility with OCI specifications for containers and images to ensure interoperability.

* **Pod support**: Enable Kubernetes-style pod management with proper security isolation.

### Non-goals

* **Orchestration**: Podman does not provide cluster orchestration capabilities (that's handled by Kubernetes, OpenShift, etc.).

* **Image registry**: Podman does not operate as a centralized image registry, though it can interact with various registries.

* **Container runtime**: Podman does not implement the low-level container runtime (it uses  crun, runc etc..).

* **Storage management**: Podman does not provide distributed storage solutions, only local container storage.

* **Security scanning**: While Podman can work with security scanning tools, it does not provide built-in vulnerability scanning.

## Self-assessment use

This self-assessment is created by the Podman team to perform an internal analysis of the project's security.  It is not intended to provide a security audit of Podman, or function as an independent assessment or attestation of Podman's security health.

This document serves to provide Podman users with an initial understanding of Podman's security, where to find existing security documentation, Podman plans for security, and general overview of Podman security practices, both for development of Podman as well as security of Podman.

This document provides the CNCF TAG-Security with an initial understanding of Podman to assist in a joint-assessment, necessary for projects under incubation.  Taken together, this document and the joint-assessment serve as a cornerstone for if and when Podman seeks graduation and is preparing for a security audit.

## Security functions and features

### Critical Security Components

* **Rootless containers**: Podman's core security feature that allows containers to run without root privileges, significantly reducing the attack surface and preventing privilege escalation attacks.

* **User namespaces**: Provides process isolation by mapping container user IDs to host user IDs, enabling secure rootless operation.

* **Seccomp profiles**: Default seccomp profiles restrict system calls available to containers, preventing many potential attack vectors.

* **SELinux integration**: Automatic SELinux labeling and enforcement for containers, volumes, and images to provide mandatory access control.

* **Capability dropping**: Removes unnecessary Linux capabilities from containers by default, following the principle of least privilege.

* **Daemonless architecture**: Eliminates the daemon process, reducing the attack surface and preventing daemon-based attacks.

### Security Relevant Components

* **Image signing and verification**: Support for container image signatures using GPG/Sequoia and other signing mechanisms.

* **Resource limits**: CPU, memory, and I/O limits to prevent resource exhaustion attacks.

* **Network policies**: Configurable network isolation and firewall rules for container networking.

* **Volume security**: Secure volume mounting with proper permissions and SELinux labels.

* **Pod security policies**: Pod-level security controls that apply to all containers within a pod.


## Project compliance

* **OCI Compliance**: Podman is fully compliant with the Open Container Initiative (OCI) specifications for containers and images.

* **CIS Docker Benchmark**: Podman provides security benchmarking tools that align with the Center for Internet Security (CIS) Docker Benchmark.

* **FIPS 140-2**: Podman supports FIPS 140-2 compliant cryptographic modules when running on FIPS-enabled systems.

* **SELinux**: Full integration with SELinux for mandatory access control compliance.

* **AppArmor**: Support for AppArmor profiles for additional access control.

## Secure development practices

### Development Pipeline

* **Code Review Process**: All code changes typically require review by two maintainers before merging. Critical security changes may require multiple reviews. The project uses GitHub pull requests for all contributions.

* **Automated Testing**: Comprehensive test suite including unit tests, integration tests, and security-focused tests that run on every pull request. A comprehensive e2e and system test suite is run in CI on every PR and also on a nightly basis.

* **Security Scanning**: Automated vulnerability scanning of dependencies using tools like Dependabot and GitHub Security Advisories. All medium and higher severity exploitable vulnerabilities are fixed in a timely way after they are confirmed.

* **Static Analysis**: Code quality and security analysis using golangci-lint which is run on every PR, ensuring testing is done prior to merge.

* **OpenSSF Best Practices Compliance**: Podman has achieved a [passing OpenSSF Best Practices badge](https://www.bestpractices.dev/projects/10499), demonstrating adherence to security best practices including proper licensing, contribution guidelines, and security processes.

### Communication Channels

* Podman user room: [\#podman:fedoraproject.org](https://matrix.to/#/#podman:fedoraproject.org)

* Podman dev room: [\#podman-dev:matrix.org](https://matrix.to/#/#podman-dev:matrix.org)

* **Inbound**:

  - GitHub Issues for bug reports and feature requests
  - GitHub Discussions for community questions
  - Security issues via the security mailing list
  - Mailing lists for formal discussions
  - Clear contribution guidelines documented in [CONTRIBUTING.md](https://github.com/containers/podman/blob/main/CONTRIBUTING.md)

* **Outbound**:

  - Release announcements via the [official Podman website](https://podman.io), GitHub releases, and the Podman mailing list
  - Documentation updates and blog posts
  - Conference presentations and talks
  - Project website at [podman.io](https://podman.io) with comprehensive documentation

### Ecosystem

Podman is a critical component of the cloud-native ecosystem:

* **Container Ecosystem**: Integrates with Buildah for building containers, Skopeo for image and registry operations.

* **Development Tools**: Widely used in development environments as a secure alternative to Docker.

* **CI/CD Pipelines**: Used in  CI/CD systems for testing containerized applications.

## Security issue resolution

### Responsible Disclosures Process

* **Reporting**: Security vulnerabilities should be reported by email as documented in the [SECURITY.md](https://github.com/containers/podman/blob/main/SECURITY.md) file.

* **Response Time**: The team commits to responding to vulnerability reports within 48 hours. All medium and higher severity exploitable vulnerabilities are prioritized as a matter of general practice.

* **Coordination**: For critical vulnerabilities, ct Security team coordinates with downstream projects to file bug trackers for downstreams (Fedora / RHEL).

* **Credit**: Security researchers who responsibly disclose vulnerabilities are credited in security advisories and release notes.

* **Public Disclosure**: Vulnerabilities are disclosed by the project maintainers with appropriate embargo periods for critical issues, following industry best practices for responsible disclosure.

### Vulnerability Response Process

* **Triage**: Security reports are triaged by the project maintainers and assigned severity levels (Critical, High, Medium, Low) using CVSS scoring where applicable.

* **Investigation**: The team investigates the vulnerability, determines impact, and develops fixes. All medium and higher severity exploitable vulnerabilities discovered through static or dynamic analysis are fixed in a timely way after they are confirmed.

* **Fix Development**: Security fixes for embargoed CVEs are developed in private repositories to prevent premature disclosure.

* **Disclosure**: Vulnerabilities are disclosed by the project maintainers with appropriate embargo periods for critical issues. The project follows industry best practices for coordinated vulnerability disclosure.

### Incident Response

* **Detection**: Tools like renovate automatically update dependencies, including fixes for security issues.

* **Containment**: Immediate steps are taken to contain and mitigate the impact of security incidents. If the system and e2e tests point out any issues in the development phase, those get fixed before any code is merged.

## Appendix

### Known Issues Over Time

* See [this NIST Vulnerability Database list](https://nvd.nist.gov/vuln/search#/nvd/home?vulnRevisionStatusList=published&offset=0&rowCount=50&keyword=podman&resultType=records) for CVEs to date. This includes issues in the Go toolchain and dependencies used by Podman.
(Four of the entries as of the date of this writing aren't directly related to Podman but contain Podman in the search terms.)

### OpenSSF Best Practices

* **Current Status**: Podman has achieved a [passing OpenSSF Best Practices badge](https://www.bestpractices.dev/projects/10499) (100% compliance), demonstrating adherence to security best practices.

* **Key Achievements**:

  - Comprehensive project documentation and contribution guidelines
  - Robust security testing and analysis processes
  - Clear vulnerability disclosure and response procedures
  - Strong development practices with code review and automated testing
  - Proper licensing and project governance

### Case Studies

* List of companies and organizations using / shipping Podman [https://github.com/containers/podman/blob/main/ADOPTERS.md](https://github.com/containers/podman/blob/main/ADOPTERS.md)

### Related Projects / Vendors

* **Buildah**: A tool that facitiliates building OCI container images.

* **Skopeo**: A command line utility to perform various operations on container images and image repositories like copying an image, inspecting a remote image, deleting an image from an image repository.
