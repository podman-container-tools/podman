## OVERVIEW
This document provides details on Podman's threat model. It is meant to provide assistance to security researchers and/or LLM agents performing fuzzing or other analysis to identify security issues in the project.
This document is focused purely on the most recent version of Podman. It covers the operation of Podman in all applicable modes (root and rootless, remote and local).

Supply chain attacks are explicitly *excluded* from this threat model, but should still be reported using the security reporting process.

## CORE TENETS
* User input is always completely trusted. For example, if a user requests that a container be created with all security isolation disabled and the host's root filesystem mounted in, that is *not* a vulnerability, but explicit user intention that must be honored.
* As a corollary, Podman assumes that the user has validated all input to Podman. If the `--env-file` option to `podman run` is passed with a line containing `*` (add all host environment variables to the container), it is assumed that the user has deliberately requested this, and it is not a vulnerability.
* Container isolation is requested by Podman, configured by the OCI runtime, and enforced by the Linux kernel. Podman is responsible for creating a secure default configuration and passing it to the OCI runtime. Issues with this configuration that allow access or modification to the host system are Podman vulnerabilities; all other isolation issues are not.

## TRUST BOUNDARIES
### Container-Host Boundary
Podman can create containers in a variety of ways - `podman create` and `podman run`, `podman play kube`, both Libpod and Docker-compatible REST APIs, restoring checkpoints, Quadlet `.container` units, and several more.
Creating a container establishes the trust boundary between the containerized application and the host, mostly through the use of kernel-level security primitives like SELinux, AppArmor, cgroups, namespaces, capabilities, and Seccomp.
Podman's role is to ensure that sane defaults are chosen for security and isolation.
In many cases, container creation also involves the use of an OCI image.
Use of an untrusted image is *excluded* from the security model as a supply chain problem, but Podman does take steps to limit the blast radius of a malicious image.
Attacks where an image can cause a container created from it to reveal a large part of the host's configuration - e.g. [GHSA-4hq8-gpf5-8p68](https://github.com/podman-container-tools/podman/security/advisories/GHSA-4hq8-gpf5-8p68) - are considered legitimate.

Once containers are operating in steady state, Podman's role in their operation is minimal.
The Linux kernel is responsible for maintaining separation using the primitives selected by Podman.
However, there are three steady-state operations which must operate correctly.
First, Podman must correctly log the container's STDOUT and STDERR to a user-chosen destination.
Attacks which prevent Podman from correctly logging container contents, including denial-of-service attacks stopping the `conmon` process used to log containers, are valid.
Secondly, Podman must allow the user to attach to running containers.
User input or container output being able to crash the `conmon` process as part of attaching to a container is a legitimate denial-of-service attack.
Third, containers can optionally perform regular healthchecks to validate they are running correctly.
Causing a healthcheck to fail is *not* a vulnerability.
For all steady-state operations, any attack that can escape the container to access or modify the host is legitimate.

Also in steady state, Podman can run a number of maintenance operations on the container.
Some of these operations cross the container-host trust boundary.
For example, `podman cp` allows files to be moved into or out of the container.
For all operations with filesystem involvement, ensuring proper sanitization of paths is critical.
Attacks using symlinks to allow these operations to perform unexpected writes outside the container are the most common type of report Podman sees.
In general, any unexpected access or changes to the host from the container during maintenance operations are considered vulnerabilities.
Of particular concern are TOCTOU attacks, where Podman validates and resolves a path in the container, but it is changed by a malicious application in the container before it is used.
The `podman exec` command (and some others, like healthchecks) allow processes to be inserted into a running container.
This process must receive a security context the same as or stricter than the container it was created within, except for explicit user intention.
Much of this work is performed by the OCI runtime (and, correspondingly, many container escape vulnerabilities with `exec` operations are OCI runtime vulnerabilities, not Podman ones).
However, Podman does provide a configuration for how the exec operation should be performed, and errors in that config that would lead to incomplete or inadequate isolation are considered vulnerabilities.

Some kernel filesystems intended to provide generic system information are mounted into Podman containers - for example, `/sys` and `/proc`.
If these grant access to files that have sensitive or excessive information about the host, or can be used to cause changes to the host system, this could be considered a vulnerability.
Complexity in determining what constitutes sensitive information requires that such information disclosures be handled on a case-by-case basis.

Container isolation is managed by the same mechanisms and boundaries as container-host operations, using the same Linux kernel primitives.
Thus, container-to-container isolation is subject to the same rules as container-to-host attacks.
Shared resources requested by the user - for example, shared namespaces in pods - are *excluded* as attack vectors for obvious reasons.

Host to container attacks are not considered a valid attack vector, as the host already has full access to Podman to mutate containers.
The sole exception is cross-user attacks where a user on the system can access or modify containers created and owned by another user.
Podman's core storage directories are considered fully trusted.
The default permissions for these directories are restricted to only allow access by the user running Podman to the greatest extent possible.

### Host-VM Boundary
The `podman machine` suite of commands runs and manages Linux virtual machines to provide access to Linux containers on non-Linux hosts.
These have the ability to access parts of the host via volume mounts.
Ports are forwarded from the host to the VM via `gvisor-tap-vsock`.
The security model for virtual machines depends on the specific provider in use; please consult the virtual machine provider's threat model where applicable.
The ability to access or modify the host outside of volume mounts and user-specified port forwarding is usually treated a vulnerability, subject to the security model of the VM provider and `gvisor-tap-vsock`.
Access or modification to other `podman machine` VMs on the same system is treated identically to access or modification of the host system.
Podman is responsible for downloading and installing the virtual machine image that `podman machine` VMs are created from, unless overridden by the user.
Issues that would lead to an untrusted VM image being pulled are vulnerabilities.

### API Boundary
Podman's provided REST API (including both the Libpod and Docker-compatible endpoints) is considered inherently privileged.
Any user with access to the REST API has the ability to perform arbitrary changes to the host system with the privileges of the user running the Podman API server (via creation of privileged containers mounting content from the host into them).
We strongly recommend *against* exposing the REST API over the network.
When this must be done, TLS certificate authentication to secure access should be considered mandatory.
As such, almost all issues with the REST API are treated as bugs, *not* vulnerabilities.

An exception is issues that bypass API authentication.
The API can optionally be served via TCP with TLS and certificate authentication.
Bypassing certificate authentication to access the API is a vulnerability.

The API server that Podman connects to must be trusted by the user.
Many API operations return data from the server that is used to mutate the client.
Connecting to a malicious API server thus provides abundant opportunity to manipulate the API client without the use of vulnerabilities.
As such, vulnerabilities requiring a malicious API server are *excluded*.

### Host-OCI Registry Boundary
Access to external OCI registries is a trust boundary, but the threat model for this is discussed in a separate document.

### Host-Image Boundary
Building OCI images is a trust boundary, but the threat model for this is discussed in a separate document.

Once an image has been pulled from the registry, it should remain confined in Podman's storage.
Any modification to the host system outside of Podman's storage directories as a result of mounting, inspecting, removing, or performing other operations on an image is considered a vulnerability.

### Secrets: Host to Secrets and Secrets to Containers Boundaries
Podman secrets are not an access control mechanism.
Secrets are stored using multiple potential backends, including `file`, `shell`, and `pass`, but all secret drivers are intended to run on a single node.
As such, the user who wrote the secrets must be able to access them in order to use them.
Some drivers, like the `pass` driver, provide encryption at rest, but once the secret has been decrypted and added to a container, it is available in plaintext and can be accessed by the user running Podman.
Secrets thus do not provide significant security guarantees around access control from the host; most drivers store the secret unencrypted, and the user running Podman can access all secrets at all times without limitation.

Podman commands should not show the value of a secret without explicit user request; e.g. `podman inspect` showing the value of a secret environment variable would be a vulnerability.
Secrets should never be leaked into a created image; e.g. `podman commit` on a container with secrets added should not include the secret in the image.
File secrets can be mounted into containers with security controls (e.g. set UID/GID of the secret).
Those access controls not being respected would be a vulnerability.

### Networking Boundaries: Container-Container and Container-Internet
Podman can create networks, in which various containers are placed.
Networks, by default, are isolated - containers can only access other containers on networks both are members of.
The ability to access containers that are part of a network that the attacking container is not part of would be a vulnerability if network isolation is enabled.
Some networks are internal.
Internal networks have no access to the internet, and gaining internet access from an internal network is a vulnerability.
Podman provides DNS services on appropriately configured networks, allowing containers to look up other containers by name.
This uses the same rules as network access - a container can only get DNS results for containers in shared networks.
Access to container DNS results outside of shared networks would be a vulnerability.
The DNS server runs as an external service, and a denial-of-service attack on the server would be a valid vulnerability.

## OUT OF SCOPE
The following are not considered security issues.
* Layered mounts - i.e., a bind mount of a directory inside a directory that has already been bind-mounted into a container - are not considered a viable vector for vulnerabilities. Layered mounts are impossible to fully secure due to TOCTOU issues - a malicious container can be modifying the image as Podman writes to it.
* All `podman container restore` operations are inherently privileged. Restoring creates a container where all aspects (including security configuration, mounts from the host, and running processes) are controlled entirely by the checkpoint. Similar to the REST API, the ability to run the `restore` command must already be treated as the ability to act with no restrictions, as the user running Podman would.
* The `podman image scp` command is not a security boundary. The user running Podman already has access to all credentials used by `image scp` and can access any remote hosts with those privileges without using the command.
* Malicious volume plugins - volume plugins are fully trusted in Podman's security model. The user is responsible for ensuring that all volume plugins installed are trustworthy.

## ARCHITECTURE
Podman consists of a large number of components, described briefly below.

### Podman CLI (`cmd/podman/`)
The Podman CLI parses user input and passes it through to Libpod via the Infra intermediate layer.
Being primarily responsible for user input parsing, the CLI component has security-critical interactions with file paths (both on the container and host), resource limits, and other user input.

### Quadlet (`cmd/quadlet/`)
Quadlet is an alternative method of creating Podman containers, pods, volumes, and images under systemd management.
Quadlet is a systemd generator, and compiles its input into Podman CLI commands.

### Infra Layer (`pkg/domain/infra`)
The infra layer is an intermediate layer between the Podman CLI and Libpod, allowing the CLI to talk to Libpod directly or remotely via the REST API.

### Podman API Service (`pkg/api/`)
The Podman API service provides a pair of REST APIs (a Podman-specific Libpod API and Docker-compatible API) for interacting with Podman remotely.
By default the API is served via Unix socket (secured via file permissions), but it can also be made available remotely (with optional security via TLS with certificate authentication).

### Machine (`pkg/machine/`)
Machine contains Podman's virtual machine management code which allows Podman to run Linux VMs on Linux, Mac and Windows for the ability to run Linux containers on any operating system.

### Libpod (`libpod/`)
Libpod is the core container management logic of Podman, responsible for managing container lifecycles.
It uses the common, storage, and image libraries to perform detailed setup and teardown operations for containers, pods, and images.

### Container-libs (storage, image, common libraries) ([external](https://github.com/podman-container-tools/container-libs))
These libraries contain the storage backend for container and image layers, code for interacting with OCI registries, and code for managing the lifecycle of OCI images and container networks.
They are stored in a separate repository and threat model information is stored there.

### Conmon ([external](https://github.com/containers/conmon))
Conmon is a minimal supervisor daemon that manages running containers (one Conmon per container), providing logging, post-exit cleanup, and the ability to attach to running containers.
It is stored in a separate repository and threat model information is stored there.

### Buildah ([external](https://github.com/podman-container-tools/buildah))
Buildah provides build services, transforming Containerfiles and Dockerfiles into OCI images.
It is stored in a separate repository and threat model information is contained there.

### Netavark and Aardvark-dns (external - [Netavark](https://github.com/containers/netavark) and [Aardvark](https://github.com/containers/aardvark-dns/))
Netavark performs network configuration on Podman's behalf, including interface creation, firewall configuration, and routing.
Aardvark-dns is a DNS server for Podman-managed networks allowing container name resolution.
Both are stored in separate repositories and threat model information is stored there.
