# Release Notes

## 5.6.0
### Features
- A new set of commands for managing Quadlets has been added as `podman quadlet install` (install a new Quadlet for the current user), `podman quadlet list` (list installed Quadlets), `podman quadlet print` (print the contents of a Quadlet file), and `podman quadlet rm` (remove a Quadlet). These commands are presently not available with the remote Podman client - we expect support for this to arrive in a future release.
- Quadlet `.container` units can now specify environment variables without values in the `Environment=` key, retrieving the value from the host when the container is started ([#26247](https://github.com/containers/podman/issues/26247)).
- Quadlet `.pod` units now support two new keys, `Label=` (to set labels on the created pod) and `ExitPolicy=` (to set exit policy for the created pod) ([#25961](https://github.com/containers/podman/issues/25961) and [#25596](https://github.com/containers/podman/issues/25596)).
- Quadlet `.image` units now support a new key, `Policy=`, to set pull policy for the image (e.g. pull always, pull only if newer version available) ([#26446](https://github.com/containers/podman/issues/26446)).
- Quadlet `.network` units now support a new key, `InterfaceName=`, to specify the name of the network interface created.
- The `podman machine init` command now supports a new option, `--swap`, enabling swap in the created virtual machine and setting it to a given size (in megabytes) ([#15980](https://github.com/containers/podman/issues/15980)).
- The `--mount` option to `podman create` and `podman run` now supports `dest=` as a valid alias for `destination=`.
- The `podman kube play` command can now restrict container execution to specific CPU cores and specific memory nodes using the `io.podman.annotations.cpuset/$ctrname` and `io.podman.annotations.memory-nodes/$ctrname` annotations ([#26172](https://github.com/containers/podman/issues/26172)).
- The `podman kube play` command now supports the `lifecycle.stopSignal` field in Pod YAML, allowing the signal used to stop containers to be specified ([#25389](https://github.com/containers/podman/issues/25389)).
- The `podman artifact` suite of commands for interacting with OCI artifacts is now available in the remote Podman client and the bindings for the REST API.
- The `podman volume import` and `podman volume export` commands are now available in the remote Podman client ([#26049](https://github.com/containers/podman/issues/26409)).
- The `--build-context` option to `podman build` is now supported by the remote Podman client ([#23433](https://github.com/containers/podman/issues/23433)).
- The `podman volume create` command now accepts two new options, `--uid` and `--gid`, to set the UID and GID the volume will be created with.
- The `podman secret create` command now has a new option, `--ignore`, causing the command to succeed even if a secret with the given name already exists.
- The `podman pull` command now has a new option, `--policy`, to configure pull policy.
- The `--mount type=artifact` option to `podman create`, `podman run`, and `podman pod create` now allows the filename of the artifact in the container to be set using the `name=` option (e.g. `podman run --mount type=artifact,name=$NAME,...`).
- The `--tmpfs` option to `podman create` and `podman run` now allows a new option, `noatime`, to be passed (e.g. `podman run --tmpfs /run:noatime ...`) ([#26102](https://github.com/containers/podman/issues/26102)).
- The `podman update` command now has a new option, `--latest`, to update the latest container instead of specifying a specific container ([#26380](https://github.com/containers/podman/issues/26380)).
- A new command, `podman buildx inspect`, has been added to improve Docker compatibility ([#13014](https://github.com/containers/podman/issues/13014)).

### Breaking Changes
- Rosetta support in `podman machine` VMs has been disabled by default due to issues with newer Linux kernels. These issues have been addressed in the Tahoe beta, and we plan on re-enabling support for Rosetta in a future Podman release once the fix is in wider circulation. You can find more details [here](https://blog.podman.io/2025/08/podman-5-6-released-rosetta-status-update/).

### Changes
- In preparation for a planned removal of the BoltDB database in Podman 6.0, a warning has been added for installations still using BoltDB. These warnings are presently not visible by default, which will happen in Podman 5.7.
- The `podman artifact` suite of commands for interacting with OCI artifacts is now considered stable.
- For users running `podman machine` VMs using the `libkrun` provider on an M3 or newer host running macOS 15+, nested virtualization is enabled by default.
- When creating `podman machine` VMs on Windows using the WSL v2 provider, images are now pulled as artifacts from `quay.io/podman/machine-os`, matching the behavior of other VM providers.
- Signal forwarding done by the `--sig-proxy` option to `podman run` and `podman attach` is now more robust to races and no longer forwards the `SIGSTOP` signal.
- The `podman system check --quick` command now skips checking layer digests.
- Podman on Windows using the WSLv2 provider now prefers the WSL executable in `C:\Program Files\Windows Subsystem for Linux\wsl.exe` over the one in `WindowsApps`, avoiding common “access denied” issues ([#25787](https://github.com/containers/podman/issues/25787)).
- The `--mount type=artifact` option to `podman create`, `podman run`, and `podman pod create` now mounts artifacts containing a only a single blob as a file at the given destination path if the path does not exist in the image.
- The `podman volume export` command now refuses to export to `STDOUT` if it is a TTY ([#26506](https://github.com/containers/podman/issues/26506)).
- When generating Quadlet units with options known to be problematic when used with Podman, such as `User=`, `Group=`, and `DynamicUser=` in the `[Service]` section of a unit, Quadlet will now warn the user of the potential incompatibility ([#26543](https://github.com/containers/podman/issues/26543)).

### Bugfixes
- Fixed a bug where the `--security-opt unmask=` option to `podman create` and `podman run` did not allow comma-separated lists of paths to be passed, instead only allowing a single path.
- Fixed a bug where stopping a Podman container could unintentionally kill non-Podman processes if the PID of an exec session started inside the container was reused for a new process while the container was running ([#25104](https://github.com/containers/podman/issues/25104)).
- Fixed a bug where `podman machine init` could fail if run in a Podman container ([#25950](https://github.com/containers/podman/issues/25950)).
- Fixed a bug where `podman machine` VMs would sometimes receive incorrect timezone information.
- Fixed a bug where `podman machine` VMs created with a custom username would not have lingering enabled.
- Fixed a bug where the `podman machine init` command on Windows when using the WSL 2 provider did not reliably determine if WSL was installed ([#25523](https://github.com/containers/podman/issues/25523)).
- Fixed a bug where the name of Quadlet `.pod` units that did not specify the `PodName=` key was set incorrectly ([#26062](https://github.com/containers/podman/issues/26062)).
- Fixed a bug where Quadlet `.container` units joining a pod specified in a `.pod` unit would fail as the pod name was set incorrectly when creating the container ([#26105](https://github.com/containers/podman/issues/26105)).
- Fixed a bug where Quadlet would not generate `RequiresMountsFor` when mounting a `.volume` unit with `Type=bind` set into a container ([#26125](https://github.com/containers/podman/issues/26125)).
- Fixed a bug where Quadlet dropin files were not correctly overwritten by new dropin files with the same name further along the hierarchy if the two dropin files did not share a parent directory ([#26555](https://github.com/containers/podman/issues/26555)).
- Fixed a bug where Quadlet would sometimes not print warnings when failing to parse units ([#26542](https://github.com/containers/podman/issues/26542)).
- Fixed a bug where Quadlet `.pod` files did not include the last `Environment=` key in the `[Service]` section in the generated systemd service ([#26521](https://github.com/containers/podman/issues/26521)).
- Fixed a bug where starting a container with already-running dependencies would fail.
- Fixed a bug where OCI hooks in a directory specified with `--hooks-dir` would fail to run when containers were restarted ([#17935](https://github.com/containers/podman/issues/17935)).
- Fixed a bug where the `--mount` option to `podman create` and `podman run`  required the `type=` option to be specified, instead of defaulting to `volume` when it was not present ([#26101](https://github.com/containers/podman/issues/26101)).
- Fixed a bug where the `podman kube play` command would fail on Windows when specifying an absolute path to YAML files ([#26350](https://github.com/containers/podman/issues/26350)).
- Fixed a bug where the `--security-opt seccomp=` option to `podman create`, `podman run`, and `podman pod create` could error on Windows when given a path to a Seccomp profile ([#26558](https://github.com/containers/podman/issues/26558)).
- Fixed a bug where the `--blkio-weight-device`, `--device-read-bps`, `--device-write-bps`, `--device-read-iops`, and `--device-write-iops` options to `podman create` and `podman run` incorrectly accepted non-block devices.
- Fixed a bug where the `podman build` command handled the `--ignorefile` option differently from the `buildah bud` command ([#25746](https://github.com/containers/podman/issues/25746)).
- Fixed a bug where the `podman rm -f` command could return an error when trying to remove a running container whose `conmon` process had been killed ([#26640](https://github.com/containers/podman/issues/26640)).
- Fixed a bug where the `podman inspect` command did not correctly display log size for containers when `log_size_max` was set in containers.conf.

### API
- A full set of API endpoints for interacting with artifacts has been added, including inspecting artifacts (`GET /libpod/artifacts/{name}/json`), listing all artifacts (`GET /libpod/artifacts/json`), pulling an artifact (`POST /libpod/artifacts/pull`), removing an artifact (`DELETE /libpod/artifacts/{name}`), adding an artifact (or appending to an existing artifact) from a tar file in the request body (`POST /libpod/artifacts/add`), pushing an artifact to a registry (`/libpod/artifacts/{name}/push`), and retrieving the contents of an artifact (`GET /libpod/artifacts/{name}/extract`).
- The Compat Create endpoint for Containers now accepts a new parameter, `HostConfig.CgroupnsMode`, to specify the cgroup namespace mode of the created container.
- The Compat Create endpoint for Containers now respects the `base_hosts_file` option in `containers.conf`.
- The Compat System Info endpoint now returns a new field, `DefaultAddressPools`.
- The Compat System DF endpoint has removed the deprecated `BuilderSize` field.
- The Compat Ping endpoint now sets `Builder-Version` to `1` to match Docker installs that do not include BuildKit.
- The Compat List endpoint for Images now returns the `shared-size` field unconditionally, even if the `shared-size` query parameter was not set to true. If not requested through query parameter, it is set to `-1`. This improves Docker API compatibility.
- The Compat Inspect endpoint for Images now no longer returns the deprecated `VirtualSize` field when Docker API version 1.44 and up is requested.
- Fixed a bug where the Compat Delete API for Containers would remove running containers when the `FORCE` parameter was set to true; Docker only removes stopped containers ([#25871](https://github.com/containers/podman/issues/25871)).
- Fixed a bug where the Compat List and Compat Inspect endpoints for Containers returned container status using Podman statuses instead of converting to Docker-compatible statuses ([#17728](https://github.com/containers/podman/issues/17728)).
- Fixed a bug where healthchecks that exceeded their timeout were not properly terminated; they now receive SIGTERM, then SIGKILL after a delay, if their timeout is exceeded ([#26086](https://github.com/containers/podman/pull/26086)).
- Fixed a bug where `application/json` responses would be HTML escaped, mutating some responses (e.g. `<missing>` becoming `\u003cmissing\u003e` in image history responses) ([#17769](https://github.com/containers/podman/issues/17769)).

### Misc
- Quadlet now no longer uses container/pod ID files when stopping containers, but instead passes the name of the container/pod directly to `podman stop`/`podman pod stop`.
- When building Podman via Makefile, it will now attempt to dynamically link sqlite3 if the library and header are installed locally. This and other optimizations should result in a significant reduction in binary size relative to Podman 5.5.x. Packagers can use the `libsqlite3` build tag to force this behavior when not using the Makefile to build.
- Updated Buildah to v1.41.3
- Updated the containers/common library to v0.64.1
- Updated the containers/storage library to v1.59.1
- Updated the containers/image library to v5.36.1

## 5.5.2
### Security
- This release addresses CVE-2025-6032, in which the TLS connection used to pull VM images for `podman machine` was, by default, not validated, allowing connections to servers with invalid certificates by default and potentially allowing a Man in the Middle attack.

### Bugfixes
- Fixed a bug where Podman could panic after a reboot on systems with pods containing containers ([#26469](https://github.com/containers/podman/issues/26469)).

## 5.5.1
### Bugfixes
- Fixed a bug where containers mounting a volume to `/` could overmount important directories such as `/proc` causing start and/or runtime failures due to an issue with mount ordering ([#26161](https://github.com/containers/podman/issues/26161)).
- Fixed a bug where Quadlet `.pod` units could fail to start due to their storage not being mounted ([#26190](https://github.com/containers/podman/issues/26190)).
- Fixed a bug where containers joined to a network with DNS enabled would not include the host's search domains in their `resolv.conf` ([#24713](https://github.com/containers/podman/issues/24713)).
- Fixed a bug where the `--dns-opt` option to `podman create`, `podman run`, and `podman pod create` would append options to the container's `resolv.conf`, instead of replacing them ([#22399](https://github.com/containers/podman/issues/22399)).
- Fixed a bug where the `podman kube play` command would add an empty network alias for containers created with no name specified, causing Netavark to emit extraneous warnings.
- Fixed a bug where the `podman system df` command would panic when one or more containers were created using a root filesystem (the `--rootfs` option to `podman create` and `podman run`) instead of from an image ([#26224](https://github.com/containers/podman/issues/26224)).
- Fixed a bug where the `log_tag` field in `containers.conf` would override the `--log-opt tag=value` option to `podman create` and `podman run` ([#26236](https://github.com/containers/podman/issues/26236)).
- Fixed a bug where the `podman volume rm` and `podman volume inspect` commands would incorrectly handle volume names containing the `_` character when the SQLite database backend was in use ([#26168](https://github.com/containers/podman/issues/26168)).
- Fixed a bug where the Podman remote client on Windows was unable to mount local folders into containers using overlay mounts (`-v source:destination:O`) ([#25988](https://github.com/containers/podman/issues/25988)).

### API
- Fixed a bug in the Libpod Create API for Containers where rlimits specified with a value of `-1` were causing errors, instead of being interpreted as the maximum possible value ([#24886](https://github.com/containers/podman/issues/24886)).
- Fixed a bug in the Compat Create API for Containers where specifying an entrypoint of `[]` (an empty array) was ignored, instead of setting an empty entrypoint ([#26078](https://github.com/containers/podman/issues/26078)).

### Misc
- Updated Buildah to v1.40.1
- Updated the containers/common library to v0.63.1

## 5.5.0
### Features
- A new command has been added, `podman machine cp`, to copy files into a running `podman machine` VM.
- A new command has been added, `podman artifact extract`, to copy some or all of the contents of an OCI artifact to a location on disk.
- The `--mount` option to `podman create`, `podman run`, and `podman pod create` now supports a new mount type, `--mount type=artifact`, to mount OCI artifacts into containers.
- The `podman artifact add` command now features two new options, `--append` (to add new files to an existing artifact) and `--file-type` (to specify the MIME type of the file added to the artifact) ([#25884](https://github.com/containers/podman/issues/25884)).
- The `podman artifact rm` command now features a new option, `--all`, to remove all artifacts in the local store.
- The `--filter` option to `podman pause`, `podman ps`, `podman restart`, `podman rm`, `podman start`, `podman stop`, and `podman unpause` now accepts a new filter, `command`, which filters on the first element (`argv[0]`) of the command run in the container.
- The `podman exec` command now supports a new option, `--cidfile`, to specify the ID of the container to exec into via a file ([#21256](https://github.com/containers/podman/issues/21256)).
- The `podman kube generate` and `podman kube play` commands now supports a new annotation, `io.podman.annotation.pids-limit/$containername`, preserving the PID limit for containers across `kube generate` and `kube play` ([#24418](https://github.com/containers/podman/issues/24418)).
- Quadlet `.container` units now support three new keys, `Memory=` (set maximum memory for the created container), `ReloadCmd` (execute a command via systemd `ExecReload`), and `ReloadSignal` (kill the container with the given signal via systemd `ExecReload`) ([#22036](https://github.com/containers/podman/issues/22036)).
- Quadlet `.container`, `.image`, and `.build` units now support two new keys, `Retry` (number of times to retry pulling image on failure) and `RetryDelay` (delay between retries) ([#25109](https://github.com/containers/podman/issues/25109)).
- Quadlet `.pod` units now support a new key, `HostName=`, to set the pod's hostname ([#25639](https://github.com/containers/podman/issues/25639)).
- Quadlet files now support a new option, `UpheldBy`, in the `Install` section, corresponding to the systemd `Upholds` option.
- The names of Quadlet units specified as systemd dependencies are now automatically translated - e.g. `Wants=my.container` is now valid.
- Podman now generates events for the creation and removal of secrets ([#24030](https://github.com/containers/podman/issues/24030)).
- A new global option has been added to Podman, `--cdi-spec-dir`, to specify additional search paths for CDI specs to the CDI loader ([#18292](https://github.com/containers/podman/issues/18292) and [#25691](https://github.com/containers/podman/issues/25691)).
- The `podman build` command now supports a new option, `--inherit-labels` (defaults to true), which controls whether labels are inherited from the base image or base stages.
- The `podman update` command now supports two new options, `--env` and `--unsetenv`, to alter the environment variables of existing containers ([#24875](https://github.com/containers/podman/issues/24875)).

### Breaking Changes
- Due to changes in Docker API types, two small breaking changes have been made in the Go bindings for the REST API. The `containers.Commit()` function now returns a new struct (`types.IDResponse`) with identical contents, and the `containers.ExecCreate` function's `handlers.ExecCreateConfig` parameter now contains a different embedded struct, potentially requiring changes to how it is assigned to.

### Changes
- Podman now requires at least Go 1.23 to build.
- Healthchecks have been refactored to avoid writing to the database as much as possible, greatly improving performance on systems with many simultaneous healthchecks running.
- Healthchecks now have a new status, `stopped`, which is reported if the container the healthcheck was run on stopped before the check could be completed ([#25276](https://github.com/containers/podman/issues/25276)).
- Containers in pods are now stopped in order based on their dependencies, with the infra container being stopped last, preventing application containers from losing networking before they are stopped due to the infra container stopping prematurely.
- Due to challenges with handling automatic installation, the Windows installer no longer installs WSLv2 or Hyper-V.
- Quadlet will now print warnings when skipping lines to help identify malformed Quadlet files ([#25339](https://github.com/containers/podman/issues/25339)).
- Creating `podman machine` VMs with a host mount over the VM's `/tmp` directory is no longer allowed ([#18230](https://github.com/containers/podman/issues/18230)).
- The `podman logs` command now allows options to be specified after the container name (e.g. `podman logs $containername --follow`) ([#25653](https://github.com/containers/podman/issues/25653)).
- Podman, by default, no longer uses a pause image for pod infra and service containers. Instead, a root filesystem containing only the `catatonit` binary will be used ([#23292](https://github.com/containers/podman/issues/23292)).
- The `podman system reset` command no longer removes the user's `podman.sock` API socket.
- When using Netavark v1.15 and higher, containers in non-default networks will no longer have the default search domain `dns.podman` added. Queries resolving such names will still work.
- Stopping a Quadlet `.network` unit will now delete the network (if no containers are actively using it) ([#23678](https://github.com/containers/podman/issues/23678)).
- For security hardening, the `/proc/interrupts` and `/sys/devices/system/cpu/$CPU/thermal_throttle` paths are now masked by default in containers ([#25634](https://github.com/containers/podman/issues/25634)).

### Bugfixes
- Fixed a bug where healthchecks would still run while a container was paused ([#24590](https://github.com/containers/podman/issues/24590)).
- Fixed a bug where the remote Podman client on Windows could not mount named volumes with a single-character name into containers ([#25218](https://github.com/containers/podman/issues/25218)).
- Fixed a bug where mounting an image could panic when run without `CAP_SYS_ADMIN` ([#25241](https://github.com/containers/podman/issues/25241)).
- Fixed a bug where Podman would not report errors when setting up healthchecks ([#25034](https://github.com/containers/podman/issues/25034)).
- Fixed a bug where the `podman exec` command would not add the additional groups of the user the exec session was run as unless the user was explicitly added with the `--user` option ([#25610](https://github.com/containers/podman/issues/25610)).
- Fixed a bug where errors during the `podman network connect` and `podman network disconnect` commands could create errors in the database which would cause `podman inspect` on the container to fail.
- Fixed a bug where the `podman kube generate` command did not correctly generate YAML for volume mounts using a subpath.
- Fixed a bug where the `podman system df` command could show a negative reclaimable size.
- Fixed a bug where accessing a rootful `podman machine` VM that was not `podman-machine-default` (the default VM) with the `podman machine ssh` command would put the user into the rootless shell ([#25332](https://github.com/containers/podman/issues/25332)).
- Fixed a bug where the `podman machine init` would report nonsensical memory values in error messages when trying to create a machine with more memory than the system.
- Fixed a bug where the remote Podman client's `podman start --attach` command would incorrectly print an error when run on a container created with the `--rm` option ([#25965](https://github.com/containers/podman/issues/25965)).
- Fixed a bug where the remote Podman client's `podman pull` command could hang and leak memory if the server was unexpectedly stopped or encountered an error during a pull.
- Fixed a bug where the remote Podman client's `podman cp` command would, on Windows, often fail to copy files into the container due to improper handling of Windows paths ([#14862](https://github.com/containers/podman/issues/14862)).
- Fixed a bug where the `podman container clone` command did not correctly copy healthcheck settings to the new container ([#21630](https://github.com/containers/podman/issues/21630)).
- Fixed a bug where the `podman kube play` command would fail to start empty pods ([#25786](https://github.com/containers/podman/issues/25786)).
- Fixed a bug where the `podman volume ls` command did not output headers when no volumes were present ([#25911](https://github.com/containers/podman/issues/25911)).
- Fixed a bug where healthcheck configuration provided by a container's image could not be overridden unless the `--health-cmd` option was specified when creating the container ([#20212](https://github.com/containers/podman/issues/20212)).
- Fixed a bug where the `--user` option to `podman create` and `podman run` could not be used with users added to the container by the `--hostuser` option ([#25805](https://github.com/containers/podman/issues/25805)).
- Fixed a bug where the `podman system reset` command on FreeBSD would incorrectly print an error.
- Fixed a bug where stopping the `podman machine start` command with SIGINT could result in machine state being incorrectly set to "Starting" ([#24416](https://github.com/containers/podman/issues/24416)).
- Fixed a bug where the `podman machine start` command would fail when starting a VM with volume mounts containing spaces using the HyperV machine provider ([#25500](https://github.com/containers/podman/issues/25500)).

### API
- Fixed a bug where the Compat Create API for Containers ignored ulimits specified in the request when Podman was run rootless ([#25881](https://github.com/containers/podman/issues/25881)).

### Misc
- Erroneous errors from the `ExecStartAndAttach()` function in the Go bindings for the REST API have been silenced, where the function would incorrectly report errors when stdin was consumed after the exec session was stopped ([#25344](https://github.com/containers/podman/issues/25344)).
- Updated Buildah to v1.40.0
- Updated the containers/common library to v0.63.0
- Updated the containers/image library to v5.35.0
- Updated the containers/storage library to v1.58.0

## 5.4.2
### Bugfixes
- Fixed a bug where the `podman import` command could not import images compressed with algorithms other than gzip ([#25593](https://github.com/containers/podman/issues/25593)).
- Fixed a bug where the `podman cp` command could deadlock when copying into a non-empty volume on a container that is not running ([#25585](https://github.com/containers/podman/issues/25585)).

### API
- Fixed a bug where the default values for some fields in the Libpod Create endpoint for Containers did not have sensible defaults for some healthcheck fields, causing unrestricted log growth for containers which did not set these fields ([#25473](https://github.com/containers/podman/issues/25473)).

### Misc
- Updated vendored Buildah to v1.39.4
- Updated the containers/common library to v0.62.3
- Updated the containers/image library to v5.34.3
- Updated the containers/storage library to v1.57.2

## 5.4.1
### Bugfixes
- Fixed a bug where volume quotas were not being applied ([#25368](https://github.com/containers/podman/issues/25368)).
- Fixed a bug where the `--pid-limit=-1` option did not function properly with containers using the `runc` OCI runtime.
- Fixed a bug where the `podman artifact pull` command did not respect the `--retry-delay` option.
- Fixed a bug where Podman would leak a file and directory for every container created.
- Fixed a bug where the `podman wait` command would sometimes error when waiting for a container set to auto-remove.
- Fixed a bug where Quadlet `.kube` units would not report an error (and stay running) even when a pod failed to start ([#20667](https://github.com/containers/podman/issues/20667)).

### API
- Fixed a bug where the Compat DF endpoint did not correctly report total size of all images.

### Misc
- Updated Buildah to v1.39.2
- Updated the containers/common library to v0.62.1
- Updated the containers/image library to v5.34.1

## 5.4.0
### Features
- A preview of Podman's support for OCI artifacts has been added through the `podman artifact` suite of commands, including `add`, `inspect`, `ls`, `pull`, `push`, and `rm`. This support is very early and not fully complete, and the command line interface for these tools has not been finalized. We welcome feedback on the new artifact experience through our issue tracker!
- The `podman update` command now supports a wide variety of options related to healthchecks (including `--health-cmd` to define a new healthcheck and `--no-healthcheck` to disable an existing healthcheck), allowing healthchecks to be added to, removed from, and otherwise updated on existing containers. You can find full details on the 15 added options in the manpage.
- The `--mount type=volume` option for the `podman run`, `podman create`, and `podman volume create` commands now supports a new option, `subpath=`, to make only a subset of the volume visible in the container ([#20661](https://github.com/containers/podman/issues/20661)).
- The `--userns=keep-id` option for the `podman run`, `podman create`, and `podman pod create` commands now supports a new option, `--userns=keep-id:size=`, to configure the size of the user namespace ([#24387](https://github.com/containers/podman/issues/24837)).
- The `podman kube play` command now supports Container Device Interface (CDI) devices ([#17833](https://github.com/containers/podman/issues/17833)).
- The `podman machine init` command now supports a new option, `--playbook`, to run an Ansible playbook in the created VM on first boot for initial configuration.
- Quadlet `.pod` files now support a new field, `ShmSize`, to specify the size of the pod's shared SHM ([#22915](https://github.com/containers/podman/issues/22915)).
- The `podman run`, `podman create`, and `podman pod create` commands now support a new option, `--hosts-file`, to define the base file used for `/etc/hosts` in the container.
- The `podman run`, `podman create`, and `podman pod create` commands now support a new option, `--no-hostname`, which disables the creation of `/etc/hostname` in the container ([#25002](https://github.com/containers/podman/issues/25002)).
- The `podman network create` command now supports a new option for `bridge` networks, `--opt mode=unmanaged`, which allows Podman to use an existing network bridge on the system without changes.
- The `--network` option to `podman run`, `podman create`, and `podman pod create` now accepts a new option for `bridge` networks, `host_interface_name`, which specifies a name for the network interface created outside the container.
- The `podman manifest rm` command now supports a new option, `--ignore`, to not error when removing manifests that do not exist.
- The `podman system prune` command now supports a new option, `--build`, to remove build containers leftover from prematurely terminated builds.
- The `podman events` command now generates events for the creation and removal of networks ([#24032](https://github.com/containers/podman/issues/24032)).

### Breaking Changes
- Due to a lack of availability of hardware to test on, the Podman maintainers are no longer capable of providing full support for Podman on Intel Macs. Binaries and machine images will still be produced, and pull requests related to MacOS on Intel systems will still be merged, but bugs will be fixed on a best effort basis only. We welcome any potential new maintainers who would be able to assist in restoring full support.
- Quadlet previously incorrectly allowed `:` as a character to define comments. This was a mistake; developer intent and documentation was that `#` and `;` were to be used as comment characters instead, matching systemd. This has been corrected, and semicolons now define comments instead of colons.

### Changes
- Podman now passes container hostnames to Netavark, which will use them for any DHCP requests for the container.
- Partial pulls of `zstd:chunked` images now only happen for images that have a `RootFS.DiffID` entry in the image's OCI config JSON, and require the layer contents to match. This resolves issues with image ID ambiguity when partial pulls were enabled.
- Packagers can now set the `BUILD_ORIGIN` environment variable when building podman from the `Makefile`. This provides information on who built the Podman binary, and is displayed in `podman version` and  `podman info`. This will help upstream bug reports, allowing maintainers to trace how and where the binary was built and installed from.

### Bugfixes
- Fixed a bug where `podman machine` VMs on WSL could fail to start when using usermode networking could fail to start due to a port conflict ([#20327](https://github.com/containers/podman/issues/20327)).
- Fixed a bug where overlay mounts could not be made at paths where the image specifies a volume ([#24555](https://github.com/containers/podman/issues/24555)).
- Fixed a bug where the `podman build` command did not honor the `no_pivot_root` setting from `containers.conf` ([#24546](https://github.com/containers/podman/issues/24546)).
- Fixed a bug where volumes would have the wrong permissions if `podman cp` was used to copy into a fresh volume in a container that had never been started.
- Fixed a bug where using `podman cp` to copy into a named volume requiring a mount (image volumes, volumes backed by a volume plugin, or other volumes with options) would fail when the container being copied into was stopped.
- Fixed a bug where rlimits would be set incorrectly when Podman was run as root but without `CAP_SYS_RESOURCE` ([#24692](https://github.com/containers/podman/issues/24692)).
- Fixed a bug where the `podman stats --all` command would fail if a container started with `--cgroups=none` was present ([#24632](https://github.com/containers/podman/issues/24632)).
- Fixed a bug where the `podman info` command would only return details on one image store even if additional image stores were configured in `storage.conf`.
- Fixed a bug where the `podman update` command could reset resource limits that were not being modified to default ([#24610](https://github.com/containers/podman/issues/24610)).
- Fixed a bug where the remote Podman client's `podman update` command could not update resource limits on devices mounted into the container ([#24734](https://github.com/containers/podman/issues/24734)).
- Fixed a bug where the `podman manifest annotate` command could panic when the `--index` option was used ([#24750](https://github.com/containers/podman/issues/24750)).
- Fixed a bug where a Quadlet container reusing another container's network could cause errors if the second container was not already running.
- Fixed a bug where Quadlet files containing lines with a trailing backslash could cause an infinite loop during parsing ([#24810](https://github.com/containers/podman/issues/24810)).
- Fixed a bug where Quadlet would, when run as a non-root user, not generate for files in subfolders of `/etc/containers/systemd/users/` ([#24783](https://github.com/containers/podman/issues/24783)).
- Fixed a bug where values in Quadlet files containing octal escape sequences were incorrectly unescaped.
- Fixed a bug where `podman generate kube` could generate persistent volumes with mixed-case names or names containing an underscore, which are not supported by Kubernetes ([#16542](https://github.com/containers/podman/issues/16542)).
- Fixed a bug where the `ptmxmode` option to `--mount type=devpts` did not function.
- Fixed a bug where shell completion on Windows would include `.exe` in the executable name, breaking completion on some shells.
- Fixed a bug where the output of `podman inspect` on containers did not include the ID of the network the container was joined to, improving Docker compatibility ([#24910](https://github.com/containers/podman/issues/24910)).
- Fixed a bug where containers created with the remote API incorrectly included a create command ([#25026](https://github.com/containers/podman/issues/25026)).
- Fixed a bug where it was possible to specify the `libkrun` backend for VMs on Intel Macs (`libkrun` only supports Arm systems).
- Fixed a bug where `libkrun` and `applehv` VMs from `podman machine` could be started at the same time on Macs ([#25112](https://github.com/containers/podman/issues/25112)).
- Fixed a bug where `podman exec` commands could not detach from the exec session using the detach keys ([#24895](https://github.com/containers/podman/issues/24895)).
- Fixed a bug where Podman would fail to start due to a database configuration mismatch when certain fields were configured to the empty string ([#24738](https://github.com/containers/podman/issues/24738)).

### API
- The Compat and Libpod Build APIs for Images now support a new query parameter, `nohosts`, which (when set to true) does not create `/etc/hosts` in the image when building.
- Fixed a bug where the Compat Create API for Containers did not honor CDI devices, preventing (among other things) the use of GPUs with `docker compose` ([#19338](https://github.com/containers/podman/issues/19338)).

### Misc
- The Docker alias script has been fixed to better handle variable substitution.
- Fixed a bug where `podman-restart.service` functioned incorrectly when no containers were present.
- Updated Buildah to v1.39.0
- Updated the containers/common library to v0.62.0
- Updated the containers/storage library to v1.57.1
- Updated the containers/image library to v5.34.0

## 5.3.2
### Security
- This release contains Buildah v1.38.1 which addresses [CVE-2024-11218](https://github.com/advisories/GHSA-5vpc-35f4-r8w6).

### Bugfixes
- Fixed a bug where Quadlet `.build` files could create an invalid podman command line when `Pull=` was used ([#24599](https://github.com/containers/podman/issues/24599)).
- Fixed a bug where the Mac installer did not install the Podman manpages ([#24756](https://github.com/containers/podman/issues/24756)).

### Misc
- Updated Buildah to v1.38.1
- Updated the containers/common library to v0.61.1
- Updated the containers/storage library to v1.56.1
- Updated the containers/image library to v5.33.1

## 5.3.1
### Bugfixes
- Fixed a bug where the `--ignition-path` option to `podman machine init` would prevent creation of necessary files for the VM, rendering it unusable ([#23544](https://github.com/containers/podman/issues/23544)).
- Fixed a bug where rootless containers using the `bridge` networking mode would be unable to start due to a panic caused by a nil pointer dereference ([#24566](https://github.com/containers/podman/issues/24566)).
- Fixed a bug where Podman containers would try to set increased rlimits when started in a user namespace, rendering containers unable to start ([#24508](https://github.com/containers/podman/issues/24508)).
- Fixed a bug where certain SSH configurations would make the remote Podman client unable to connect to the server ([#24567](https://github.com/containers/podman/issues/24567)).
- Fixed a bug where the Windows installer could install WSLv2 when upgrading an existing Podman installation that used the Hyper-V virtualization backend.

## 5.3.0
### Features
- The `podman kube generate` and `podman kube play` commands can now create and run Kubernetes Job YAML ([#17011](https://github.com/containers/podman/issues/17011)).
- The `podman kube generate` command now includes information on the user namespaces for pods and containers in generated YAML. The `podman kube play` command uses this information to duplicate the user namespace configuration when creating new pods based on the YAML.
- The `podman kube play` command now supports Kubernetes volumes of type image ([#23775](https://github.com/containers/podman/issues/23775)).
- The service name of systemd units generated by Quadlet can now be set with the `ServiceName` key in all supported Quadlet files ([#23414](https://github.com/containers/podman/issues/23414)).
- Quadlets can now disable their implicit dependency on `network-online.target` via a new key, `DefaultDependencies`, supported by all Quadlet files ([#24193](https://github.com/containers/podman/issues/24193)).
- Quadlet `.container` and `.pod` files now support a new key, `AddHost`, to add hosts to the container or pod.
- The `PublishPort` key in Quadlet `.container` and `.pod` files can now accept variables in its value ([#24081](https://github.com/containers/podman/issues/24081)).
- Quadlet `.container` files now support two new keys, `CgroupsMode` and `StartWithPod`, to configure cgroups for the container and whether the container will be started with the pod it is part of ([#23664](https://github.com/containers/podman/issues/23664) and [#24401](https://github.com/containers/podman/issues/24401)).
- Quadlet `.container` files can now use the network of another container by specifying the `.container` file of the container to share with in the `Network` key.
- Quadlet `.container` files can now mount images managed by `.image` files into the container by using the `Mount=type=image` key with a `.image` target.
- Quadlet `.pod` files now support six new keys, `DNS`, `DNSOption`, `DNSSearch`, `IP`, `IP6`, and `UserNS`, to configure DNS, static IPs, and user namespace settings for the pod ([#23692](https://github.com/containers/podman/issues/23692)).
- Quadlet `.image` files can now give an image multiple times by specifying the `ImageTag` key multiple times ([#23781](https://github.com/containers/podman/issues/23781)).
- Quadlets can now be placed in the `/run/containers/systemd` directory as well as existing directories like `$HOME/containers/systemd` and `/etc/containers/systemd/users`.
- Quadlet now properly handles subdirectories of a unit directory being a symlink ([#23755](https://github.com/containers/podman/issues/23755)).
- The `podman manifest inspect` command now includes the manifest's annotations in its output.
- The output of the `podman inspect` command for containers now includes a new field, `HostConfig.AutoRemoveImage`, which shows whether a container was created with the `--rmi` option set.
- The output of the `podman inspect` command for containers now includes a new field, `Config.ExposedPorts`, which includes all exposed ports from the container, improving Docker compatibility.
- The output of the `podman inspect` command for containers now includes a new field, `Config.StartupHealthCheck`, which shows the container's startup healthcheck configuration.
- The output of the `podman inspect` command for containers now includes a new field in `Mounts`, `SubPath`, which contains any subpath set for image or named volumes.
- The `podman machine list` command now supports a new option, `--all-providers`, which lists machines from all supported VM providers, not just the one currently in use.
- VMs run by `podman machine` on Windows will now provide API access by exposing a Unix socket on the host filesystem which forwards into the VM ([#23408](https://github.com/containers/podman/issues/23408)).
- The `podman buildx prune` and `podman image prune` commands now support a new option, `--build-cache`, which will also clean the build cache.
- The Windows installer has a new radio button to select virtualization provider (WSLv2 or Hyper-V).
- The `--add-host` option to `podman create`, `podman run`, and `podman pod create` now supports specifying multiple hostnames, semicolon-separated (e.g. `podman run --add-host test1;test2:192.168.1.1`) ([#23770](https://github.com/containers/podman/issues/23770)).
- The `podman run` and `podman create` commands now support three new options for configuring healthcheck logging: `--health-log-destination` (specify where logs are stored), `--health-max-log-count` (specify how many healthchecks worth of logs are stored), and `--health-max-log-size` (specify the maximum size of the healthcheck log).

### Changes
- Podman now uses the Pasta `--map-guest-addr` option by default which is used for the `host.containers.internal` entry in `/etc/hosts` to allow containers to reach the host by default ([#19213](https://github.com/containers/podman/issues/19213)).
- The names of the infra containers of pods created by Quadlet are changed to the pod name suffixed with `-infra` ([#23665](https://github.com/containers/podman/issues/23665)).
- The `podman system connection add` command now respects HTTP path prefixes specified with `tcp://` URLs.
- Proxy environment variables (e.g. `https_proxy`) declared in `containers.conf` no longer escape special characters in their values when used with `podman machine` VMs ([#23277](https://github.com/containers/podman/issues/23277)).
- The `podman images --sort=repository` command now also sorts by image tag as well, guaranteeing deterministic output ordering ([#23803](https://github.com/containers/podman/issues/23803)).
- When a user has a rootless `podman machine` VM running and second rootful `podman machine` VM initialized, and the rootless VM is removed, the connection to the second, rootful machine now becomes the default as expected ([#22577](https://github.com/containers/podman/issues/22577)).
- Environment variable secrets are no longer contained in the output of `podman inspect` on a container the secret is used in ([#23788](https://github.com/containers/podman/issues/23788)).
- Podman no longer exits 0 on SIGTERM by default.
- Podman no longer explicitly sets rlimits to their default value, as this could lower the actual value available to containers if it had been set higher previously.
- Quadlet user units now correctly wait for the network to be ready to use via a new service, `podman-user-wait-network-online.service`, instead of the user session's nonfunctional `network-online.target`.
- Exposed ports in the output of `podman ps` are now correctly grouped and deduplicated when they are also published ([#23317](https://github.com/containers/podman/issues/23317)).
- Quadlet build units no longer use `RemainAfterExit=yes` by default.

### Bugfixes
- Fixed a bug where the `--build-context` option to `podman build` did not function properly on Windows, breaking compatibility with Visual Studio Dev Containers ([#17313](https://github.com/containers/podman/issues/17313)).
- Fixed a bug where Quadlet would generate bad arguments to Podman if the `SecurityLabelDisable` or `SecurityLabelNested` keys were used ([#23432](https://github.com/containers/podman/issues/23432)).
- Fixed a bug where the `PODMAN_COMPOSE_WARNING_LOGS` environment variable did not suppress warnings printed by `podman compose` that it was redirecting to an external provider.
- Fixed a bug where, if the `podman container cleanup` command was run on a container in the process of being removed, an error could be printed.
- Fixed a bug where rootless Quadlet units placed in `/etc/containers/systemd/users/` would be loaded for root as well when `/etc/containers/systemd` was a symlink ([#23483](https://github.com/containers/podman/issues/23483)).
- Fixed a bug where the remote Podman client's `podman stop` command would, if called with `--cidfile` pointing to a non-existent file and the `--ignore` option set, stop all containers ([#23554](https://github.com/containers/podman/issues/23554)).
- Fixed a bug where the `podman wait` would only exit only after 20 second when run on a container which rapidly exits and is then restarted by the `on-failure` restart policy.
- Fixed a bug where `podman volume rm` and `podman run -v` could deadlock when run simultaneously on the same volume ([#23613](https://github.com/containers/podman/issues/23613)).
- Fixed a bug where running `podman mount` on a container in the process of being created could cause a nonsensical error indicating the container already existed ([#23637](https://github.com/containers/podman/issues/23637)).
- Fixed a bug where the `podman stop` command could deadlock when run on containers with very large annotations ([#22246](https://github.com/containers/podman/issues/22246)).
- Fixed a bug where the `podman machine stop` command could segfault on Mac when a VM failed to stop gracefully ([#23654](https://github.com/containers/podman/issues/23654)).
- Fixed a bug where the `podman stop` command would not ensure containers created with `--rm` were removed when it exited ([#22852](https://github.com/containers/podman/issues/22852)).
- Fixed a bug where the `--rmi` option to `podman run` did not function correctly with detached containers.
- Fixed a bug where running `podman inspect` on a container on FreeBSD would emit an incorrect value for the `HostConfig.Device` field, breaking compatibility with the Ansible Podman module.
- Fixed a bug where rootless Podman could fail to start containers using the `--cgroup-parent` option ([#23780](https://github.com/containers/podman/issues/23780)).
- Fixed a bug where the `podman build -v` command did not properly handle Windows paths passed as the host directory.
- Fixed a bug where Podman could leak network namespace files if it was interrupted while creating a network namespace ([#24044](https://github.com/containers/podman/issues/24044)).
- Fixed a bug where the remote Podman client's `podman run` command could sometimes fail to retrieve a container's exit code for containers run with the `--rm` option.
- Fixed a bug where `podman machine` on Windows could fail to run VMs for certain usernames containing special characters.
- Fixed a bug where Quadlet would reject `RemapUsers=keep-id` when run as root.
- Fixed a bug where XFS quotas on volumes were not unique, meaning that all volumes using a quota shared the same maximum size and inodes (set by the most recent volume with a quota to be created).
- Fixed a bug where `Service` section of Quadlet files would only use defaults and not respect user input ([#24322](https://github.com/containers/podman/issues/24322)).
- Fixed a bug where `podman volume ls` would sometimes fail when a volume was removed at the same time it was run.
- Fixed a bug where the `--tz=local` option could not be used when the `TZDIR` environment variable was set.

### API
- The Play API for Kubernetes YAML now supports `application/x-tar` compressed context directories ([#24015](https://github.com/containers/podman/pull/24015)).
- Fixed a bug in the Attach API for Containers (for both Compat and Libpod endpoints) which could cause inconsistent failures due to a race condition ([#23757](https://github.com/containers/podman/issues/23757)).
- Fixed a bug where the output for the Compat Top API for Containers did not properly split the output into an array ([#23981](https://github.com/containers/podman/issues/23981)).
- Fixed a bug where the Info API could fail when running `podman system service` via a socket-activated systemd service ([#24152](https://github.com/containers/podman/issues/24152)).
- Fixed a bug where the Events and Logs endpoints for Containers now send status codes immediately, as opposed to when the first event or log line is sent ([#23712](https://github.com/containers/podman/issues/23712)).

### Misc
- Podman now requires Golang 1.22 or higher to build.
- The output of `podman machine start` has been improved when trying to start a machine when another is already running ([#23436](https://github.com/containers/podman/issues/23436)).
- Quadlet will no longer log spurious ENOENT errors when resolving unit directories ([#23620](https://github.com/containers/podman/issues/23620)).
- The Docker alias shell script will now also honor the presence of `$XDG_CONFIG_HOME/containers/nodocker` when considering whether it should print its warning message that Podman is in use.
- The podman-auto-update systemd unit files have been moved into the `contrib/systemd/system` directory in the repo for consistency with our other unit files.
- Updated Buildah to v1.38.0
- Updated the containers/common library to v0.61.0
- Updated the containers/storage library to v1.56.0
- Updated the containers/image library to v5.33.0

## 5.2.5
### Security
- This release addresses [CVE-2024-9675](https://access.redhat.com/security/cve/cve-2024-9675), which allows arbitrary access to the host filesystem from `RUN --mount type=cache` arguments to a Dockerfile being built.
- This release also addresses [CVE-2024-9676](https://access.redhat.com/security/cve/cve-2024-9676), which allows malicious images with a symlink `/etc/passwd` or `/etc/group` to potentially cause a denial of service through reading a FIFO on the host.

### Misc
- Updated Buildah to v1.37.5
- Updated the containers/storage library to v1.55.1

## 5.2.4
### Security
- This release addresses [CVE-2024-9407](https://github.com/advisories/GHSA-fhqq-8f65-5xfc), which allows arbitrary access to the host filesystem from `RUN --mount` arguments to a Dockerfile being built.
- This release also addresses [CVE-2024-9341](https://github.com/advisories/GHSA-mc76-5925-c5p6), allowing the mounting of arbitrary directories from the host into containers on FIPS enabled systems using a malicious image with crafted symlinks.

### Misc
- Updated Buildah to v1.37.4
- Updated the containers/common library to v0.60.4

## 5.2.3
### Bugfixes
- Fixed a bug that could cause network namespaces to fail to unmount, resulting in Podman commands hanging.
- Fixed a bug where Podman could not run images which included SCTP exposed ports.
- Fixed a bug where containers run by the root user, but inside a user namespace (including inside a container), could not use the `pasta` network mode.
- Fixed a bug where volume copy-up did not properly chown empty volumes when the `:idmap` mount option was used.

### Misc
- Updated Buildah to v1.37.3

## 5.2.2
### Bugfixes
- Fixed a bug where rootless Podman could fail to validate the runtime's volume path on systems with a symlinked `/home` ([#23515](https://github.com/containers/podman/issues/23515)).

### Misc
- Updated Buildah to v1.37.2
- Updated the containers/common library to v0.60.2
- Updated the containers/image library to v5.32.2

## 5.2.1
### Bugfixes
- Fixed a bug where Podman could sometimes save an incorrect container state to the database, which could cause a number of issues including but not limited to attempting to clean up containers twice ([#21569](https://github.com/containers/podman/issues/21569)).

### Misc
- Updated Buildah to v1.37.1
- Updated the containers/common library to v0.60.1
- Updated the containers/image library to v5.32.1

## 5.2.0
### Features
- Podman now supports `libkrun` as a backend for creating virtual machines on MacOS. The `libkrun` backend has the advantage of allowing GPUs to be mounted into the virtual machine to accelerate tasks. The default backend remains `applehv`.
- Quadlet now has support for `.build` files, which allows images to be built by Quadlet and then used by Quadlet containers.
- Quadlet `.container` files now support two new fields, `LogOpt` to specify container logging configuration and `StopSignal` to specify container stop signal ([#23050](https://github.com/containers/podman/issues/23050)).
- Quadlet `.container` and `.pod` files now support a new field, `NetworkAlias`, to add network aliases.
- Quadlet drop-in search paths have been expanded to include top-level type drop-ins (`container.d`, `pod.d`) and truncated unit drop-ins (`unit-.container.d`) ([#23158](https://github.com/containers/podman/issues/23158)).
- Podman now supports a new command, `podman system check`, which will identify (and, if possible, correct) corruption within local container storage.
- The `podman machine reset` command will now reset all providers available on the current operating system (e.g. ensuring that both HyperV and WSL `podman machine` VMs will be removed on Windows).

### Changes
- Podman now requires the new kernel mount API, introducing a dependency on Linux Kernel v5.2 or higher.
- Quadlet `.image` units now have a dependency on `network-online.target` ([#21873](https://github.com/containers/podman/issues/21873)).
- The `--device` option to `podman create` and `podman run` is no longer ignored when `--privileged` is also specified ([#23132](https://github.com/containers/podman/issues/23132)).
- The `podman start` and `podman stop` commands no longer print the full ID of the pod started/stopped, but instead the user's input used to specify the pod (e.g. `podman pod start b` will print `b` instead of the pod's full ID) ([#22590](https://github.com/containers/podman/issues/22590)).
- Virtual machines created by `podman machine` on Linux now use `virtiofs` instead of `9p` for mounting host filesystems. Existing mounts will be transparently changed on machine restart or recreation. This should improve performance and reliability of host mounts. This requires the installation of `virtiofsd` on the host system to function.
- Using both the `--squash` and `--layers=false` options to `podman build` at the same time is now allowed.
- Podman now passes container's stop timeout to systemd when creating cgroups, causing it to be honored when systemd stops the scope. This should prevent hangs on system shutdown due to running Podman containers.
- The `--volume-driver` option to `podman machine init` is now deprecated.

### Bugfixes
- Fixed a bug where rootless containers created with the `--sdnotify=healthy` option could panic when started ([#22651](https://github.com/containers/podman/issues/22651)).
- Fixed a bug where containers created with the `--sdnotify=healthy` option that exited quickly would sometimes return an error instead of notifying that the container was ready ([#22760](https://github.com/containers/podman/issues/22760)).
- Fixed a bug where the `podman system reset` command did not remove the containers/image blob cache ([#22825](https://github.com/containers/podman/issues/22825)).
- Fixed a bug where Podman would sometimes create a cgroup for itself even when the `--cgroups=disabled` option was specified at container creation time ([#20910](https://github.com/containers/podman/issues/20910)).
- Fixed a bug where the `/etc/hosts` file in a container was not created with a newline at the end of the file ([#22729](https://github.com/containers/podman/issues/22729)).
- Fixed a bug where the `podman start` command could sometimes panic when starting a container in the stopped state.
- Fixed a bug where the `podman system renumber` command would fail if volumes existed when using the `sqlite` database backend ([#23052](https://github.com/containers/podman/issues/23052)).
- Fixed a bug where the `podman container restore` command could not successfully restore a container in a pod.
- Fixed a bug where an error message from `podman diff` would suggest using the `--latest` option when using the remote Podman client ([#23038](https://github.com/containers/podman/issues/23038)).
- Fixed a bug where user could assign more memory to a Podman machine than existed on the host ([#18206](https://github.com/containers/podman/issues/18206)).
- Fixed a bug where the `podman events` command was rarely unable to report errors that occurred ([#23165](https://github.com/containers/podman/issues/23165)).
- Fixed a bug where containers run in systemd units would sometimes not be removed correctly on exit when using the `--cidfile` option.
- Fixed a bug where the first Podman command run after a reboot could cause hang when using transient mode ([#22984](https://github.com/containers/podman/issues/22984)).
- Fixed a bug where Podman could throw errors about a database configuration mismatch if certain paths did not exist on the host.
- Fixed a bug where the `podman run` and `podman start` commands could throw strange errors if another Podman process stopped the container at a midpoint in the process of starting ([#23246](https://github.com/containers/podman/issues/23246)).
- Fixed a bug where the `podman system service` command could leak a mount on termination.
- Fixed a bug where the Podman remote client would panic if an invalid image filter was passed to `podman images` ([#23120](https://github.com/containers/podman/issues/23120)).
- Fixed a bug where the `podman auto-update` and `podman system df` commands could fail when a container was removed while the command was running ([#23279](https://github.com/containers/podman/issues/23279)).
- Fixed a bug where the `podman machine init` command could panic when trying to decompress an empty file when preparing the VM image ([#23281](https://github.com/containers/podman/issues/23281)).
- Fixed a bug where the `podman ps --pod` and `podman pod stats` commands could sometimes fail when a pod was removed while the command was running ([#23282](https://github.com/containers/podman/issues/23282)).
- Fixed a bug where the `podman stats` and `podman pod stats` commands would sometimes exit with a `container is stopped` error when showing all containers (or pod containers, for `pod stats`) if a container stopped while the command was running ([#23334](https://github.com/containers/podman/issues/23334)).
- Fixed a bug where the output of container healthchecks was not properly logged if it did not include a final newline ([#23332](https://github.com/containers/podman/issues/23332)).
- Fixed a bug where the port forwarding firewall rules of an existing container could be be overwritten when starting a second container which forwarded the same port on the host even if the second container failed to start as the port was already bound.
- Fixed a bug where the containers created by the `podman play kube` command could sometimes not properly clean up their network stacks ([#21569](https://github.com/containers/podman/issues/21569)).

### API
- The Build API for Images now accepts a comma-separated list in the Platform query parameter, allowing a single API call to built an image for multiple architectures ([#22071](https://github.com/containers/podman/issues/22071)).
- Fixed a bug where the Remove endpoint for Volumes would return an incorrectly formatted error when called with an ambiguous volume name ([#22616](https://github.com/containers/podman/issues/22616)).
- Fixed a bug where the Stats endpoint for Containers would return an incorrectly formatted error when called on a container that did not exist ([#22612](https://github.com/containers/podman/issues/22612)).
- Fixed a bug where the Start endpoint for Pods would return a 409 error code in cases where a 500 error code should have been returned ([#22989](https://github.com/containers/podman/issues/22989)).
- Fixed a bug where the Top endpoint for Pods would return a 200 status code and then subsequently an error ([#22986](https://github.com/containers/podman/issues/22986)).

### Misc
- Podman no longer requires all parent directories of its root and runroot to be world-executable ([#23028](https://github.com/containers/podman/issues/23028)).
- Error messages from the `podman build` command when the `-f` option is given, but points to a file that does not exist, have been improved ([#22940](https://github.com/containers/podman/issues/22940)).
- The Podman windows installer is now built using WiX 5.
- Updated the gvisor-tap-vsock library to v0.7.4. This release contains a fix for a gvproxy crash on macOS when there is heavy network traffic on a fast link.
- Updated Buildah to v1.37.0
- Updated the containers/image library to v5.32.0
- Updated the containers/storage library to v1.55.0
- Updated the containers/common library to v0.60.0

## 5.1.2
### Bugfixes
- Fixed a bug that would sometimes prevent the mount of some `podman machine` volumes into the virtual machine when using the Apple hypervisor ([#22569](https://github.com/containers/podman/issues/22569)).
- Fixed a bug where `podman top` would show the incorrect UID for processes in containers run in a user namespace ([#22293](https://github.com/containers/podman/issues/22293)).
- Fixed a bug where the `/etc/hosts` and `/etc/resolv.conf` files in a container would be empty after restoring from a checkpoint ([#22901](https://github.com/containers/podman/issues/22901)).
- Fixed a bug where the `--pod-id-file` argument to `podman run` and `podman create` did not respect the pod's user namespace ([#22931](https://github.com/containers/podman/issues/22931)).
- Fixed a bug in the Podman remote client where specifying a invalid connection in the `CONTAINER_CONNECTION` environment variable would lead to a panic.

### Misc
- Virtual machines run by `podman machine` using the Apple hypervisor now wait 90 seconds before forcibly stopping the VM, matching the standard systemd shutdown timeout ([#22515](https://github.com/containers/podman/issues/22515)).
- Updates the containers/image library to v5.31.1

## 5.1.1
### Bugfixes
- Fixed a bug where systemd timers associated with startup healthchecks would not be properly deleted after transitioning to the regular healthcheck ([#22884](https://github.com/containers/podman/issues/22884)).

### Misc
- Updated the containers/common library to v0.59.1

## 5.1.0
### Features
- VMs created by `podman machine` on macOS with Apple silicon can now use Rosetta 2 (a.k.a Rosetta) for high-speed emulation of x86 code. This is enabled by default. If you wish to change this option, you can do so in `containers.conf`.
- Changes made by the `podman update` command are now persistent, and will survive container restart and be reflected in `podman inspect`.
- The `podman update` command now includes a new option, `--restart`, to update the restart policy of existing containers.
- Quadlet `.container` files now support a new key, `GroupAdd`, to add groups to the container.
- Container annotations are now printed by `podman inspect`.
- Image-based mounts using `podman run --mount type=image,...` now support a new option, `subpath`, to mount only part of the image into the container.
- A new field, `healthcheck_events`, has been added to `containers.conf` under the `[engine]` section to allow users to disable the generation of `health_status` events to avoid spamming logs on systems with many healthchecks.
- A list of images to automatically mount as volumes can now be specified in Kubernetes YAML via the `io.podman.annotations.kube.image.automount/$CTRNAME` annotation (where `$CTRNAME` is the name of the container they will be mounted into).
- The `podman info` command now includes the default rootless network command (`pasta` or `slirp4netns`).
- The `podman ps` command now shows ports from `--expose` that have not been published with `--publish-all` to improve Docker compatibility.
- The `podman container runlabel` command now expands `$HOME` in the label being run to the user's home directory.
- A new alias, `podman network list`, has been added to the `podman network ls` command.
- The name and shell of containers created by `podmansh` can now be set in `containers.conf`.
- The `podman-setup.exe` Windows installer now provides 3 new CLI variables, `MachineProvider` (choose the provider for the machine, `windows` or `wsl`, the default), `HyperVCheckbox` (can be set to `1` to install HyperV if it is not already installed or `0`, the default, to not install HyperV), and `SkipConfigFileCreation` (can be set to `1` to disable the creation of configuration files, or `0`, the default).

### Changes
- Podman now changes volume ownership every time an empty named volume is mounted into a container, not just the first time, matching Docker's behavior.
- When running Kubernetes YAML with `podman kube play` that does not include an `imagePullPolicy` and does not set a tag for the image, the image is now always pulled ([#21211](https://github.com/containers/podman/issues/21211)).
- When running Kubernetes YAML with `podman kube play`, pod-level restart policies are now passed down to individual containers within the pod ([#20903](https://github.com/containers/podman/issues/20903)).
- The `--runroot` global option can now accept paths with lengths longer than 50 characters ([#22272](https://github.com/containers/podman/issues/22272)).
- Updating containers with the `podman update` command now emits an event.

### Bugfixes
- Fixed a bug where the `--userns=keep-id:uid=0` option to `podman create` and `podman run` would generate incorrect UID mappings and cause the container to fail to start ([#22078](https://github.com/containers/podman/issues/22078)).
- Fixed a bug where `podman stats` could report inaccurate percentages for very large or very small values ([#22064](https://github.com/containers/podman/issues/22064)).
- Fixed a bug where bind-mount volumes defaulted to `rbind` instead of `bind`, meaning recursive mounts were allowed by default ([#22107](https://github.com/containers/podman/issues/22107)).
- Fixed a bug where the `podman machine rm -f` command would fail to remove Hyper-V virtual machines if they were running.
- Fixed a bug where the `podman ps --sync` command could sometimes fail to properly update the status of containers.
- Fixed a bug where bind-mount volumes using the `:idmap` option would sometimes be inaccessible with rootless Podman ([#22228](https://github.com/containers/podman/issues/22228)).
- Fixed a bug where bind-mount volumes using the `:U` option would have their ownership changed to the owner of the directory in the image being mounted over ([#22224](https://github.com/containers/podman/issues/22224)).
- Fixed a bug where removing multiple containers, pods, or images with the `--force` option did not work when multiple arguments were given to the command and one of them did not exist ([#21529](https://github.com/containers/podman/issues/21529)).
- Fixed a bug where Podman did not properly clean up old cached Machine images.
- Fixed a bug where rapidly-restarting containers with healthchecks could sometimes fail to start their healthchecks after restarting.
- Fixed a bug where nested Podman could create its `pause.pid` file in an incorrect directory ([#22327](https://github.com/containers/podman/issues/22327)).
- Fixed a bug where Podman would panic if an OCI runtime was configured without associated paths in `containers.conf` ([#22561](https://github.com/containers/podman/issues/22561)).
- Fixed a bug where the `podman kube down` command would not respect the `StopTimeout` and `StopSignal` of containers that it stopped ([#22397](https://github.com/containers/podman/issues/22397)).
- Fixed a bug where Systemd-managed containers could be stuck in the Stopping state, unable to be restarted, if systemd killed the unit before `podman stop` finished stopping the container ([#19629](https://github.com/containers/podman/issues/19629)).
- Fixed a bug where the remote Podman client's `podman farm build` command would not updating manifests on the registry that were already pushed ([#22647](https://github.com/containers/podman/issues/22647)).
- Fixed a bug where rootless Podman could fail to re-exec itself when run with a custom `argv[0]` that is not a valid command path, as might happen when used in `podmansh` ([#22672](https://github.com/containers/podman/issues/22672)).
- Fixed a bug where `podman machine` connection URIs could be incorrect after an SSH port conflict, rendering machines inaccessible.
- Fixed a bug where the `podman events` command would not print an error if incorrect values were passed to its `--since` and `--until` options.
- Fixed a bug where an incorrect `host.containers.internal` entry could be added when running rootless containers using the `bridge` network mode ([#22653](https://github.com/containers/podman/issues/22653)).

### API
- A new Docker-compatible endpoint, Update, has been added for containers.
- The Compat Create endpoint for Containers now supports setting container annotations.
- The Libpod List endpoint for Images now includes additional information in its responses (image architecture, OS, and whether the image is a manifest list) ([#22184](https://github.com/containers/podman/issues/22184) and [#22185](https://github.com/containers/podman/issues/22185)).
- The Build endpoint for Images no longer saves the build context as a temporary file, substantially improving performance and reducing required filesystem space on the server.
- The Inspect API for Containers now returns results compatible with Podman v4.x when a request with version v4.0.0 is made. This allows Podman 4.X remote clients work with a Podman 5.X server ([#22657](https://github.com/containers/podman/issues/22657)).
- Fixed a bug where the Build endpoint for Images would not clean up temporary files created by the build if an error occurred.

### Misc
- Podman now detects unhandled system reboots and advises the user on proper mitigations.
- Improved debugging output for `podman machine` on Darwin systems when `--log-level=debug` is used.
- The Makefile now allows injecting extra build tags via the `EXTRA_BUILD_TAGS` environment variable.
- Updated Buildah to v1.36.0
- Updated the containers/common library to v0.59.0
- Updated the containers/image library to v5.31.0
- Updated the containers/storage library to v1.54.0

## 5.0.3
### Security
- This release addresses CVE-2024-3727, a vulnerability in the containers/image library which allows attackers to trigger authenticated registry access on behalf of the victim user.

### Bugfixes
- Fixed a bug where `podman machine start` would fail if the machine had a volume with a long target path ([#22226](https://github.com/containers/podman/issues/22226)).
- Fixed a bug where `podman machine start` mounted volumes with paths that included dashes in the wrong location ([#22505](https://github.com/containers/podman/issues/22505)).

### Misc
- Updated Buildah to v1.35.4
- Updated the containers/common library to v0.58.3
- Updated the containers/image library to v5.30.1

## 5.0.2
### Bugfixes
- Fixed a bug that could leak IPAM entries when a network was removed ([#22034](https://github.com/containers/podman/issues/22034)).
- Fixed a bug that could cause the rootless network namespace to not be cleaned up on if an error occurred during setup resulting in errors relating to a missing resolv.conf being displayed ([#22168](https://github.com/containers/podman/issues/22168)).
- Fixed a bug where Podman would use rootless network namespace logic for nested containers ([#22218](https://github.com/containers/podman/issues/22218)).
- Fixed a bug where writing to volumes on a Mac could result in EACCESS failures when using the `:z` or `:Z` volume mount options on a directory with read only files ([#19852](https://github.com/containers/podman/issues/19852))

### API
- Fixed a bug in the Compat List endpoint for Networks which could result in a server crash due to concurrent writes to a map ([#22330](https://github.com/containers/podman/issues/22330)).

## 5.0.1
### Bugfixes
- Fixed a bug where rootless containers using the Pasta network driver did not properly handle localhost DNS resolvers on the host leading to DNS resolution issues ([#22044](https://github.com/containers/podman/issues/22044)).
- Fixed a bug where Podman would warn that cgroups v1 systems were no longer supported on FreeBSD hosts.
- Fixed a bug where HyperV `podman machine` VMs required an SSH client be installed on the system ([#22075](https://github.com/containers/podman/issues/22075)).
- Fixed a bug that prevented the remote Podman client's `podman build` command from working properly when connecting from a rootless client to a rootful server ([#22109](https://github.com/containers/podman/issues/22109)).

### Misc
- The HyperV driver to `podman machine` now fails immediately if admin privileges are not available (previously, it would only fail when it reached operations that required admin privileges).

## 5.0.0
### Features
- VMs created by `podman machine` can now use the native Apple hypervisor (`applehv`) when run on MacOS.
- A new command has been added, `podman machine reset`, which will remove all existing `podman machine` VMs and relevant configurations.
- The `podman manifest add` command now supports a new `--artifact` option to add OCI artifacts to a manifest list.
- The `podman create`, `podman run`, and `podman push` commands now support the `--retry` and `--retry-delay` options to configure retries for pushing and pulling images.
- The `podman run` and `podman exec` commands now support a new option, `--preserve-fd`, which allows passing a list of file descriptors into the container (as an alternative to `--preserve-fds`, which passes a specific number of file descriptors).
- Quadlet now supports templated units ([#17744](https://github.com/containers/podman/discussions/17744)).
- The `podman kube play` command can now create image-based volumes using the `volume.podman.io/image` annotation.
- Containers created with `podman kube play` can now include volumes from other containers (similar to the `--volumes-from` option) using a new annotation, `io.podman.annotations.volumes-from` ([#16819](https://github.com/containers/podman/issues/16819)).
- Pods created with `podman kube play` can now set user namespace options through the `io.podman.annotations.userns` annotation in the pod definition ([#20658](https://github.com/containers/podman/issues/20658)).
- Macvlan and ipvlan networks can adjust the name of the network interface created inside containers via the new `containers.conf` field `interface_name` ([#21313](https://github.com/containers/podman/issues/21313)).
- The `--gpus` option to `podman create` and `podman run` is now compatible with Nvidia GPUs ([#21156](https://github.com/containers/podman/issues/21156)).
- The `--mount` option to `podman create` and `podman run` supports a new mount option, `no-dereference`, to mount a symlink (instead of its dereferenced target) into a container ([#20098](https://github.com/containers/podman/issues/20098)).
- Podman now supports a new global option, `--config`, to point to a Docker configuration where we can source registry login credentials.
- The `podman ps --format` command now supports a new format specifier, `.Label` ([#20957](https://github.com/containers/podman/issues/20957)).
- The `uidmapping` and `gidmapping` options to the `podman run --userns=auto` option can now map to host IDs by prefixing host IDs with the `@` symbol.
- Quadlet now supports systemd-style drop-in directories.
- Quadlet now supports creating pods via new `.pod` unit files ([#17687](https://github.com/containers/podman/discussions/17687)).
- Quadlet now supports two new keys, `Entrypoint` and `StopTimeout`, in `.container` files ([#20585](https://github.com/containers/podman/issues/20585) and [#21134](https://github.com/containers/podman/issues/21134)).
- Quadlet now supports specifying the `Ulimit` key multiple times in `.container` files to set more than one ulimit on a container.
- Quadlet now supports setting the `Notify` key to `healthy` in `.container` files, to only sdnotify that a container has started when its health check begins passing ([#18189](https://github.com/containers/podman/issues/18189)).

### Breaking Changes
- The backend for the `podman machine` commands has seen extensive rewrites. Configuration files have changed format and VMs from Podman 4.x and earlier are no longer usable. `podman machine` VMs must be recreated with Podman 5.
- The `podman machine init` command now pulls images as OCI artifacts, instead of using HTTP. As a result, a valid `policy.json` file is required on the host. Windows and Mac installers have been changed to install this file.
- QEMU is no longer a supported VM provider for `podman machine` on Mac. Instead, the native Apple hypervisor is supported.
- The `ConfigPath` and `Image` fields are no longer provided by the `podman machine inspect` command. Users can also no longer use `{{ .ConfigPath }}` or `{{ .Image }}` as arguments to `podman machine inspect --format`.
- The output of `podman inspect` for containers has seen a number of breaking changes to improve Docker compatibility, including changing `Entrypoint` from a string to an array of strings and StopSignal from an int to a string.
- The `podman inspect` command for containers now returns nil for healthchecks when inspecting containers without healthchecks.
- The `podman pod inspect` command now outputs a JSON array regardless of the number of pods inspected (previously, inspecting a single pod would omit the array).
- It is no longer possible to create new BoltDB databases; attempting to do so will result in an error. All new Podman installations will now use the SQLite database backend. Existing BoltDB databases remain usable.
- Support for CNI networking has been gated by a build tag and will not be enabled by default.
- Podman will now print warnings when used on cgroups v1 systems. Support for cgroups v1 is deprecated and will be removed in a future release. The `PODMAN_IGNORE_CGROUPSV1_WARNING` environment variable can be set to suppress warnings.
- Network statistics sent over the Docker API are now per-interface, and not aggregated, improving Docker compatibility.
- The default tool for rootless networking has been swapped from `slirp4netns` to `pasta` for improved performance. As a result, networks named `pasta` are no longer supported.
- The `--image` option replaces the now deprecated `--image-path` option for `podman machine init`.
- The output of `podman events --format "{{json .}}"` has been changed to improve Docker compatibility, including the `time` and `timeNano` fields ([#14993](https://github.com/containers/podman/issues/14993)).
- The name of `podman machine` VMs and the username used within the VM are now validated and must match this regex: `[a-zA-Z0-9][a-zA-Z0-9_.-]*`.
- Using multiple filters with the List Images REST API now combines the filters with AND instead of OR, improving Docker compatibility ([#18412](https://github.com/containers/podman/issues/18412)).
- The parsing for a number of Podman CLI options which accept arrays has been changed to no longer accept string-delineated lists, and instead to require the option to be passed multiple times. These options are `--annotation` to `podman manifest annotate` and `podman manifest add`, the `--configmap`, `--log-opt`, and `--annotation` options to `podman kube play`, the `--pubkeysfile` option to `podman image trust set`, the `--encryption-key` and `--decryption-key` options to `podman create`, `podman run`, `podman push` and `podman pull`, the `--env-file` option to `podman exec`, the `--bkio-weight-device`, `--device-read-bps`, `--device-write-bps` `--device-read-iops`, `--device-write-iops`, `--device`, `--label-file`, `--chrootdirs`, `--log-opt`, and `--env-file` options to `podman create` and `podman run`, and the `--hooks-dir` and `--module` global options.

### Changes
- The `podman system reset` command no longer waits for running containers to gracefully stop, and instead immediately sends SIGKILL ([#21874](https://github.com/containers/podman/issues/21874)).
- The `podman network inspect` command now includes running containers using the network in its output ([#14126](https://github.com/containers/podman/issues/14126)).
- The `podman compose` command is now supported on non-AMD64/ARM64 architectures.
- VMs created by `podman machine` will now pass HTTP proxy environment variables into the VM for all providers.
- The `--no-trunc` option to the `podman kube play` and `podman kube generate` commands has been deprecated. Podman now complies to the Kubernetes specification for annotation size, removing the need for this option.
- The `DOCKER_HOST` environment variable will be set by default for rootless users when podman-docker is installed.
- Connections from `podman system connection` and farms from `podman farm` are now written to a new configuration file called `podman-connections.conf`. As a result, Podman no longer writes to `containers.conf`. Existing connections from `containers.conf` will still be respected.
- Most `podman farm` subcommands (save for `podman farm build`) no longer need to connect to the machines in the farm to run.
- The `podman create` and `podman run` commands no longer require specifying an entrypoint on the command line when the container image does not define one. In this case, an empty command will be passed to the OCI runtime, and the resulting behavior is runtime-specific.
- The default SELinux label for content mounted from the host in `podman machine` VMs on Mac is now `system_u:object_r:nfs_t:s0` so that it can be shared with all containers without issue.
- Newly-created VMs created by `podman machine` will now share a single SSH key key for access. As a result, `podman machine rm --save-keys` is deprecated as the key will persist by default.

### Bugfixes
- Fixed a bug where the `podman stats` command would not show network statistics when the `pasta` network mode was used.
- Fixed a bug where `podman machine` VMs using the HyperV provider could not mount shares on directories that did not yet exist.
- Fixed a bug where the `podman compose` command did not respect the `--connection` and `--url` options.
- Fixed a bug where the `podman stop -t -1` command would wait for 0 seconds, not infinite seconds, before sending SIGKILL ([#21811](https://github.com/containers/podman/issues/21811)).
- Fixed a bug where Podman could deadlock when cleaning up a container when the `slirp4netns` network mode was used with a restart policy of `always` or `unless-stopped` or `on-failure` and a user namespace ([#21477](https://github.com/containers/podman/issues/21477)).
- Fixed a bug where uninstalling Podman on Mac did not remove the `docker.sock` symlink ([#20650](https://github.com/containers/podman/issues/20650)).
- Fixed a bug where preexisting volumes being mounted into a new container using a path that exists in said container would not be properly chowned ([#21608](https://github.com/containers/podman/issues/21608)).
- Fixed a bug where the `podman image scp` command could fail if there was not sufficient space in the destination machine's `/tmp` for the image ([#21239](https://github.com/containers/podman/issues/21239)).
- Fixed a bug where containers killed by running out of memory (including due to a memory limit) were not properly marked as OOM killed in `podman inspect` ([#13102](https://github.com/containers/podman/issues/13102)).
- Fixed a bug where `podman kube play` did not create memory-backed emptyDir volumes using a tmpfs filesystem.
- Fixed a bug where containers started with `--rm` were sometimes not removed after a reboot ([#21482](https://github.com/containers/podman/issues/21482)).
- Fixed a bug where the `podman events` command using the remote Podman client did not display the network name associated with network events ([#21311](https://github.com/containers/podman/issues/21311)).
- Fixed a bug where the `podman farm build` did not properly handle the `--tls-verify` option and would override server defaults even if the option was not set by the user ([#21352](https://github.com/containers/podman/issues/21352)).
- Fixed a bug where the `podman inspect` command could segfault on FreeBSD ([#21117](https://github.com/containers/podman/issues/21117)).
- Fixed a bug where Quadlet did not properly handle comment lines ending with a backslash ([#21555](https://github.com/containers/podman/issues/21555)).
- Fixed a bug where Quadlet would sometimes not report errors when malformed quadlet files were present.
- Fixed a bug where Quadlet could hang when given a `.container` file with certain types of trailing whitespace ([#21109](https://github.com/containers/podman/issues/21109)).
- Fixed a bug where Quadlet could panic when generating from Kubernetes YAML containing the `bind-mount-options` key ([#21080](https://github.com/containers/podman/issues/21080)).
- Fixed a bug where Quadlet did not properly strip quoting from values in `.container` files ([#20992](https://github.com/containers/podman/issues/20992)).
- Fixed a bug where the `--publish-all` option to `podman kube play` did not function when used with the remote Podman client.
- Fixed a bug where the `podman kube play --build` command could not build images whose Dockerfile specified an image from a private registry with a self-signed certificate in a `FROM` directive ([#20890](https://github.com/containers/podman/discussions/20890)).
- Fixed a bug where container remove events did not have the correct exit code set ([#19124](https://github.com/containers/podman/issues/19124)).

### API
- A new API endpoint, `/libpod/images/$name/resolve`, has been added to resolve a (potential) short name to a list of fully-qualified image references Podman which could be used to pull the image.
- Fixed a bug where the List API for Images did not properly handle filters and would discard all but the last listed filter.
- Fixed a bug in the Docker Create API for Containers where entries from `/etc/hosts` were copied into create containers, resulting in incompatibility with network aliases.
- The API bindings have been refactored to reduce code size, leading to smaller binaries ([#17167](https://github.com/containers/podman/issues/17167)).

### Misc
- Failed image pulls will now generate an event including the error.
- Updated Buildah to v1.35.0
- Updated the containers/image library to v5.30.0
- Updated the containers/storage library to v1.53.0
- Updated the containers/common library to v0.58.0
- Updated the libhvee library to v0.7.0

## 4.9.3
### Features
- The `podman container commit` command now features a `--config` option which accepts a filename containing a JSON-encoded container configuration to be merged in to the newly-created image.

## 4.9.2
### Security
- This release addresses a number of Buildkit vulnerabilities including but not limited to: [CVE-2024-23651](https://github.com/advisories/GHSA-m3r6-h7wv-7xxv), [CVE-2024-23652](https://github.com/advisories/GHSA-4v98-7qmw-rqr8), and [CVE-2024-23653](https://github.com/advisories/GHSA-wr6v-9f75-vh2g).

### Misc
- Updated Buildah to v1.33.5
- Updated the containers/common library to v0.57.4

## 4.9.1
### Bugfixes
- Fixed a bug where the `--rootful` option to `podman machine set` would not set the machine to use the root connection ([#21195](https://github.com/containers/podman/issues/21195)).
- Fixed a bug where podman would crash when running in a containerized environment with `euid != 0` and capabilities set ([#20766](https://github.com/containers/podman/issues/20766)).
- Fixed a bug where the `podman info` command would crash on if called multiple times when podman was running as `euid=0` without `CAP_SYS_ADMIN` ([#20908](https://github.com/containers/podman/issues/20908)).
- Fixed a bug where `podman machine` commands were not relayed to the correct machine on AppleHV ([#21115](https://github.com/containers/podman/issues/21115)).
- Fixed a bug where the `podman machine list` and `podman machine inspect` commands would not show the correct `Last Up` time on AppleHV ([#21244](https://github.com/containers/podman/issues/21244)).

### Misc
- Updated the Mac pkginstaller QEMU to v8.2.1
- Updated Buildah to v1.33.4
- Updated the containers/image library to v5.29.2
- Updated the containers/common library to v0.57.3

## 4.9.0
### Features
- The `podman farm` suite of commands for multi-architecture builds is now fully enabled and documented.
- Add a network recovery service to Podman Machine VMs using the QEMU backend to detect and recover from an inoperable host networking issues experienced by Mac users when running for long periods of time.

### Bugfixes
- Fixed a bug where the HyperV provider for `podman machine` did not forward the API socket to the host machine.
- Fixed a bug where improperly formatted annotations passed to `podman kube play` could cause Podman to panic.
- Fixed a bug where `podman system reset` could fail if non-Podman containers (e.g. containers created by Buildah) were present.

### Misc
- Containers run in `podman machine` VMs now default to a PID limit of unlimited, instead of 2048.

## 4.8.3
### Security
- Fixed [GHSA-45x7-px36-x8w8](https://github.com/advisories/GHSA-45x7-px36-x8w8): CVE-2023-48795 by vendoring golang.org/x/crypto v0.17.0.

## 4.8.2
### Bugfixes
- Fixed a bug in the MacOS pkginstaller where Podman machine was using a different QEMU binary than the one installed using the installer, if it existed on the system ([#20808](https://github.com/containers/podman/issues/20808)).
- Fixed a bug on Windows (WSL) with the first-time install of user-mode networking when using the init command, as opposed to set ([#20921](https://github.com/containers/podman/issues/20921)).

### Quadlet
- Fixed a bug where Kube image build failed when starting service with missing image ([#20432](https://github.com/containers/podman/issues/20432)).

## 4.8.1
### Bugfixes
- Fixed a bug on Windows (WSL) where wsl.conf/resolv.conf was not restored when user-mode networking was disabled after being enabled ([#20625](https://github.com/containers/podman/issues/20625)).
- Fixed a bug where currently if user specifies `podman kube play --replace`, the pod is removed on the client side, not the server side ([#20705](https://github.com/containers/podman/discussions/20705)).
- Fixed a bug where `podman machine rm -f` would cause a deadlock when running with WSL.
- Fixed `database is locked` errors with the new sqlite database backend ([#20809](https://github.com/containers/podman/issues/20809)).
- Fixed a bug where `podman-remote exec` would fail if the server API version is older than 4.8.0 ([#20821](https://github.com/containers/podman/issues/20821)).
- Fixed a bug where Podman would not run any command on systems with a symlinked $HOME ([#20872](https://github.com/containers/podman/issues/20872)).

## 4.8.0
### Features
- Podman machine now supports HyperV as a provider on Windows. This option can be set via the `CONTAINERS_MACHINE_PROVIDER` environment variable, or via containers.conf. HyperV requires Powershell to be run as Admin. Note that running WSL and HyperV machines at the same time is not supported.
- The `podman build` command now supports Containerfiles with heredoc syntax.
- The `podman login` and `podman logout` commands now support a new option, `--compat-auth-file`, which allows for editing Docker-compatible config files ([#18617](https://github.com/containers/podman/issues/18617)).
- The `podman machine init` and `podman machine set` commands now support a new option, `--usb`, which sets allows USB passthrough for the QEMU provider ([#16707](https://github.com/containers/podman/issues/16707)).
- The `--ulimit` option now supports setting -1 to indicate the maximum limit allowed for the current process ([#19319](https://github.com/containers/podman/issues/19319)).
- The `podman play kube` command now supports the `BUILDAH_ISOLATION` environment variable to change build isolation when the `--build` option is set ([#20024](https://github.com/containers/podman/issues/20024)).
- The `podman volume create` command now supports `--opt o=size=XYZ` on tmpfs file systems ([#20449](https://github.com/containers/podman/issues/20449)).
- The `podman info` command for remote calls now reports client information even if the remote connection is unreachable
- Added a new field, `privileged`, to containers.conf, which sets the defaults for the `--privileged` flag when creating, running or exec'ing into a container.
- The `podman kube play` command now supports setting DefaultMode for volumes ([#19313](https://github.com/containers/podman/issues/19313)).
- The `--opt` option to the `podman network create` command now accepts a new driver specific option, `vrf`, which assigns a VRF to the bridge interface.
- A new option `--rdt-class=COS` has been added to the `podman create` and `podman run` commands that enables assigning a container to a Class Of Service (COS). The COS has to be pre-configured based on a pseudo-filesystem created by the *resctrl* kernel driver that enables interacting with the Intel RDT CAT feature.
- The `podman kube play` command now supports a new option, `--publish-all`, which exposes all containerPorts on the host.
- The --filter option now supports `label!=`, which filters for containers without the specified label.

### Upcoming Deprecations
- We are beginning development on Podman 5.0, which will include a number of breaking changes and deprecations. We are still finalizing what will be done, but a preliminary list is below. Please note that none of these changes are present in Podman 4.8; this is a preview of upcoming changes.
- Podman 5.0 will deprecate the BoltDB database backend. Exact details on the transition to SQLite are still being decided - expect more news here soon.
- The containers.conf configuration file will be broken up into multiple separate files, ensuring that it will never be rewritten by Podman.
- Support for the CNI network backend and Cgroups V1 are being deprecated and gated by build tags. They will not be enabled in Podman builds by default.
- A variety of small breaking changes to the REST API are planned, both to improve Docker compatibility and to better support `containers.conf` settings when creating and managing containers.

### Changes
- Podman now defaults to sqlite as its database backend. For backwards compatibility, if a boltdb database already exists on the system, Podman will continue using it.
- RHEL Subscriptions from the host now flow through to quay.io/podman/* images.
- The `--help` option to the `podman push` command now shows the compression algorithm used.
- The remote Podman client’s `commit` command now shows progress messages ([#19947](https://github.com/containers/podman/issues/19947)).
- The `podman kube play` command now sets the pod hostname to the node/machine name when hostNetwork=true in k8s yaml ([#19321](https://github.com/containers/podman/issues/19321)).
- The `--tty,-t` option to the `podman exec` command now defines the TERM environment variable even if the container is not running with a terminal ([#20334](https://github.com/containers/podman/issues/20334)).
- Podman now also uses the `helper_binaries_dir` option in containers.conf to lookup the init binary (catatonit).
- Podman healthcheck events are now logged as notices.
- Podman machines no longer automatically update, preventing accidental service interruptions ([#20122](https://github.com/containers/podman/issues/20122)).
- The amount of CPUs a podman machine uses now defaults to available cores/2 ([#17066](https://github.com/containers/podman/issues/17066)).
- Podman machine now prohibits using provider names as machine names. `applehv`, `qemu`, `wsl`, and `hyperv` are no longer valid Podman machine names

### Quadlet
- Quadlet now supports the `UIDMap`, `GIDMap`, `SubUIDMap`, and `SubGIDMap` options in .container files.
- Fixed a bug where symlinks were not resolved in search paths ([#20504](https://github.com/containers/podman/issues/20504)).
- Quadlet now supports the `ReadOnlyTmpfs` option.
- The VolatileTmpfs option is now deprecated.
- Quadlet now supports systemd specifiers in User and Group keys.
- Quadlet now supports `ImageName` for .image files.
- Quadlet now supports a new option, `--force`, to the stop command.
- Quadlet now supports the `oneshot` service type for .kube files, which allows yaml files without containers.
- Quadlet now supports podman level arguments ([#20246](https://github.com/containers/podman/issues/20246)).
- Fixed a bug where Quadlet would crash when specifying non key-value options ([#20104](https://github.com/containers/podman/issues/20104)).
- Quadlet now removes anonymous volumes when removing a container ([#20070](https://github.com/containers/podman/issues/20070)).
- Quadlet now supports a new unit type, `.image`.

### Bugfixes
- Fixed a bug where mounted volumes on Podman machines on MacOS would have a max open files limit ([#16106](https://github.com/containers/podman/issues/16106)).
- Fixed a bug where setting both the `--uts` and `--network` options to `host` did not fill /etc/hostname with the host's name ([#20448](https://github.com/containers/podman/issues/20448)).
- Fixed a bug where the remote Podman client’s `build` command would incorrectly parse https paths ([#20475](https://github.com/containers/podman/issues/20475)).
- Fixed a bug where running Docker Compose against a WSL podman machine would fail ([#20373](https://github.com/containers/podman/issues/20373)).
- Fixed a race condition where parallel tagging and untagging of images would fail ([#17515](https://github.com/containers/podman/issues/17515)).
- Fixed a bug where the `podman exec` command would leak sessions when the specified command does not existFixed a bug where the `podman exec` command would leak sessions when the specified command does not exist ([#20392](https://github.com/containers/podman/issues/20392)).
- Fixed a bug where the `podman history` command did not display the size of certain layers ([#20375](https://github.com/containers/podman/issues/20375)).
- Fixed a bug where a container with a custom user namespace and `--restart always/on-failure` would not correctly cleanup the netnsm on restart, resulting in leaked ips and network namespaces ([#18615](https://github.com/containers/podman/issues/18615)).
- Fixed a bug where remote calls to the `podman top` command would incorrectly parse options ([#19176](https://github.com/containers/podman/issues/19176)).
- Fixed a bug where the `--read-only-tmpfs` option to the `podman run` command was incorrectly handled when the `--read-only` option was set ([#20225](https://github.com/containers/podman/issues/20225)).
- Fixed a bug where creating containers in parallel may cause a deadlock if both containers attempt to use the same named volume ([#20313](https://github.com/containers/podman/issues/20313)).
- Fixed a bug where a container restarted by the Podman service would occasionally not mount its storage ([#17042](https://github.com/containers/podman/issues/17042)).
- Fixed a bug where the `--filter` option to the `podman images` command would not correctly filter ids, digests, or intermediates ([#19966](https://github.com/containers/podman/issues/19966)).
- Fixed a bug where setting the `--replace` option to the `podman run` command would print both the old and new container ID. Now, only the new container ID is printed.
- Fixed a bug where the `podman machine ls` command would show Creation time as LastUp time for machines that have never been booted. Now, new machines show `Never`, with the json value being ZeroTime.
- Fixed a bug in the `podman build` command where the default pull policy was not set to `missing` ([#20125](https://github.com/containers/podman/issues/20125)).
- Fixed a bug where setting the static or volume directory in `containers.conf` would lead to cleanup errors ([#19938](https://github.com/containers/podman/issues/19938)).
- Fixed a bug where the `podman kube play` command exposed all containerPorts on the host ([#17028](https://github.com/containers/podman/issues/17028)).
- Fixed a bug where the `podman farm update` command did not verify farm and connection existence before updating ([#20080](https://github.com/containers/podman/issues/20080)).
- Fixed a bug where remote Podman calls would not honor the `--connection` option while the `CONTAINER_HOST` environment variable was set. The active destination is not resolved with the correct priority, that is, CLI flags, env vars, ActiveService from containers.conf, RemoteURI ([#15588](https://github.com/containers/podman/issues/15588)).
- Fixed a bug where the `--env-host` option was not honoring the default from containers.conf

### API
- Fixed a bug in the Compat Image Prune endpoint where the dangling filter was set twice ([#20469](https://github.com/containers/podman/issues/20469)).
- Fixed a bug in the Compat API where attempting to connect a container to a network while the connection already exists returned a 200 status code. It now correctly returns a 500 error code.
- Fixed a bug in the Compat API where some responses would not have compatible error details if progress data had not been sent yet ([#20013](https://github.com/containers/podman/issues/20013)).
- The Libpod Pull endpoint now supports a new option, compatMode which causes the streamed JSON payload to be identical to the Compat endpoint.
- Fixed a bug in the Libpod Container Create endpoint where it would return an incorrect status code if the image was not found. The endpoint now correctly returns 404.
- The Compat Network List endpoint should see a significant performance improvement ([#20035](https://github.com/containers/podman/issues/20035)).

### Misc
- Updated Buildah to v1.33.2
- Updated the containers/storage library to v1.51.0
- Updated the containers/image library to v5.29.0
- Updated the containers/common library to v0.57.0
- Updated the containers/libhvee library to v0.5.0
- Podman Machine now runs with gvproxy v0.7.1

## 4.7.2
### Security
- Fixed [GHSA-jq35-85cj-fj4p](https://github.com/moby/moby/security/advisories/GHSA-jq35-85cj-fj4p).

### Bugfixes
- WSL: Fixed `podman compose` command.
- Fixed a bug in `podman compose` to try all configured providers before throwing an error ([#20502](https://github.com/containers/podman/issues/20502)).

## 4.7.1
### Bugfixes
- Fixed a bug involving non-English locales of Windows where machine installs using user-mode networking were rejected due to erroneous version detection ([#20209](https://github.com/containers/podman/issues/20209)).
- Fixed a regression in --env-file handling ([#19565](https://github.com/containers/podman/issues/19565)).
- Fixed a bug where podman inspect would fail when stat'ing a device failed.

### API
- The network list compat API endpoint is now much faster ([#20035](https://github.com/containers/podman/issues/20035)).

## 4.7.0
### Security
- Now the io.containers.capabilities LABEL in an image can be an empty string.

### Features
- New command set: `podman farm [create,list,remove,update]` has been created to "farm" out builds to machines running Podman for different architectures.
- New command: `podman compose` as a thin wrapper around an external compose provider such as docker-compose or podman-compose.
- FreeBSD: `podman run --device` is now supported.
- Linux: Add a new `--module` flag for Podman.
- Podmansh: Timeout is now configurable using the `podmansh_timeout` option in containers.conf.
- SELinux: Add support for confined users to create containers but restrict them from creating privileged containers.
- WSL: Registers shared socket bindings on Windows, to allow other WSL distributions easy remote access ([#15190](https://github.com/containers/podman/issues/15190)).
- WSL: Enabling user-mode-networking on older WSL2 generations will now detect an error with upgrade guidance.
- The `podman build` command now supports two new options: `--layer-label` and `--cw`.
- The `podman kube generate` command now supports generation of k8s DaemonSet kind ([#18899](https://github.com/containers/podman/issues/18899)).
- The `podman kube generate` and `podman kube play` commands now support the k8s `TerminationGracePeriodSeconds` field ([RH BZ#2218061](https://bugzilla.redhat.com/show_bug.cgi?id=2218061)).
- The `podman kube generate` and `podman kube play` commands now support `securityContext.procMount: Unmasked` ([#19881](https://github.com/containers/podman/issues/19881)).
- The `podman generate kube` command now supports a `--podman-only` flag to allow podman-only reserved annotations to be used in the generated YAML file. These annotations cannot be used by Kubernetes.
- The `podman kube generate` now supports a `--no-trunc` flag that supports YAML files with annotations longer than 63 characters. Warning: if an annotation is longer than 63 chars, then the generated yaml file is not Kubernetes compatible.
- An infra name annotation `io.podman.annotations.infra.name` is added in the generated yaml when the `pod create` command has `--infra-name` set. This annotation can also be used with `kube play` when wanting to customize the infra container name ([#18312](https://github.com/containers/podman/issues/18312)).
- The syntax of `--uidmap` and `--gidmap` has been extended to lookup the parent user namespace and to extend default mappings ([#18333](https://github.com/containers/podman/issues/18333)).
- The `podman kube` commands now support the `List` kind ([#19052](https://github.com/containers/podman/issues/19052)).
- The `podman kube play` command now supports environment variables in kube.yaml ([#15983](https://github.com/containers/podman/issues/15983)).
- The `podman push` and `podman manifest push` commands now support the `--force-compression` optionto prevent reusing other blobs ([#18860](https://github.com/containers/podman/issues/18660)).
- The `podman manifest push` command now supports `--add-compression` to push with compressed variants.
- The `podman manifest push` command now honors the `add_compression` field from containers.conf if `--add-compression` is not set.
- The `podman run` and `podman create --mount` commands now support the `ramfs` type ([#19659](https://github.com/containers/podman/issues/19659)).
- When running under systemd (e.g., via Quadlet), Podman will extend the start timeout in 30 second steps up to a maximum of 5 minutes when pulling an image.
- The `--add-host` option now accepts the special string `host-gateway` instead of an IP Address, which will be mapped to the host IP address.
- The `podman generate systemd` command is deprecated.  Use Quadlet for running containers and pods under systemd.
- The `podman secret rm` command now supports an `--ignore` option.
- The `--env-file` option now supports multiline variables ([#18724](https://github.com/containers/podman/issues/18724)).
- The `--read-only-tmpfs` flag now affects /dev and /dev/shm as well as /run, /tmp, /var/tmp ([#12937](https://github.com/containers/podman/issues/12937)).
- The Podman `--mount` option now supports bind mounts passed as globs.
- The `--mount` option can now be specified in containers.conf using the `mounts` field.
- The `podman stats` now has an `--all` option to get all containers stats ([#19252](https://github.com/containers/podman/issues/19252)).
- There is now a new `--sdnotify=healthy` policy where Podman sends the READY message once the container turns healthy ([#6160](https://github.com/containers/podman/issues/6160)).
- Temporary files created when dealing with images in `/var/tmp` will automatically be cleaned up on reboot.
- There is now a new filter option `since` for `podman volume ls` and `podman volume prune` ([#19228](https://github.com/containers/podman/issues/19228)).
- The `podman inspect` command now has tab-completion support ([#18672])(https://github.com/containers/podman/issues/18672)).
- The `podman kube play` command now has support for the use of reserved annotations in the generated YAML.
- The progress bar is now displayed when decompressing a Podman machine image ([#19240](https://github.com/containers/podman/issues/19240)).
- The `podman secret inspect` command supports a new option `--showsecret` which will output the actual secret.
- The `podman secret create` now supports a `--replace` option, which allows you to modify secrets without replacing containers.
- The `podman login` command can now read the secret for a registry from its secret database created with `podman secret create` ([#18667]](https://github.com/containers/podman/issues/18667)).
- The remote Podman client’s `podman play kube` command now works with the `--userns` option ([#17392](https://github.com/containers/podman/pull/17392)).

### Changes
- The `/tmp` and `/var/tmp` inside of a `podman kube play` will no longer be `noexec`.
- The limit of inotify instances has been bumped from 128 to 524288 for podman machine ([#19848](https://github.com/containers/podman/issues/19848)).
- The `podman kube play` has been improved to only pull a newer image for the "latest" tag ([#19801](https://github.com/containers/podman/issues/19801)).
- Pulling from an `oci` transport will use the optional name for naming the image.
- The `podman info` command will always display the existence of the Podman socket.
- The echo server example in socket_activation.md has been rewritten to use quadlet instead of `podman generate systemd`.
- Kubernetes support table documentation correctly show volumes support.
- The `podman auto-update` manpage and documentation has been updated and now includes references to Quadlet.

### Quadlet
- Quadlet now supports setting Ulimit values.
- Quadlet now supports setting the PidsLimit option in a container.
- Quadlet unit files allow DNS field in Network group and DNS, DNSSearch, and DNSOption field in Container group ([#19884](https://github.com/containers/podman/issues/19884)).
- Quadlet now supports ShmSize option in unit files.
- Quadlet now recursively calls in user directories for unit files.
- Quadlet now allows the user to set the service working directory relative to the YAML or Unit files ([17177](https://github.com/containers/podman/discussions/17177)).
- Quadlet now allows setting user-defined names for `Volume` and `Network` units via the `VolumeName` and `NetworkName` directives, respectively.
- Kube quadlets can now support autoupdate.

### Bugfixes
- Fixed an issue where containers were being restarted after a `podman kill`.
- Fixed a bug where events could report incorrect healthcheck results ([#19237](https://github.com/containers/podman/issues/19237).
- Fixed a bug where running a container in a pod didn't fail if volumes or mounts were specified in the containers.conf file.
- Fixed a bug where pod cgroup limits were not being honored after a reboot ([#19175](https://github.com/containers/podman/issues/19175)).
- Fixed a bug where `podman rm -af` could fail to remove containers under some circumstances ([#18874](https://github.com/containers/podman/issues/18874)).
- Fixed a bug in rootless to clamp oom_score_adj to current value if it is too low ([#19829](https://github.com/containers/podman/issues/19829)).
- Fixed a bug where `--hostuser` was being parsed in base 8 instead of base 10 ([#19800](https://github.com/containers/podman/issues/19800)).
- Fixed a bug where `kube down` would error when an object did not exist ([#19711](https://github.com/containers/podman/issues/19711)).
- Fixed a bug where containers created via DOCKER API without specifying StopTimeout had StopTimeout defaulting to 0 seconds ([#19139](https://github.com/containers/podman/issues/19139)).
- Fixed a bug in `podman exec` to set umask to match the container it's execing into ([#19713](https://github.com/containers/podman/issues/19713)).
- Fixed a bug where `podman kube play` failed to set a container's Umask to the default `0022`.
- Fixed a bug to automatically reassign Podman's machine ssh port on Windows when it conflicts with in-use system ports ([#19554](https://github.com/containers/podman/issues/19554)).
- Fixed a bug where locales weren't passed to conmon correctly, resulting in a crash if some characters were specified over CLI ([containers/common/#272](https://github.com/containers/conmon/issues/272)).
- Fixed a bug where `podman top` would sometimes not print the full output ([#19504](https://github.com/containers/podman/issues/19504)).
- Fixed a bug were `podman logs --tail` could return incorrect lines when the k8s-file logger is used ([#19545](https://github.com/containers/podman/issues/19545)).
- Fixed a bug where `podman stop` did not ignore cidfile not existing when user specified --ignore flag ([#19546](https://github.com/containers/podman/issues/19546)).
- Fixed a bug where a container with an image volume and an inherited mount from the `--volumes-from` option that used the same path could not be created ([#19529](https://github.com/containers/podman/issues/19529)).
- Fixed a bug where `podman cp` via STDIN did not delete temporary files ([#19496](https://github.com/containers/podman/issues/19496)).
- Fixed a bug where Compatibility API did not accept timeout=-1 for stopping containers ([#17542](https://github.com/containers/podman/issues/17542)).
- Fixed a bug where `podman run --rmi` did not remove the container ([#15640](https://github.com/containers/podman/issues/15640)).
- Fixed a bug to recover from inconsistent podman-machine states with QEMU ([#16054](https://github.com/containers/podman/issues/16054)).
- Fixed a bug where CID Files on remote clients are not removed when container is removed ([#19420](https://github.com/containers/podman/issues/19420)).
- Fixed a bug in `podman inspect` to show a `.NetworkSettings.SandboxKey` path for containers created with --net=none ([#16716](https://github.com/containers/podman/issues/16716)).
- Fixed a concurrency bug in `podman machine start` using the QEMU provider ([#18662](https://github.com/containers/podman/issues/18662)).
- Fixed a bug in `podman run` and `podman create` where the command fails if the user specifies a non-existent authfile path ([#18938](https://github.com/containers/podman/issues/18938)).
- Fixed a bug where some distributions added extra quotes around the distribution name removed from `podman info` output ([#19340](https://github.com/containers/podman/issues/19340)).
- Fixed a crash validating --device argument for create and run ([#19335](https://github.com/containers/podman/issues/19335)).
- Fixed a bug where `.HostConfig.PublishAllPorts` always evaluates to `false` when inspecting a container created with `--publish-all`.
- Fixed a bug in `podman image trust` command to allow using the local policy.json file ([#19073](https://github.com/containers/podman/issues/19073)).
- Fixed a bug where the cgroup file system was not correctly mounted when running without a network namespace in rootless mode ([#20073](https://github.com/containers/podman/issues/20073)).
- Fixed a bug where the `--syslog` flag was not passed to the cleanup process.

### API
- Fixed a bug with parsing of the pull query parameter for the compat /build endpoint ([#17778](https://github.com/containers/podman/issues/17778)).

### Misc
- Updated Buildah to v1.32.0.

## 4.6.2
### Changes
- Fixed a performance issue when calculating diff sizes in overlay. The `podman system df` command should see a significant performance improvement ([#19467](https://github.com/containers/podman/issues/19467)).

### Bugfixes
- Fixed a bug where containers in a pod would use pod the restart policy over the set container restart policy ([#19671](https://github.com/containers/podman/issues/19671)).

### API
- Fixed a bug in the Compat Build endpoint where the pull query parameter did not parse 0/1 as a boolean ([#17778](https://github.com/containers/podman/issues/17778)).

### Misc
- Updated the containers/storage library to v1.48.1

## 4.6.1
### Quadlet
- Quadlet now selects the first Quadlet file found when multiple Quadlets exist with the same name.

### API
- Fixed a bug in the container kill endpoint to correctly return 409 when a container is not running ([#19368](https://github.com/containers/podman/issues/19368)).

### Misc
- Updated Buildah to v1.31.2
- Updated the containers/common library to v0.55.3

## 4.6.0
### Features
- The `podman manifest inspect` command now supports the `--authfile` option, for authentication purposes.
- The `podman wait` command now supports `--condition={healthy,unhealthy}`, allowing waits on successful health checks.
- The `podman push` command now supports a new option, ` --compression-level`, which specifies the compression level to use ([#18939](https://github.com/containers/podman/issues/18939)).
- The `podman machine start` command, when run with `--log-level=debug`, now creates a console window to display the virtual machine while booting.
- Podman now supports a new option, `--imagestore`, which allows images to be stored in a different directory than the graphroot.
- The `--ip-range` option to the `podman network create` command now accepts a new syntax, `<startIP>-<endIP>`, which allows more flexibility when limiting the ip range that Podman assigns.
- [Tech Preview] A new command, `podmansh`, has been added, which executes a user shell within a container when the user logs into the system. The container that the users get added to can be defined via a Podman Quadlet file. This feature is currently a `Tech Preview` which means it's ready for users to try out but changes can be expected in upcoming versions.
- The `podman network create` command supports a new `--option`, `bclim`, for the `macvlan` driver.
- The `podman network create` command now supports adding static routes using the `--route` option.
- The `podman network create` command supports a new `--option`, `no_default_route` for all drivers.
- The `podman info` command now prints network information about the binary path, package version, program version and DNS information ([#18443](https://github.com/containers/podman/issues/18443)).
- The `podman info` command now displays the number of free locks available, helping to debug lock exhaustion scenarios.
- The `podman info` command now outputs information about pasta, if it exists in helper_binaries_dir or $PATH.
- The remote Podman client’s `podman build` command now accepts Containerfiles that are not in the context directory ([#18239](https://github.com/containers/podman/issues/18239)).
- The remote Podman client’s `podman play kube` command now supports the `--configmap` option ([#17513](https://github.com/containers/podman/issues/17513)).
- The `podman kube play` command now supports multi-doc YAML files for configmap arguments. ([#18537](https://github.com/containers/podman/issues/18537)).
- The `podman pod create` command now supports a new flag, `--restart`, which sets the restart policy for all the containers in a pod.
- The `--format={{.Restarts}}` option to the `podman ps` command now shows the number of times a container has been restarted based on its restart policy.
- The `--format={{.Restarts}}` option to the `podman pod ps` command now shows the total number of container restarts in a pod.
- The podman machine provider can now be specified via the `CONTAINERS_MACHINE_PROVIDER` environment variable, as well as via the `provider` field in `containers.conf` ([#17116](https://github.com/containers/podman/issues/17116)).
- A default list of pasta arguments can now be set in `containers.conf` via `pasta_options`.
- The `podman machine init` and `podman machine set` commands now support a new option, `--user-mode-networking`, which improves interops with VPN configs that drop traffic from WSL networking, on Windows.
- The remote Podman client’s `podman push` command now supports the `--digestfile` option ([#18216](https://github.com/containers/podman/issues/18216)).
- Podman now supports a new option, `--out`, that allows redirection or suppression of STDOUT ([#18120](https://github.com/containers/podman/issues/18120)).

### Changes
- When looking up an image by digest, the entire repository of the specified value is now considered. This aligns with Docker's behavior since v20.10.20. Previously, both the repository and the tag was ignored and Podman looked for an image with only a matching digest. Ignoring the name, repository, and tag of the specified value can lead to security issues and is considered harmful.
- The `podman system service` command now emits a warning when binding to a TCP socket. This is not a secure configuration and the Podman team recommends against using it.
- The `podman top` command no longer depends on ps(1) being present in the container image and now uses the one from the host ([#19001](https://github.com/containers/podman/issues/19001)).
- The `--filter id=xxx` option will now treat `xxx` as a CID prefix, and not as a regular expression ([#18471](https://github.com/containers/podman/issues/18471)).
- The `--filter` option now requires multiple `--filter` flags to specify multiple filters. It will no longer support the comma syntax (`--filter label=a,label=b`).
- The `slirp4netns` binary for will now be searched for in paths specified by the `helper_binaries_dir` option in `containers.conf` ([#18239](https://github.com/containers/podman/issues/18568)).
- Podman machine now updates `/run/docker.sock` within the guest to be consistent with its rootless/rootful setting ([#18480](https://github.com/containers/podman/issues/18480)).
- The `podman system df` command now counts files which podman generates for use with specific containers as part of the disk space used by those containers, and which can be reclaimed by removing those containers. It also counts space used by files it associates with specific images and volumes as being used by those images and volumes.
- The `podman build` command now returns a clearer error message when the Containerfile cannot be found. ([#16354](https://github.com/containers/podman/issues/16354)).
- Containers created with `--pid=host` will no longer print errors on podman stop ([#18460](https://github.com/containers/podman/issues/18460)).
- The `podman manifest push` command no longer requires a destination to be specified. If a destination is not provided, the source is used as the destination ([#18360](https://github.com/containers/podman/issues/18360)).
- The `podman system reset` command now warns the user that the graphroot and runroot directories will be deleted ([#18349](https://github.com/containers/podman/issues/18349)), ([#18295](https://github.com/containers/podman/issues/18295)).
- The `package` and `package-install` targets in Makefile have now been fixed and also renamed to `rpm` and `rpm-install` respectively for clarity ([#18817](https://github.com/containers/podman/issues/18817)).

### Quadlet
- Quadlet now exits with a non-zero exit code when errors are found ([#18778](https://github.com/containers/podman/issues/18778)).
- Rootless podman quadlet files can now be installed in `/etc/containers/systemd/users` directory.
- Quadlet now supports the `AutoUpdate` option.
- Quadlet now supports the `Mask` and `Unmask` options.
- Quadlet now supports the `WorkingDir` option, which specifies the default working dir in a container.
- Quadlet now supports the `Sysctl` option, which sets namespaced kernel parameters for containers ([#18727](https://github.com/containers/podman/issues/18727)).
- Quadlet now supports the `SecurityLabelNetsted=true` option, which allows nested SELinux containers.
- Quadlet now supports the `Pull` option in `.container` files ([#18779](https://github.com/containers/podman/issues/18779)).
- Quadlet now supports the `ExitCode` field in `.kube` files, which reflects the exit codes of failed containers.
- Quadlet now supports `PodmanArgs` field.
- Quadlet now supports the `HostName` field, which sets the container's host name, in `.container` files ([#18486](https://github.com/containers/podman/issues/18486)).

### Bugfixes
- Fixed a bug where the `podman machine start` command would fail with a 255 exit code. It now waits for systemd-user sessions to be up, and for SSH to be ready, addressing the flaky machine starts ([#17403](https://github.com/containers/podman/issues/17403)).
- Fixed a bug where the `podman auto update` command did not correctly use authentication files when contacting container registries.
- Fixed a bug where `--label` option to the `podman volume ls` command would return volumes that matched any of the filters, not all of them  ([#19219](https://github.com/containers/podman/issues/19219)).
- Fixed a bug where the `podman kube play` command did not recognize containerPort names inside Kubernetes liveness probes. Now, liveness probes support both containerPort names as well as port numbers ([#18645](https://github.com/containers/podman/issues/18645)).
- Fixed a bug where the `--dns` option to the `podman run` command was ignored for macvlan networks ([#19169](https://github.com/containers/podman/issues/19169)).
- Fixed a bug in the `podman system service` command where setting LISTEN_FDS when listening on TCP would misbehave.
- Fixed a bug where hostnames were not recognized as a network alias. Containers can now resolve other hostnames, in addition to their names ([#17370](https://github.com/containers/podman/issues/17370)).
- Fixed a bug where the `podman pod run` command would error after a reboot on a non-systemd system ([#19175](https://github.com/containers/podman/issues/19175)).
- Fixed a bug where the `--syslog` option returned a fatal error when no syslog server was found ([#19075](https://github.com/containers/podman/issues/19075)).
- Fixed a bug where the `--mount` option would parse the `readonly` option incorrectly ([#18995](https://github.com/containers/podman/issues/18995)).
- Fixed a bug where hook executables invoked by the `podman run` command set an incorrect working directory. It now sets the correct working directory pointing to the container bundle directory ([#18907](https://github.com/containers/podman/issues/18907)).
- Fixed a bug where the `-device-cgroup-rule` option was silently ignored in rootless mode ([#18698](https://github.com/containers/podman/issues/18698)).
- Listing images is now more resilient towards concurrently running image removals.
- Fixed a bug where the `--force` option to the `podman kube down` command would not remove volumes ([#18797](https://github.com/containers/podman/issues/18797)).
- Fixed a bug where setting the `--list-tags` option in the `podman search` command would cause the command to ignore the `--format` option ([#18939](https://github.com/containers/podman/issues/18939)).
- Fixed a bug where the `podman machine start` command did not properly translate the proxy IP.
- Fixed a bug where the `podman auto-update` command would not restart dependent units (specified via `Requires=`) on auto update ([#18926](https://github.com/containers/podman/issues/18926)).
- Fixed a bug where the `podman pull` command would print ids multiple times when using additional stores ([#18647](https://github.com/containers/podman/issues/18647)).
- Fixed a bug where creating a container while setting unmask option to an empty array would cause the create to fail ([#18848](https://github.com/containers/podman/issues/18848)).
- Fixed a bug where the propagation of proxy settings for QEMU VMs was broken.
- Fixed a bug where the `podman rm -fa` command could fail to remove dependency containers such as pod infra containers ([#18180](https://github.com/containers/podman/issues/18180)).
- Fixed a bug  where ` --tz` option to the `podman create ` and `podman run` commands would not create a proper localtime symlink to the zoneinfo file, which was causing some applications (e.g. java) to not read the timezone correctly.
- Fixed a bug where lowering the ulimit after container creation would cause the container to fail ([#18714](https://github.com/containers/podman/issues/18714)).
- Fixed a bug where signals were not forwarded correctly in rootless containers ([#16091](https://github.com/containers/podman/issues/16091)).
- Fixed a bug where the `--filter volume=` option to the `podman events` command would not display the relevant events ([#18618](https://github.com/containers/podman/issues/18618)).
- Fixed a bug in the `podman wait` command where containers created with the `--restart=always` option would result in the container staying in a stopped state.
- Fixed a bug where the `podman stats` command returned an incorrect memory limit after a `container update`. ([#18621](https://github.com/containers/podman/issues/18621)).
- Fixed a bug in the `podman run` command where the `PODMAN_USERNS` environment variable was not ignored when the `--pod` option was set, resulting in a container created in a different user namespace than its pod ([#18580](https://github.com/containers/podman/issues/18580)).
- Fixed a bug where the `podman run` command would not create the `/run/.containerenv` when the tmpfs is mounted on `/run` ([#18531](https://github.com/containers/podman/issues/18531)).
- Fixed a bug where the `$HOME` environment variable would be configured inconsistently between container starts if a new passwd entry had to be created for the container.
- Fixed a bug where the `podman play kube` command would restart initContainers based on the restart policy of the pod. initContainers should never be restarted.
- Fixed a bug in the remote Podman client’s `build` command where an invalid platform would be set.
- Fixed a bug where the `podman history` command did not display tags ([#17763](https://github.com/containers/podman/issues/17763)).
- Fixed a bug where the `podman machine init` command would create invalid machines when run with certain UIDs ([#17893](https://github.com/containers/podman/issues/17893)).
- Fixed a bug in the remote Podman client’s `podman manifest push` command where an error encountered during the push incorrectly claimed that the error occurred while adding an item to the list.
- Fixed a bug where the `podman machine rm` command would remove the machine connection before the user confirms the removal of the machine ([#18330](https://github.com/containers/podman/issues/18330)).
- Fixed a bug in the sqlite database backend where the first read access may fail ([#17859](https://github.com/containers/podman/issues/17859)).
- Fixed a bug where a podman machine could get stuck in the `starting` state ([#16945](https://github.com/containers/podman/issues/16945)).
- Fixed a bug where running a container with the `--network=container:` option would fail when the target container uses the host network mode. The same also now works for the other namespace options (`--pid`, `--uts`, `--cgroupns`, `--ipc`) ([#18027](https://github.com/containers/podman/issues/18027)).
- Fixed a bug where the `--format {{.State}}` option to the `podman ps` command would display the status rather than the state ([#18244](https://github.com/containers/podman/issues/18244)).
- Fixed a bug in the `podman commit` command where setting a `--message` while also specifying `--format=docker` options would incorrectly warn that setting a message is incompatible with OCI image formats ([#17773](https://github.com/containers/podman/issues/17773)).
- Fixed a bug in the `--format` option to the `podman history` command, where the `{{.CreatedAt}}` and `{{.Size}}` fields were inconsistent with Docker’s output ([#17767](https://github.com/containers/podman/issues/17767)), ([#17768](https://github.com/containers/podman/issues/17768)).
- Fixed a bug in the remote Podman client where filtering containers would not return all matching containers ([#18153](https://github.com/containers/podman/issues/18153)).

### API
- Fixed a bug where the Compat and Libpod Top endpoints for Containers did not correctly report errors.
- Fixed a bug in the Compat Pull and Compat Push endpoints where errors were incorrectly handled.
- Fixed a bug in the Compat Wait endpoint to correctly handle the "removed" condition ([#18889](https://github.com/containers/podman/issues/18889)).
- Fixed a bug in the Compat Stats endpoint for Containers where the `online_cpus` field was not set correctly ([#15754](https://github.com/containers/podman/issues/15754)).
- Fixed a bug in the Compat Build endpoint where the pull field accepted a boolean value instead of a string ([#17778](https://github.com/containers/podman/issues/17778)).
- Fixed a bug where the Compat History endpoint for Images did not prefix the image ID with `sha256:` ([#17762](https://github.com/containers/podman/issues/17762)).
- Fixed a bug in the Libpod Export endpoint for Images where exporting to an oci-dir or a docker-dir format would not export to the correct format ([#15897](https://github.com/containers/podman/issues/15897)).
- The Compat Create endpoint for Containers now supports the `platform` parameter ([#18951](https://github.com/containers/podman/issues/18951)).
- The Compat Remove endpoint for Images now supports the `noprune` query parameter, which ensures that dangling parents of the specified image are not removed
- The Compat Info endpoint now reports running rootless and SELinux enabled as security options.
- Fixed a bug in the Auth endpoint where a nil dereference could potentially occur.

### Misc
- The `podman system service` command is now supported on FreeBSD.
- Updated the Mac pkginstaller QEMU to v8.0.0
- Updated Buildah to v1.31.0
- Updated the containers/storage library to v1.48.0
- Updated the containers/image library to v5.26.1
- Updated the containers/common library to v0.55.2

## 4.5.1
### Security
- Do not include image annotations when building spec. These annotations can have security implications - crun, for example, allows rootless containers to preserve the user's groups through an annotation.

### Quadlet
- Fixed a bug in quadlet to recognize the systemd optional prefix '-'.

### Bugfixes
- Fixed a bug where fully resolving symlink paths included the version number, breaking the path to homebrew-installed qemu files ([#18111](https://github.com/containers/podman/issues/18111)).
- Fixed a bug where Podman was splitting the filter map slightly differently compared to Docker ([#18092](https://github.com/containers/podman/issues/18092)).
- Fixed a bug where running `make package` did not work on RHEL 8 environments ([#18421](https://github.com/containers/podman/issues/18421)).
- Fixed a bug to allow comma separated dns server IP addresses in `podman network create --dns` and `podman network update --dns-add/--dns-drop` ([#18663](https://github.com/containers/podman/pull/18663)).
- Fixed a bug to correctly stop containers created with --restart=always in all cases ([#18259](https://github.com/containers/podman/issues/18259)).
- Fixed a bug in podman-remote logs to correctly display errors reported by the server.
- Fixed a bug to correctly tear down the network stack again when an error happened during the setup.
- Fixed a bug in the remote API exec inspect call to correctly display updated information, e.g. when the exec process died ([#18424](https://github.com/containers/podman/issues/18424)).
- Fixed a bug so that podman save on Windows can now write to stdout by default ([#18147](https://github.com/containers/podman/issues/18147)).
- Fixed a bug where podman machine rm with the qemu backend now correctly removes the machine connection after the confirmation message not before ([#18330](https://github.com/containers/podman/issues/18330)).
- Fixed a problem where podman machine connections would try to connect to the ipv6 localhost ipv6 (::1) ([#16470](https://github.com/containers/podman/issues/16470)).

### API
- Fixed a bug in the compat container create endpoint which could result in a "duplicate mount destination" error when the volume path was not "clean", e.g. included a final slash at the end. ([#18454](https://github.com/containers/podman/issues/18454)).
- The compat API now correctly accepts a tag in the images/create?fromSrc endpoint ([#18597](https://github.com/containers/podman/issues/18597)).

## 4.5.0
### Features
- The `podman kube play` command now supports the hostIPC field ([#17157](https://github.com/containers/podman/issues/17157)).
- The `podman kube play` command now supports a new flag, `--wait`, that keeps the workload running in foreground until killed with a sigkill or sigterm. The workloads are cleaned up and removed when killed ([#14522](https://github.com/containers/podman/issues/14522)).
- The `podman kube generate` and `podman kube play` commands now support SELinux filetype labels.
- The `podman kube play` command now supports sysctl options ([#16711](https://github.com/containers/podman/issues/16711)).
- The `podman kube generate` command now supports generating the Deployments ([#17712](https://github.com/containers/podman/issues/17712)).
- The `podman machine inspect` command now shows information about named pipe addresses on Windows ([#16860](https://github.com/containers/podman/issues/16860)).
- The `--userns=keep-id` option for `podman create`, ` run`, and `kube play` now works for root containers by copying the current mapping into a new user namespace ([#17337](https://github.com/containers/podman/issues/17337)).
- A new command has been added, `podman secret exists`, to verify if a secret with the given name exists.
- The `podman kube generate` and `podman kube play` commands now support ulimit annotations ([#16404](https://github.com/containers/podman/issues/16404)).
- The `podman create`, `run`, `pod create`, and `pod clone` commands now support a new option, `--shm-size-systemd`, that allows limiting tmpfs sizes for systemd-specific mounts ([#17037](https://github.com/containers/podman/issues/17037)).
- The `podman create` and `run` commands now support a new option, `--group-entry` which customizes the entry that is written to the `/etc/group` file within the container when the `--user` option is used ([#14965](https://github.com/containers/podman/issues/14965)).
- The `podman create` and `podman run` commands now support a new option, `--security-opt label=nested`, which allows SELinux labeling within a confined container.
- A new command, `podman machine os apply` has been added, which applies OS changes to a Podman machine, from an OCI image.
- The `podman search` command now supports two new options: `--cert-dir` and `--creds`.
- Defaults for the `--cgroup-config` option for `podman create` and `podman run` can now be set in `containers.conf`.
- Podman now supports auto updates for containers running inside a pod ([#17181](https://github.com/containers/podman/issues/17181)).
- Podman can now use a SQLite database as a backend for increased stability. The default remains the old database, BoltDB. The database to use is selected through the `database_backend` field in `containers.conf`.
- Netavark plugin support is added, the netavark network backend now allows users to create custom network drivers. `podman network create -d <plugin>` can be used to create a network config for your plugin and then podman will use it like any other config and takes care of setup/teardown on container start/stop. This requires at least netavark version 1.6.

### Changes
- Remote builds using the `podman build` command no longer allows `.containerignore` or `.dockerignore` files to be symlinks outside the build context.
- The `podman system reset` command now clears build caches.
- The `podman play kube` command now adds ctrName as an alias to the pod network ([#16544](https://github.com/containers/podman/issues/16544)).
- The `podman kube generate` command no longer adds hostPort to the pod spec when generating service kinds.
- Using a private cgroup namespace with systemd containers on a cgroups v1 system will explicitly error (this configuration has never worked) ([#17727](https://github.com/containers/podman/issues/17727)).
- The `SYS_CHROOT` capability has been re-added to the default set of capabilities.
- Listing large quantities of images with the `podman images` command has seen a significant performance improvement ([#17828](https://github.com/containers/podman/issues/17828)).

### Quadlet
- Quadlet now supports the `Rootfs=` option, allowing containers to be based on rootfs in addition to image.
- Quadlet now supports the Secret key in the Container group.
- Quadlet now supports the Logdriver key in `.container` and `.kube` units.
- Quadlet now supports the Mount key in `.container` files ([#17632](https://github.com/containers/podman/issues/17632)).
- Quadlet now supports specifying static IPv4 and IPv6 addresses in `.container` files via the IP= and IP6= options.
- Quadlet now supports health check configuration in `.container` files.
- Quadlet now supports relative paths in the Volume key in .container files ([#17418](https://github.com/containers/podman/issues/17418)).
- Quadlet now supports setting the UID and GID options for `--userns=keep-id` ([#17908](https://github.com/containers/podman/issues/17908)).
- Quadlet now supports adding `tmpfs` filesystems through the `Tmpfs` key in `.container` files ([#17907](https://github.com/containers/podman/issues/17907)).
- Quadlet now includes a `--version` option.
- Quadlet now forbids specifying SELinux label types, including disabling selinux separation.
- Fixed a bug where Quadlet did not recognize paths starting with systemd specifiers as absolute ([#17906](https://github.com/containers/podman/issues/17906)).

### Bugfixes
- Fixed a bug in the network list API where a race condition would cause the list to fail if a container had just been removed ([#17341](https://github.com/containers/podman/issues/17341)).
- Fixed a bug in the `podman image scp` command to correctly use identity settings.
- Fixed a bug in the remote Podman client's `podman build` command where building from stdin would fail.  `podman --remote build -f -` now works correctly ([#17495](https://github.com/containers/podman/issues/17495)).
- Fixed a bug in the `podman volume prune` command where exclusive (`!=`) filters would fail ([#17051](https://github.com/containers/podman/issues/17051)).
- Fixed a bug in the `--volume` option in the `podman create`, `run`, `pod create`, and `pod clone` commands where specifying relative mappings or idmapped mounts would fail ([#17517](https://github.com/containers/podman/issues/17517)).
- Fixed a bug in the `podman kube play` command where a secret would be created, but nothing would be printed on the terminal ([#17071](https://github.com/containers/podman/issues/17071)).
- Fixed a bug in the `podman kube down` command where secrets were not removed.
- Fixed a bug where cleaning up after an exited container could segfault on non-Linux operating systems.
- Fixed a bug where the `podman inspect` command did not properly list the network configuration of containers created with `--net=none` or `--net=host` ([#17385](https://github.com/containers/podman/issues/17385)).
- Fixed a bug where containers created with user-specified SELinux labels that created anonymous or named volumes would create those volumes with incorrect labels.
- Fixed a bug where the `podman checkpoint restore` command could panic.
- Fixed a bug in the `podman events` command where events could be returned more than once after a log file rotation ([#17665](https://github.com/containers/podman/issues/17665)).
- Fixed a bug where errors from systemd when restarting units during a `podman auto-update` command were not reported.
- Fixed a bug where containers created with the `--health-on-failure=restart` option were not restarting when the health state turned unhealthy ([#17777](https://github.com/containers/podman/issues/17777)).
- Fixed a bug where containers using the `slirp4netns` network mode with the `cidr` option and a custom user namespace did not set proper DNS IPs in `resolv.conf`.
- Fixed a bug where the `podman auto-update` command could fail to restart systemd units ([#17607](https://github.com/containers/podman/issues/17607)).
- Fixed a bug where the `podman play kube` command did not properly handle `secret.items` in volumes ([#17829](https://github.com/containers/podman/issues/17829)).
- Fixed a bug where the `podman generate kube` command could generate pods with invalid names and hostnames ([#18054](https://github.com/containers/podman/issues/18054)).
- Fixed a bug where names of limits (such as `RLIMIT_NOFILE`) passed to the `--ulimit` option to `podman create` and `podman run` were case-sensitive ([#18077](https://github.com/containers/podman/issues/18077)).

### API
- The Compat Stats endpoint for Containers now returns the `Id` key as lowercase `id` to match Docker ([#17869](https://github.com/containers/podman/issues/17869)).

### Misc
- The `podman version` command no longer joins the rootless user namespace ([#17657](https://github.com/containers/podman/issues/17657)).
- The `podman-events --stream` option is no longer hidden and is now documented.

## 4.4.4
### Changes
- Podman now writes direct mappings for idmapped mounts.

### Bugfixes
- Fixed a regression which caused the MacOS installer to fail if podman-mac-helper was already installed ([#17910](https://github.com/containers/podman/issues/17910)).

## 4.4.3
### Security
- This release fixes CVE-2022-41723, a vulnerability in the golang.org/x/net package where a maliciously crafted HTTP/2 stream could cause excessive CPU consumption, sufficient to cause a denial of service.

### Changes
- Added `SYS_CHROOT` back to the default set of capabilities.

### Bugfixes
- Fixed a bug where quadlet would not use the default runtime set.
- Fixed a bug where `podman system service --log-level=trace` did not hijack the client connection, causing remote `podman run/attach` calls to work incorrectly ([#17749](https://github.com/containers/podman/issues/17749)).
- Fixed a bug where the podman-mac-helper returned an incorrect exit code after erroring. `podman-mac-helper` now exits with 1 on error ([#17785](https://github.com/containers/podman/issues/17785)).
- Fixed a bug where `podman run --dns ... --network` would not respect the dns option. Podman will no longer add host nameservers to resolv.conf when aardvark-dns is used ([#17499](https://github.com/containers/podman/issues/17499)).
- Fixed a bug where `podman logs` errored out with the passthrough driver when the container was run from a systemd service.
- Fixed a bug where `--health-on-failure=restart` would not restart the container when the health state turned unhealthy ([#17777](https://github.com/containers/podman/issues/17777)).
- Fixed a bug where podman machine VMs could have their system time drift behind real time. New machines will no longer be affected by this ([#11541](https://github.com/containers/podman/issues/11541)).

### API
- Fixed a bug where creating a network with the Compat API would return an incorrect status code. The API call now returns 409 when creating a network with an existing name and when CheckDuplicate is set to true ([#17585](https://github.com/containers/podman/issues/17585)).
- Fixed a bug in the /auth REST API where logging into Docker Hub would fail ([#17571](https://github.com/containers/podman/issues/17571)).

### Misc
- Updated the containers/common library to v0.51.1
- Updated the Mac pkginstaller QEMU to v7.2.0

## 4.4.2
### Security
- This release fixes CVE-2023-0778, which allowed a malicious user to potentially replace a normal file in a volume with a symlink while exporting the volume, allowing for access to arbitrary files on the host file system.

### Bugfixes
- Fixed a bug where containers started via the `podman-kube` systemd template would always use the "passthrough" log driver ([#17482](https://github.com/containers/podman/issues/17482)).
- Fixed a bug where pulls would unexpectedly encounter an EOF error. Now, Podman automatically transparently resumes aborted pull connections.
- Fixed a race condition in Podman's signal proxy.

### Misc
- Updated the containers/image library to v5.24.1.

## 4.4.1
### Changes
- Added the `podman-systemd.unit` man page, which can also be displayed using `man quadlet` ([#17349](https://github.com/containers/podman/issues/17349)).
- Documented journald identifiers used in the journald backend for the `podman events` command.
- Dropped the CAP_CHROOT, CAP_AUDIT_WRITE, CAP_MKNOD, CAP_MKNOD default capabilities.

### Bugfixes
- Fixed a bug where the default handling of pids-limit was incorrect.
- Fixed a bug where parallel calls to `make docs` crashed ([#17322](https://github.com/containers/podman/issues/17322)).
- Fixed a regression in the `podman kube play` command where existing resources got mistakenly removed.

## 4.4.0
### Features
- Introduce Quadlet, a new systemd-generator that easily writes and maintains systemd services using Podman.
- The `podman kube play` command now supports hostPID in the pod.spec ([#17157](https://github.com/containers/podman/issues/17157)).
- The `podman build` command now supports the `--group-add` option.
- A new command, `podman network update` has been added, which updates networks for containers and pods.
- The `podman network create` command now supports a new option, `--network-dns-server`, which sets the DNS servers that this network will use.
- The `podman kube play` command now accepts the`--publish` option, which sets or overrides port publishing.
- The `podman inspect` command now returns an error field ([#13729](https://github.com/containers/podman/issues/13729)).
- The `podman update` command now accepts the `--pids-limit` option, which sets the PIDs limit for a container ([#16543](https://github.com/containers/podman/issues/16543)).
- Podman now supports container names beginning with a `/` to match Docker behaviour ([#16663](https://github.com/containers/podman/issues/16663)).
- The `podman events` command now supports `die` as a value (mapping to `died`) to the `--filter` option, for better Docker compatibility ([#16857](https://github.com/containers/podman/issues/16857)).
- The `podman system df`command’s `--format "{{ json . }}"` option now outputs human-readable format to improve Docker compatibility
- The `podman rm -f` command now also terminates containers in "stopping" state.
- Rootless privileged containers will now mount all tty devices, except for the virtual-console related tty devices (/dev/tty[0-9]+) ([#16925](https://github.com/containers/podman/issues/16925)).
- The `podman play kube` command now supports subpaths when using configmap and hostpath volume types  ([#16828](https://github.com/containers/podman/issues/16828)).
- All commands with the `--no-heading` option now include a short option, `-n`.
- The `podman push` command no longer ignores the hidden `--signature-policy` flag.
- The `podman wait` command now supports the `--ignore` option.
- The `podman network create` command now supports the `--ignore` option to instruct Podman to not fail when trying to create an already existing network.
- The `podman kube play` command now supports volume subpaths when using named volumes ([#12929](https://github.com/containers/podman/issues/12929)).
- The `podman kube play` command now supports container startup probes.
- A new command, `podman buildx version`, has been added, which shows the buildah version ([#16793](https://github.com/containers/podman/issues/16793)).
- Remote usage of the `podman build` command now supports the  `--volume` option ([#16694](https://github.com/containers/podman/issues/16694)).
- The `--opt parent=...` option is now accepted with the ipvlan network driver in the `podman network create` command ([#16621](https://github.com/containers/podman/issues/16621)).
- The `--init-ctr` option for the `podman container create` command now supports shell completion.
- The `podman kube play` command run with a readOnlyTmpfs Flag in the kube YAML can now write to tmpfs inside of the container.
- The `podman run` command has been extended with support for checkpoint images.
- When the new `event_audit_container_create` option is enabled in containers.conf, the verbosity of the container-create event is increased by adding the inspect data of the container to the event.
- Containers can now have startup healthchecks, allowing a command to be run to ensure the container is fully started before the regular healthcheck is activated.
- CDI devices can now be specified in containers.conf ([#16232](https://github.com/containers/podman/issues/16232)).
- The `podman push` command features two new options, `--encryption-key` and `--encrypt-layer`, for encrypting an image while pushing it to a registry ([#15163](https://github.com/containers/podman/issues/15163)).
- The `podman pull` and `podman run` commands feature a new option, `--decryption-key`, which decrypts the image while pulling it from a registry ([#15163](https://github.com/containers/podman/issues/15163)).
- Remote usage of the `podman manifest annotate` command is now supported.
- The `SSL_CERT_FILE` and `SSL_CERT_DIR` environment variables are now propagated into Podman machine VMs ([#16041](https://github.com/containers/podman/issues/16041)).
- A new environment variable, `CONTAINER_PROXY`, can be used to specify TCP proxies when using remote Podman.
- The runtime automatically detects and switches to crun-wasm if the image is a webassembly image.
- The `podman machine init` command now supports the `--quiet` option, as well a new option, `--no-info` which suppresses informational tips ([#15525](https://github.com/containers/podman/issues/15525)).
- The `podman volume create` command now includes the `-d` short option for the `--driver` option.
- The `podman events` command has a new alias, `podman system events`, for better Docker compatibility.
- The `--restart-sec` option for `podman generate systemd` now generates `RestartSec=` for both pod service files and container service files ([#16419](https://github.com/containers/podman/issues/16419)).
- The `podman manifest push` command now accepts `--purge`, `-p` options as aliases for `--rm`, for Docker compatibility.
- The `--network` option to `podman pod create` now supports using an existing network namespace via `ns:[netns-path]` ([#16208](https://github.com/containers/podman/issues/16208)).
- The `podman pod rm` and `podman container rm` commands now removes container/pod ID files along with the container/pod ([#16387](https://github.com/containers/podman/issues/16387)).
- The `podman manifest inspect` command now accepts a new option, `--insecure` as an alias to`--tls-verify=false`, improving Docker compatibility ([#14917](https://github.com/containers/podman/issues/14917)).
- A new command, `podman kube apply`, has been added, which deploys the generated yaml to a k8s cluster.
- The `--userns=keep-id` option in rootless `podman create`, `podman run`, `podman kube play`, `podman pod create`, and `podman pod clone` now can be used when only one ID is available.
- The `podman play kube` command now supports the `volume.podman.io/import-source` annotation to import the contents of tarballs.
- The `podman volume create` command now accepts the `--ignore` option, which ignores the create request if the named volume already exists.
- The `--filter` option for `podman ps` now supports regex  ([#16180](https://github.com/containers/podman/issues/16180)).
- The `podman system df` command now accepts `--format json` and autocompletes for the `--format` option ([#16204](https://github.com/containers/podman/issues/16204)).
- The `podman kube down` command accepts a new option, `--force`, which removes volumes ([#16348](https://github.com/containers/podman/issues/16348)).
- The `podman create`, `podman run`, and `podman pod create` commands now support a new networking mode, pasta, which can be enabled with the `--net=pasta` option ([#14425](https://github.com/containers/podman/issues/14425), [#13229](https://github.com/containers/podman/issues/13229)).

### Changes
- CNI is being deprecated from Podman and support will be dropped at a future date. Netavark is now advised and is the default network backend for Podman.
- The network name `pasta` is deprecated and support for it will be removed in the next major release.
- The `podman network create` command no longer accepts `default` as valid name. It is impossible to use this network name in the `podman run/create` command because it is parsed as a network mode instead ([#17169](https://github.com/containers/podman/issues/17169)).
- The `podman kube generate` command will no longer generate built-in annotations, as reserved annotations are used internally by Podman and would have no effect when run with Kubernetes.
- The `podman kube play` command now limits the replica count to 1 when deploying from kubernetes YAML ([#16765](https://github.com/containers/podman/issues/16765)).
- When a container that runs with the `--pid=host` option is terminated, Podman now sends a SIGKILL to all the active exec sessions
- The journald driver for both `podman events` and `podman logs` is now more efficient when the `--since` option is used, as it will now seek directly to the correct time instead of reading all entries from the journal ([#16950](https://github.com/containers/podman/issues/16950)).
- When the `--service-container` option is set for the `podman kube play` command, the default log-driver to is now set to `passthrough` ([#16592](https://github.com/containers/podman/issues/16592)).
- The `podman container inspect` and `podman kube generate` commands will no longer list default annotations set to false.
- Podman no longer reports errors on short-lived init containers in pods.
- Healthchecks are now automatically disabled if on non-systemd systems. If Podman is compiled without the systemd build tag, healthcheck will be disabled at build time ([#16644](https://github.com/containers/podman/issues/16644)).
- Improved atomicity of VM state persistence on Windows now better tolerates FS corruption in cases of power loss or system failure ([#16550](https://github.com/containers/podman/issues/16550)).
- A user namespace is now always created when running with EUID != 0.  This is necessary to work in a Kubernetes environment where the POD is "privileged" but it is still running with a non-root user.
- Old healthcheck states are now cleaned up during container restart.
- The  `CONTAINER_HOST` environment variable defaults to port 22 for SSH style URLs for remote connections, when set ([#16509](https://github.com/containers/podman/issues/16509)).
- The `podman kube play` command now reuses existing PersistentVolumeClaims instead of erroring.
- The `podman system reset` command will no longer prompt the user if `/usr/share/containers/storage.conf` file exists.
- Existing container/pod id files are now truncated instead of throwing an error.
- The `--format` and `--verbose` flags in `podman system df` are no longer allowed to be used in combination.
- The `podman kube generate` command now sets `runAsNonRoot=true` in the generated yaml when the image has user set as a positive integer ([#15231](https://github.com/containers/podman/issues/15231)).
- Listing containers (e.g, via `podman ps`) is considerably faster.
- The `podman push` and `podman manifest push` commands now support a new option, `--sign-by-sigstore`, which allows using Fulcio and Rekor.

### Bugfixes
- Fixed a bug where the `--dns` option was not being set correctly ([#16172](https://github.com/containers/podman/issues/16172)).
- Fixed a race condition that caused `podman rm` to fail when stopping or killing a container that has already been stopped or has exited ([#16142](https://github.com/containers/podman/issues/16142), [#15367](https://github.com/containers/podman/issues/15367)).
- Fixed a bug where `podman kube play` default environment variables have not been applied to containers ([#17016](https://github.com/containers/podman/issues/17016)).
- Fixed a bug where containers with a restart policy set could still restart even after a manual `podman stop` ([#17069](https://github.com/containers/podman/issues/17069)).
- Fixed a bug where the runtime was not shutdown correctly on error.
- Fixed a bug where a pod couldn't be removed if its service container did not exist anymore ([#16964](https://github.com/containers/podman/issues/16964)).
- Fixed a bug where the output of a non-interactive `docker run` against a podman backend would be truncated when using Docker Clients on Mac and Windows ([#16656](https://github.com/containers/podman/issues/16656)).
- Fixed a bug where `podman logs --since --follow` would not follow and just exit with the journald driver.
- Fixed a bug where `podman logs --until --follow` would not exit after the given until time.
- Fixed a bug where remote usage of the `podman attach` and `podman start` did not sigproxy ([#16662](https://github.com/containers/podman/issues/16662)).
- Fixed a race condition where a container being stopped could be removed from a separate process.
- Fixed a bug in the `podman ps` command’s `--filter` option where specifying volume as a filter would not return the correct containers ([#16019](https://github.com/containers/podman/issues/16019)).
- Fixed a bug where podman-remote would send an incorrect absolute path as context when it’s an emptydir.
- Fixed a bug with the `podman export` command on MacOS and Windows where it could not export to STDOUT ([#16870](https://github.com/containers/podman/issues/16870)).
- Fixed a bug in the http attach endpoint where it would return an incorrect length when reading logs ([#16856](https://github.com/containers/podman/issues/16856)).
- Fixed a bug where symlinks were not followed on mounted folders on MacOS.
- Fixed a bug in the `podman container restore` command’s ` --ignore-static-ip` and `--ignore-static-mac` options when restoring a normal container, i.e without `--import`, where the option was not correctly honored ([#16666](https://github.com/containers/podman/issues/16666)).
- Fixed a bug where containers, pods, and volumes were not cleaned up after an error happens while playing a kube yaml file.
- Fixed a bug where system shutdown would be delayed when running health checks on containers running in a systemd unit ([#14531](https://github.com/containers/podman/issues/14531)).
- Fixed a bug where syslog entries may be truncated when the labels map is too large, by increasing event syslog deserialization buffer.
- Fixed a bug in `podman kube play` where secrets were incorrectly unmarshalled ([#16269](https://github.com/containers/podman/issues/16269), [#16625](https://github.com/containers/podman/issues/16625)).
- Fixed a bug where barrier sd-notify messages were ignored when using notify policies in kube-play ([#16076](https://github.com/containers/podman/issues/16076), [#16515](https://github.com/containers/podman/issues/16644)).
- Fixed a bug where volumes that use idmap were chowned incorrectly to the UID/GID of the root in the container.
- Fixed a bug in `podman kube play` where IpcNS was not being properly set
([#16632](https://github.com/containers/podman/issues/16632)).
- Fixed a bug in `podman kube play` that occurred when the `optional` field of a secret volume was not set in the kube yaml, causing Podman to crash ([#16636](https://github.com/containers/podman/issues/16636)).
- Fixed a bug in the `podman stats` command where the NetInput and NetOutput fields were swapped.
- Fixed a bug in the `podman network create` command’s `--driver` option where incorrect shell completion suggestions were given.
- Fixed a bug where `podman --noout` was not suppressing output from certain commands such as `podman machine` and `podman system connection` ([#16201](https://github.com/containers/podman/issues/16201)).
- Fixed a bug where a pod was partially created even when its creation has failed ([#16502](https://github.com/containers/podman/issues/16502)).
- Fixed a bug in `podman cp` when copying directories ending with a "." ([#16421](https://github.com/containers/podman/issues/16421)).
- Fixed a bug where the root `--connection` option would not work with a cached config ([#16282](https://github.com/containers/podman/issues/16282)).
- Fixed a bug with the `--format {{ json .}}` option which resulted in different output compared to docker ([#16436](https://github.com/containers/podman/issues/16436)).
- Fixed short name resolution on Windows to `docker.io` to avoid TTY check failure ([#16417](https://github.com/containers/podman/issues/16417)).
- Fixed a bug with the systemd booted check when `/proc` is mounted with the `hidepid=2` option ([#16022](https://github.com/containers/podman/issues/16022)).
- Fixed a bug where named volumes were not properly idmapped.
- Fixed a bug in `podman kube play` where the sdnotify proxy could cause Podman to deadlock ([#16076](https://github.com/containers/podman/issues/16076)).
- Fixed a bug where the containers.conf files are reloaded redundantly.
- Fixed a bug where `podman system df` reported wrong image sizes ([#16135](https://github.com/containers/podman/issues/16135)).
- Fixed a bug where `podman inspect` did not correctly remote the IPCMode of containers ([#17189](https://github.com/containers/podman/issues/17189)).
- Fixed a bug where containers created in a pod using the `--userns keep-id` option were not correctly adding username entries to /etc/passwd within container ([#17148](https://github.com/containers/podman/issues/17148)).
- Fixed a bug where the `--publish-all` flag in the `podman create` and `podman run` commands would occasionally assign colliding ports.
- Fixed a bug where `podman machine init --image-path` on Windows was not correctly handling absolute paths ([#15995](https://github.com/containers/podman/issues/15995)).
- Fixed a bug where the `podman machine init` would fail on non-systemd Linux distributions due to the lack of timedatectl ([#17244](https://github.com/containers/podman/issues/17244)).
- Fixed a bug where `podman machine` commands would fail on Windows when the Podman managed VM is set as default in WSL, under some locales ([#17227](https://github.com/containers/podman/issues/17227), [#17158](https://github.com/containers/podman/issues/17158)).
- Fixed a bug where the `podman ps` command’s STATUS output’s human readable output would add “ago” ([#17250](https://github.com/containers/podman/issues/17250)).
- Fixed a bug where the `podman events` command run with the journald driver could show events from other users.

### API
- When creating a container with the Compat API, the `NetworkMode=default` is no longer rewritten to `NetworkMode=bridge` if the `containers.conf` configuration file overwrites `netns` ([#16915](https://github.com/containers/podman/issues/16915)).
- The Compat Create endpoint now supports the MAC address field in the container config. This ensures that the static mac from the docker-compose.yml is used ([#16411](https://github.com/containers/podman/issues/16411)).
- Fixed a bug in the Compat Build endpoint where the chunked response may have included more JSON objects than expected per chunk ([#16360](https://github.com/containers/podman/issues/16360)).
- Fixed a bug in the Compat Create endpoint where DeviceCgroupRules was not correctly set ([#17106](https://github.com/containers/podman/issues/17106)).

### Misc
- Fixed WSL auto-installation when run under Windows ARM x86_64 emulation
- Add initial support for Windows on ARM64.
- Added a systemd unit file that is useful for transient storage mode cleanup.
- The `podman-release-static.tar.gz` artfact has been renamed to `podman-release-static-linux_{amd64,arm64}.tar.gz` ([#16612](https://github.com/containers/podman/issues/16612)).
- The `podman-installer-macos-aarch64.pkg` artifact has been renamed to `podman-installer-macos-arm64.pkg`.
- The MacOS pkginstaller now installs podman-mac-helper by default ([#16547](https://github.com/containers/podman/issues/16547)).
- Manual overrides of the install location in Windows installer are now allowed.
([#16265](https://github.com/containers/podman/issues/16265)).
- Continued ongoing work on porting Podman to FreeBSD
- Updated the Mac pkginstaller qemu to v7.1.0
- Updated the Golang version to 1.18
- Updated the containers/image library to v5.24.0
- Updated the containers/storage library to v1.45.3
- Updated the containers/common library to v0.51.0
- Updated Buildah to v1.29.0

## 4.3.1
### Bugfixes
- Fixed a deadlock between the `podman ps` and `podman container inspect` commands

### Misc
- Updated the containers/image library to v5.23.1

## 4.3.0
### Features
- A new command, `podman generate spec`, has been added, which creates a JSON struct based on a given container that can be used with the Podman REST API to create containers.
- A new command, `podman update`, has been added,which makes changes to the resource limits of existing containers. Please note that these changes do not persist if the container is restarted ([#15067](https://github.com/containers/podman/issues/15067)).
- A new command, `podman kube down`, has been added, which removes pods and containers created by the given Kubernetes YAML (functionality is identical to `podman kube play --down`, but it now has its own command).
- The `podman kube play` command now supports Kubernetes secrets using Podman's secrets backend.
- Systemd-managed pods created by the `podman kube play` command now integrate with sd-notify, using the `io.containers.sdnotify` annotation (or `io.containers.sdnotify/$name` for specific containers).
- Systemd-managed pods created by `podman kube play` can now be auto-updated, using the `io.containers.auto-update` annotation (or `io.containers.auto-update/$name` for specific containers).
- The `podman kube play` command can now read YAML from URLs, e.g. `podman kube play https://example.com/demo.yml` ([#14955](https://github.com/containers/podman/issues/14955)).
- The `podman kube play` command now supports the `emptyDir` volume type ([#13309](https://github.com/containers/podman/issues/13309)).
- The `podman kube play` command now supports the `HostUsers` field in the pod spec.
- The `podman play kube` command now supports `binaryData` in ConfigMaps.
- The `podman pod create` command can now set additional resource limits for pods using the new `--memory-swap`, `--cpuset-mems`, `--device-read-bps`, `--device-write-bps`, `--blkio-weight`, `--blkio-weight-device`, and `--cpu-shares` options.
- The `podman machine init` command now supports a new option, `--username`, to set the username that will be used to connect to the VM as a non-root user ([#15402](https://github.com/containers/podman/issues/15402)).
- The `podman volume create` command's `-o timeout=` option can now set a timeout of 0, indicating volume plugin operations will never time out.
- Added support for a new volume driver, `image`, which allows volumes to be created that are backed by images.
- The `podman run` and `podman create` commands support a new option, `--env-merge`, allowing environment variables to be specified relative to other environment variables in the image (e.g. `podman run --env-merge "PATH=$PATH:/my/app" ...`) ([#15288](https://github.com/containers/podman/issues/15288)).
- The `podman run` and `podman create` commands support a new option, `--on-failure`, to allow action to be taken when a container fails health checks, with the following supported actions: `none` (take no action, the default), `kill` (kill the container), `restart` (restart the container), and `stop` (stop the container).
- The `--keep-id` option to `podman create` and `podman run` now supports new options, `uid` and `gid`, to set the UID and GID of the user in the container that will be mapped to the user running Podman (e.g. `--userns=keep-id:uid=11` will made the user running Podman to UID 11 in the container) ([#15294](https://github.com/containers/podman/issues/15294)).
- The `podman generate systemd` command now supports a new option, `--env`/`-e`, to set environment variables in the generated unit file ([#15523](https://github.com/containers/podman/issues/15523)).
- The `podman pause` and `podman unpause` commands now support the `--latest`, `--cidfile`, and `--filter` options.
- The `podman restart` command now supports the `--cidfile` and `--filter` options.
- The `podman rm` command now supports the `--filter` option to select which containers will be removed.
- The `podman rmi` command now supports a new option, `--no-prune`, to prevent the removal of dangling parents of removed images.
- The `--dns-opt` option to `podman create`, `podman run`, and `podman pod create` has received a new alias, `--dns-option`, to improve Docker compatibility.
- The `podman` command now features a new global flag, `--debug`/`-D`, which enables debug-level logging (identical to `--log-level=debug`), improving Docker compatibility.
- The `podman` command now features a new global flag, `--config`. This flag is ignored, and is only included for Docker compatibility ([#14767](https://github.com/containers/podman/issues/14767)).
- The `podman manifest create` command now accepts a new option, `--amend`/`-a`.
- The `podman manifest create`, `podman manifest add` and `podman manifest push` commands now accept a new option, `--insecure` (identical to `--tls-verify=false`), improving Docker compatibility.
- The `podman secret create` command's `--driver` and `--format` options now have new aliases, `-d` for `--driver` and `-f` for `--format`.
- The `podman secret create` command now supports a new option, `--label`/`-l`, to add labels to created secrets.
- The `podman secret ls` command now accepts the `--quiet`/`-q` option.
- The `podman secret inspect` command now accepts a new option, `--pretty`, to print output in human-readable format.
- The `podman stats` command now accepts the `--no-trunc` option.
- The `podman save` command now accepts the `--signature-policy` option ([#15869](https://github.com/containers/podman/issues/15869)).
- The `podman pod inspect` command now allows multiple arguments to be passed. If so, it will return a JSON array of the inspected pods ([#15674](https://github.com/containers/podman/issues/15674)).
- A series of new hidden commands have been added under `podman context` as aliases to existing `podman system connection` commands, to improve Docker compatibility.
- The remote Podman client now supports proxying signals for attach sessions when the `--sig-proxy` option is set ([#14707](https://github.com/containers/podman/issues/14707)).

### Changes
- Duplicate volume mounts are now allowed with the `-v` option to `podman run`, `podman create`, and `podman pod create`, so long as source, destination, and options all match ([#4217](https://github.com/containers/podman/issues/4217)).
- The `podman generate kube` and `podman play kube` commands have been renamed to `podman kube generate` and `podman kube play` to group Kubernetes-related commands. Aliases have been added to ensure the old command names still function.
- A number of Podman commands (`podman init`, `podman container checkpoint`, `podman container restore`, `podman container cleanup`) now print the user-inputted name of the container, instead of its full ID, on success.
- When an unsupported option (e.g. resource limit) is specified for a rootless container on a cgroups v1 system, a warning message is now printed that the limit will not be honored.
- The installer for the Windows Podman client has been improved.
- The `--cpu-rt-period` and `--cpu-rt-runtime` options to `podman run` and `podman create` now print a warning and are ignored on cgroups v2 systems (cgroups v2 having dropped support for these controllers) ([#15666](https://github.com/containers/podman/issues/15666)).
- Privileged containers running systemd will no longer mount `/dev/tty*` devices other than `/dev/tty` itself into the container ([#15878](https://github.com/containers/podman/issues/15878)).
- Events for containers that are part of a pod now include the ID of the pod in the event.
- SSH functionality for `podman machine` commands has seen a thorough rework, addressing many issues about authentication.
- The `--network` option to `podman kube play` now allows passing `host` to set the pod to use host networking, even if the YAML does not request this.
- The `podman inspect` command on containers now includes the digest of the image used to create the container.
- Pods created by `podman play kube` are now, by default, placed into a network named `podman-kube`. If the `podman-kube` network does not exist, it will be created. This ensures pods can connect to each other by their names, as the network has DNS enabled.

### Bugfixes
- Fixed a bug where the `podman network prune` and `podman container prune` commands did not properly support the `--filter label!=` option ([#14182](https://github.com/containers/podman/issues/14182)).
- Fixed a bug where the `podman kube generate` command added an unnecessary `Secret: null` line to generated YAML ([#15156](https://github.com/containers/podman/issues/15156)).
- Fixed a bug where the `podman kube generate` command did not set `enableServiceLinks` and `automountServiceAccountToken` to false in generated YAML ([#15478](https://github.com/containers/podman/issues/15478) and [#15243](https://github.com/containers/podman/issues/15243)).
- Fixed a bug where the `podman kube play` command did not properly handle CPU limits ([#15726](https://github.com/containers/podman/issues/15726)).
- Fixed a bug where the `podman kube play` command did not respect default values for liveness probes ([#15855](https://github.com/containers/podman/issues/15855)).
- Fixed a bug where the `podman kube play` command did not bind ports if `hostPort` was not specified but `containerPort` was ([#15942](https://github.com/containers/podman/issues/15942)).
- Fixed a bug where the `podman kube play` command sometimes did not create directories on the host for `hostPath` volumes.
- Fixed a bug where the remote Podman client's `podman manifest push` command did not display progress.
- Fixed a bug where the `--filter "{{.Config.Healthcheck}}"` option to `podman image inspect` did not print the image's configured healthcheck ([#14661](https://github.com/containers/podman/issues/14661)).
- Fixed a bug where the `podman volume create -o timeout=` option could be specified even when no volume plugin was in use.
- Fixed a bug where the `podman rmi` command did not emit `untag` events when removing tagged images ([#15485](https://github.com/containers/podman/issues/15485)).
- Fixed a bug where API forwarding with `podman machine` VMs on Windows could sometimes fail because the pipe was not created in time ([#14811](https://github.com/containers/podman/issues/14811)).
- Fixed a bug where the `podman pod rm` command could error if removal of a container in the pod was interrupted by a reboot.
- Fixed a bug where the `exited` and `exec died` events for containers did not include the container's labels ([#15617](https://github.com/containers/podman/issues/15617)).
- Fixed a bug where running Systemd containers on a system not using Systemd as PID 1 could fail ([#15647](https://github.com/containers/podman/issues/15647)).
- Fixed a bug where Podman did not pass all necessary environment variables (including `$PATH`) to Conmon when starting containers ([#15707](https://github.com/containers/podman/issues/15707)).
- Fixed a bug where the `podman events` command could function improperly when no events were present ([#15688](https://github.com/containers/podman/issues/15688)).
- Fixed a bug where the `--format` flag to various Podman commands did not properly handle template strings including a newline (`\n`) ([#13446](https://github.com/containers/podman/issues/13446)).
- Fixed a bug where Systemd-managed pods would kill every container in a pod when a single container exited ([#14546](https://github.com/containers/podman/issues/14546)).
- Fixed a bug where the `podman generate systemd` command would generate incorrect YAML for pods created without the `--name` option.
- Fixed a bug where the `podman generate systemd --new` command did not properly set stop timeout ([#16149](https://github.com/containers/podman/issues/16149)).
- Fixed a bug where a broken OCI spec resulting from the system rebooting while a container is being started could cause the `podman inspect` command to be unable to inspect the container until it was restarted.
- Fixed a bug where creating a container with a working directory on an overlay volume would result in the container being unable to start ([#15789](https://github.com/containers/podman/issues/15789)).
- Fixed a bug where attempting to remove a pod with running containers without `--force` would not error and instead would result in the pod, and its remaining containers, being placed in an unusable state ([#15526](https://github.com/containers/podman/issues/15526)).
- Fixed a bug where memory limits reported by `podman stats` could exceed the maximum memory available on the system ([#15765](https://github.com/containers/podman/issues/15765)).
- Fixed a bug where the `podman container clone` command did not properly handle environment variables whose value contained an `=` character ([#15836](https://github.com/containers/podman/issues/15836)).
- Fixed a bug where the remote Podman client would not print the container ID when running the `podman-remote run --attach stdin` command.
- Fixed a bug where the `podman machine list --format json` command did not properly show machine starting status.
- Fixed a bug where automatic updates would not error when attempting to update a container with a non-fully qualified image name ([#15879](https://github.com/containers/podman/issues/15879)).
- Fixed a bug where the `podman pod logs --latest` command could panic ([#15556](https://github.com/containers/podman/issues/15556)).
- Fixed a bug where Podman could leave lingering network namespace mounts on the system if cleaning up the network failed.
- Fixed a bug where specifying an unsupported URI scheme for `podman system service` to listen at would result in a panic.
- Fixed a bug where the `podman kill` command would sometimes not transition containers to the exited state ([#16142](https://github.com/containers/podman/issues/16142)).

### API
- Fixed a bug where the Compat DF endpoint reported incorrect reference counts for volumes ([#15720](https://github.com/containers/podman/issues/15720)).
- Fixed a bug in the Compat Inspect endpoint for Networks where an incorrect network option was displayed, causing issues with `docker-compose` ([#15580](https://github.com/containers/podman/issues/15580)).
- The Libpod Restore endpoint for Containers now features a new query parameter, `pod`, to set the pod that the container will be restored into ([#15018](https://github.com/containers/podman/issues/15018)).
- Fixed a bug where the REST API could panic while retrieving images.
- Fixed a bug where a cancelled connection to several endpoints could induce a memory leak.

### Misc
- Error messages when attempting to remove an image used by a non-Podman container have been improved ([#15006](https://github.com/containers/podman/issues/15006)).
- Podman will no longer print a warning that `/` is not a shared mount when run inside a container ([#15295](https://github.com/containers/podman/issues/15295)).
- Work is ongoing to port Podman to FreeBSD.
- The output of `podman generate systemd` has been adjusted to improve readability.
- A number of performance improvements have been made to `podman create` and `podman run`.
- A major reworking of the manpages to ensure duplicated options between commands have the same description text has been performed.
- Updated Buildah to v1.28.0
- Updated the containers/image library to v5.23.0
- Updated the containers/storage library to v1.43.0
- Updated the containers/common library to v0.50.1

## 4.2.1
### Features
- Added support for Sigstore signatures (`sigstoreSigned`) to the `podman image trust set` and `podman image trust show` commands.`
- The `podman image trust show` command now recognizes new `lookaside` field names.
- The `podman image trust show` command now recognizes `keyPaths` in `signedBy` entries.

### Changes
- BREAKING CHANGE: `podman image trust show` may now show multiple entries for the same scope, to better represent separate requirements. GPG IDs on a single row now always represent alternative keys, only one of which is required; if multiple sets of keys are required, each is represented by a single line.
- The `podman generate kube` command no longer adds the `bind-mount-options` annotation to generated Service YAML ([#15208](https://github.com/containers/podman/issues/15208)).

### Bugfixes
- Fixed a bug where Podman could deadlock when using `podman kill` to send signals to containers ([#15492](https://github.com/containers/podman/issues/15492)).
- Fixed a bug where the `podman image trust set` command would silently discard unknown fields.
- Fixed a bug where the `podman image trust show` command would not show signature enforcement configuration for the default scope.
- Fixed a bug where the `podman image trust show` command would silently ignore multiple kinds of requirements in a single scope.
- Fixed a bug where a typo in the `podman-kube@.service` unit file would cause warnings when running `systemctl status` on the unit.
- Fixed a bug where the `--compress` option to `podman image save` was incorrectly allowed with the `oci-dir` format.
- Fixed a bug where the `podman container clone` command did not properly clone environment variables ([#15242](https://github.com/containers/podman/issues/15242)).
- Fixed a bug where Podman would not accept environment variables with whitespace in their keys ([#15251](https://github.com/containers/podman/issues/15251)).
- Fixed a bug where Podman would not accept file paths containing the `:` character, preventing some commands from being used with `podman machine` on Windows ([#15247](https://github.com/containers/podman/issues/15247)).
- Fixed a bug where the `podman top` command would report new capabilities as unknown.
- Fixed a bug where running Podman in a container could cause fatal errors about an inability to create cgroups ([#15498](https://github.com/containers/podman/issues/15498)).
- Fixed a bug where the `podman generate kube` command could generate incorrect YAML when the `bind-mount-options` was used ([#15170](https://github.com/containers/podman/issues/15170)).
- Fixed a bug where generated container names were deterministic, instead of random ([#15569](https://github.com/containers/podman/issues/15569)).
- Fixed a bug where the `podman events` command would not work with custom `--format` specifiers ([#15648](https://github.com/containers/podman/issues/15648)).

### API
- Fixed a bug where the Compat List endpoint for Containers did not sort the `HostConfig.Binds` field as Docker does.
- Fixed a bug where the Compat List endpoint for Containers send the name (instead of ID) of the image the container was based on.
- Fixed a bug where the Compat Connect endpoint for Networks would return an error (instead of 200) when attempting to connect a container to a network it was already connected to ([#15499](https://github.com/containers/podman/issues/15499)).
- Fixed a bug where the Compat Events endpoint set an incorrect status for image removal events (`remove` instead of `delete`) ([#15485](https://github.com/containers/podman/issues/15485)).

## 4.2.0
### Features
- Podman now supports the GitLab Runner (using the Docker executor), allowing its use in GitLab CI/CD pipelines.
- A new command has been added, `podman pod clone`, to create a copy of an existing pod. It supports several options, including `--start` to start the new pod, `--destroy` to remove the original pod, and `--name` to change the name of the new pod ([#12843](https://github.com/containers/podman/issues/12843)).
- A new command has been added, `podman volume reload`, to sync changes in state between Podman's database and any configured volume plugins ([#14207](https://github.com/containers/podman/issues/14207)).
- A new command has been added, `podman machine info`, which displays information about the host and the versions of various machine components.
- Pods created by `podman play kube` can now be managed by systemd unit files. This can be done via a new systemd service, `podman-kube@.service` - e.g. `systemctl --user start podman-play-kube@$(systemd-escape my.yaml).service` will run the Kubernetes pod or deployment contained in `my.yaml` under systemd.
- The `podman play kube` command now honors the `RunAsUser`, `RunAsGroup`, and `SupplementalGroups` setting from the Kubernetes pod's security context.
- The `podman play kube` command now supports volumes with the `BlockDevice` and `CharDevice` types ([#13951](https://github.com/containers/podman/issues/13951)).
- The `podman play kube` command now features a new flag, `--userns`, to set the user namespace of created pods. Two values are allowed at present: `host` and `auto` ([#7504](https://github.com/containers/podman/issues/7504)).
- The `podman play kube` command now supports setting the type of created init containers via the `io.podman.annotations.init.container.type` annotation.
- The `podman pod create` command now supports an exit policy (configurable via the `--exit-policy` option), which determines what will happen to the pod's infra container when the entire pod stops. The default, `continue`, acts as Podman currently does, while a new option, `stop`, stops the infra container after the last container in the pod stops.  The latter is used for pods created via `podman play kube` ([#13464](https://github.com/containers/podman/issues/13464)).
- The `podman pod create` command now allows the pod's name to be specified as an argument, instead of using the `--name` option - for example, `podman pod create mypod` instead of the prior `podman pod create --name mypod`. Please note that the `--name` option is not deprecated and will continue to work.
- The `podman pod create` command's `--share` option now supports adding namespaces to the set by prefacing them with `+` (as opposed to specifying all namespaces that should be shared) ([#13422](https://github.com/containers/podman/issues/13422)).
- The `podman pod create` command has a new option, `--shm-size`, to specify the size of the `/dev/shm` mount that will be shared if the pod shares its UTS namespace ([#14609](https://github.com/containers/podman/issues/14609)).
- The `podman pod create` command has a new option, `--uts`, to configure the UTS namespace that will be shared by containers in the pod.
- The `podman pod create` command now supports setting pod-level resource limits via the `--cpus`, `--cpuset-cpus`, and `--memory` options. These will set a limit for all containers in the pod, while individual containers within the pod are allowed to set further limits. Look forward to more options for resource limits in our next release!
- The `podman create` and `podman run` commands now include the `-c` short option for the `--cpu-shares` option.
- The `podman create` and `podman run` commands can now create containers from a manifest list (and not an image) as long as the `--platform` option is specified ([#14773](https://github.com/containers/podman/issues/14773)).
- The `podman build` command now supports a new option, `--cpp-flag`, to specify options for the C preprocessor when using `Containerfile.in` files that require preprocessing.
- The `podman build` command now supports a new option, `--build-context`, allowing the user to specify an additional build context.
- The `podman machine inspect` command now prints the location of the VM's Podman API socket on the host ([#14231](https://github.com/containers/podman/issues/14231)).
- The `podman machine init` command on Windows now fetches an image with packages pre-installed ([#14698](https://github.com/containers/podman/issues/14698)).
- Unused, cached Podman machine VM images are now cleaned up automatically. Note that because Podman now caches in a different directory, this will not clean up old images pulled before this change ([#14697](https://github.com/containers/podman/issues/14697)).
- The default for the `--image-volume` option to `podman run` and `podman create` can now have its default set through the `image_volume_mode` setting in `containers.conf` ([#14230](https://github.com/containers/podman/issues/14230)).
- Overlay volumes now support two new options, `workdir` and `upperdir`, to allow multiple overlay volumes from different containers to reuse the same `workdir` or `upperdir` ([#14427](https://github.com/containers/podman/issues/14427)).
- The `podman volume create` command now supports two new options, `copy` and `nocopy`, to control whether contents from the overmounted folder in a container will be copied into the newly-created named volume (copy-up).
- Volumes created using a volume plugin can now specify a timeout for all operations that contact the volume plugin (replacing the standard 5 second timeout) via the `--opt o=timeout=` option to `podman volume create` ([BZ 2080458](https://bugzilla.redhat.com/show_bug.cgi?id=2080458)).
- The `podman volume ls` command's `--filter name=` option now supports regular expression matching for volume names ([#14583](https://github.com/containers/podman/issues/14583)).
- When used with a `podman machine` VM, volumes now support specification of the 9p security model using the `security_model` option to `podman create -v` and `podman run -v`.
- The remote Podman client's `podman push` command now supports the `--remove-signatures` option ([#14558](https://github.com/containers/podman/issues/14558)).
- The remote Podman client now supports the `podman image scp` command.
- The `podman image scp` command now supports tagging the transferred image with a new name.
- The `podman network ls` command supports a new filter, `--filter dangling=`, to list networks not presently used by any containers ([#14595](https://github.com/containers/podman/issues/14595)).
- The `--condition` option to `podman wait` can now be specified multiple times to wait on any one of multiple conditions.
- The `podman events` command now includes the `-f` short option for the `--filter` option.
- The `podman pull` command now includes the `-a` short option for the `--all-tags` option.
- The `podman stop` command now includes a new flag, `--filter`, to filter which containers will be stopped (e.g. `podman stop --all --filter label=COM.MY.APP`).
- The Podman global option `--url` now has two aliases: `-H` and `--host`.
- The `podman network create` command now supports a new option with the default `bridge` driver, `--opt isolate=`, which isolates the network by blocking any traffic from it to any other network with the `isolate` option enabled. This option is enabled by default for networks created using the Docker-compatible API.
- Added the ability to create sigstore signatures in `podman push` and `podman manifest push`.
- Added an option to read image signing passphrase from a file.

### Changes
- Paused containers can now be killed with the `podman kill` command.
- The `podman system prune` command now removes unused networks.
- The `--userns=keep-id` and `--userns=nomap` options to the `podman run` and `podman create` commands are no longer allowed (instead of simply being ignored) with root Podman.
- If the `/run` directory for a container is part of a volume, Podman will not create the `/run/.containerenv` file ([#14577](https://github.com/containers/podman/issues/14577)).
- The `podman machine stop` command on macOS now waits for the machine to be completely stopped to exit ([#14148](https://github.com/containers/podman/issues/14148)).
- All `podman machine` commands now only support being run as rootless, given that VMs only functioned when run rootless.
- The `podman unpause --all` command will now only attempt to unpause containers that are paused, not all containers.
- Init containers created with `podman play kube` now default to the `once` type ([#14877](https://github.com/containers/podman/issues/14877)).
- Pods created with no shared namespaces will no longer create an infra container unless one is explicitly requested ([#15048](https://github.com/containers/podman/issues/15048)).
- The `podman create`, `podman run`, and `podman cp` commands can now autocomplete paths in the image or container via the shell completion.
- The `libpod/common` package has been removed as it's not used anywhere.
- The `--userns` option to `podman create` and `podman run` is no longer accepted when an explicit UID or GID mapping is specified ([#15233](https://github.com/containers/podman/issues/15233)).

### Bugfixes
- Fixed a bug where bind-mounting `/dev` into a container which used the `--init` flag would cause the container to fail to start ([#14251](https://github.com/containers/podman/issues/14251)).
- Fixed a bug where the `podman image mount` command would not pretty-print its output when multiple images were mounted.
- Fixed a bug where the `podman volume import` command would print an unrelated error when attempting to import into a nonexistent volume ([#14411](https://github.com/containers/podman/issues/14411)).
- Fixed a bug where the `podman system reset` command could race against other Podman commands ([#9075](https://github.com/containers/podman/issues/9075)).
- Fixed a bug where privileged containers were not able to restart if the layout of host devices changed ([#13899](https://github.com/containers/podman/issues/13899)).
- Fixed a bug where the `podman cp` command would overwrite directories with non-directories and vice versa.  A new `--overwrite` flag to `podman cp` allows for retaining the old behavior if needed ([#14420](https://github.com/containers/podman/issues/14420)).
- Fixed a bug where the `podman machine ssh` command would not preserve the exit code from the command run via ssh ([#14401](https://github.com/containers/podman/issues/14401)).
- Fixed a bug where VMs created by `podman machine` would fail to start when created with more than 3072MB of RAM on Macs with M1 CPUs ([#14303](https://github.com/containers/podman/issues/14303)).
- Fixed a bug where the `podman machine init` command would fail when run from `C:\Windows\System32` on Windows systems ([#14416](https://github.com/containers/podman/issues/14416)).
- Fixed a bug where the `podman machine init --now` did not respect proxy environment variables ([#14640](https://github.com/containers/podman/issues/14640)).
- Fixed a bug where the `podman machine init` command would fail if there is no `$HOME/.ssh` dir ([#14572](https://github.com/containers/podman/issues/14572)).
- Fixed a bug where the `podman machine init` command would add a connection even if creating the VM failed ([#15154](https://github.com/containers/podman/issues/15154)).
- Fixed a bug where interrupting the `podman machine start` command could render the VM unable to start.
- Fixed a bug where the `podman machine list --format` command would still print a heading.
- Fixed a bug where the `podman machine list` command did not properly set the `Starting` field ([#14738](https://github.com/containers/podman/issues/14738)).
- Fixed a bug where the `podman machine start` command could fail to start QEMU VMs when the machine name started with a number.
- Fixed a bug where Podman Machine VMs with proxy variables could not be started more than once ([#14636](https://github.com/containers/podman/issues/14636) and [#14837](https://github.com/containers/podman/issues/14837)).
- Fixed a bug where containers created using the Podman API would, when the Podman API service was managed by systemd, be killed when the API service was stopped ([BZ 2052697](https://bugzilla.redhat.com/show_bug.cgi?id=2052697)).
- Fixed a bug where the `podman -h` command did not show help output.
- Fixed a bug where the `podman wait` command (and the associated REST API endpoint) could return before a container had fully exited, breaking some tools like the GitLab Runner.
- Fixed a bug where healthchecks generated `exec` events, instead of `health_status` events ([#13493](https://github.com/containers/podman/issues/13493)).
- Fixed a bug where the `podman pod ps` command could return an error when run at the same time as `podman pod rm` ([#14736](https://github.com/containers/podman/issues/14736)).
- Fixed a bug where the `podman systemd df` command incorrectly calculated reclaimable storage for volumes ([#13516](https://github.com/containers/podman/issues/13516)).
- Fixed a bug where an exported container checkpoint using a non-default OCI runtime could not be restored.
- Fixed a bug where Podman, when used with a recent runc version, could not remove paused containers.
- Fixed a bug where the remote Podman client's `podman manifest rm` command would remove images, not manifests ([#14763](https://github.com/containers/podman/issues/14763)).
- Fixed a bug where Podman did not correctly parse wildcards for device major number in the `podman run` and `podman create` commands' `--device-cgroup-rule` option.
- Fixed a bug where the `podman play kube` command on 32 bit systems where the total memory was calculated incorrectly ([#14819](https://github.com/containers/podman/issues/14819)).
- Fixed a bug where the `podman generate kube` command could set ports and hostname incorrectly in generated YAML ([#13030](https://github.com/containers/podman/issues/13030)).
- Fixed a bug where the `podman system df --format "{{ json . }}"` command would not output the `Size` and `Reclaimable` fields ([#14769](https://github.com/containers/podman/issues/14769)).
- Fixed a bug where the remote Podman client's `podman pull` command would display duplicate progress output.
- Fixed a bug where the `podman system service` command could leak memory when a client unexpectedly closed a connection when reading events or logs ([#14879](https://github.com/containers/podman/issues/14879)).
- Fixed a bug where Podman containers could fail to run if the image did not contain an `/etc/passwd` file ([#14966](https://github.com/containers/podman/issues/14966)).
- Fixed a bug where the remote Podman client's `podman push` command did not display progress information ([#14971](https://github.com/containers/podman/issues/14971)).
- Fixed a bug where a lock ordering issue could cause `podman pod rm` to deadlock if it was run at the same time as a command that attempted to lock multiple containers at once ([#14929](https://github.com/containers/podman/issues/14929)).
- Fixed a bug where the `podman rm --force` command would exit with a non-0 code if the container in question did not exist ([#14612](https://github.com/containers/podman/issues/14612)).
- Fixed a bug where the `podman container restore` command would fail when attempting to restore a checkpoint for a container with the same name as an image ([#15055](https://github.com/containers/podman/issues/15055)).
- Fixed a bug where the `podman manifest push --rm` command could remove image, instead of manifest lists ([#15033](https://github.com/containers/podman/issues/15033)).
- Fixed a bug where the `podman run --rm` command could fail to remove the container if it failed to start ([#15049](https://github.com/containers/podman/issues/15049)).
- Fixed a bug where the `podman generate systemd --new` command would create incorrect unit files when the container was created with the `--sdnotify` parameter ([#15052](https://github.com/containers/podman/issues/15052)).
- Fixed a bug where the `podman generate systemd --new` command would fail when `-h <hostname>` was used to create the container ([#15124](https://github.com/containers/podman/pull/15124)).

### API
- The Docker-compatible API now supports API version v1.41 ([#14204](https://github.com/containers/podman/issues/14204)).
- Fixed a bug where containers created via the Libpod API had an incorrect umask set ([#15036](https://github.com/containers/podman/issues/15036)).
- Fixed a bug where the `remote` parameter to the Libpod API's Build endpoint for Images was nonfunctional ([#13831](https://github.com/containers/podman/issues/13831)).
- Fixed a bug where the Libpod List endpoint for Containers did not return the `application/json` content type header when there were no containers present ([#14647](https://github.com/containers/podman/issues/14647)).
- Fixed a bug where the Compat Stats endpoint for Containers could return incorrect memory limits ([#14676](https://github.com/containers/podman/issues/14676)).
- Fixed a bug where the Compat List and Inspect endpoints for Containers could return incorrect strings for container status.
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle disabling healthchecks ([#14493](https://github.com/containers/podman/issues/14493)).
- Fixed a bug where the Compat Create endpoint for Networks did not support the `mtu`, `name`, `mode`, and `parent` options ([#14482](https://github.com/containers/podman/issues/14482)).
- Fixed a bug where the Compat Create endpoint for Networks did not allow the creation of networks name `bridge` ([#14983](https://github.com/containers/podman/issues/14983)).
- Fixed a bug where the Compat Inspect endpoint for Networks did not properly set netmasks in the `SecondaryIPAddresses` and `SecondaryIPv6Addresses` fields ([#14674](https://github.com/containers/podman/issues/14674)).
- The Libpod Stats endpoint for Pods now supports streaming output via two new parameters, `stream` and `delay` ([#14674](https://github.com/containers/podman/issues/14674)).

### Misc
- Podman will now check for nameservers in `/run/NetworkManager/no-stub-resolv.conf` if the `/etc/resolv.conf` file only contains a localhost server.
- The `podman build` command now supports caching with builds that specify `--squash-all` by allowing the `--layers` flag to be used at the same time.
- Podman Machine support for QEMU installations at non-default paths has been improved.
- The `podman machine ssh` command no longer prints spurious warnings every time it is run.
- When accessing the WSL prompt on Windows, the rootless user will be preferred.
- The `podman info` command now includes a field for information on supported authentication plugins for improved Docker compatibility. Authentication plugins are not presently supported by Podman, so this field is always empty.
- The `podman system prune` command now no longer prints the `Deleted Images` header if no images were pruned.
- The `podman system service` command now automatically creates and moves to a sub-cgroup when running in the root cgroup ([#14573](https://github.com/containers/podman/issues/14573)).
- Updated Buildah to v1.27.0
- Updated the containers/image library to v5.22.0
- Updated the containers/storage library to v1.42.0
- Updated the containers/common library to v0.49.1
- Podman will automatically create a sub-cgroup and move itself into it when it detects that it is running inside a container ([#14884](https://github.com/containers/podman/issues/14884)).
- Fixed an incorrect release note about regexp.
- A new MacOS installer (via pkginstaller) is now supported.

## 4.1.1
### Features
- Podman machine events are now supported on Windows.

### Changes
- The output of the `podman load` command now mirrors that of `docker load`.

### Bugfixes
- Fixed a bug where the `podman play kube` command could panic if the `--log-opt` option was used ([#13356](https://github.com/containers/podman/issues/13356)).
- Fixed a bug where Podman could, under some circumstances, fail to parse container cgroup paths ([#14146](https://github.com/containers/podman/issues/14146)).
- Fixed a bug where containers created with the `--sdnotify=conmon` option could send `MAINPID` twice.
- Fixed a bug where the `podman info` command could fail when run inside an LXC container.
- Fixed a bug where the pause image of a Pod with a custom ID mappings could not be built ([BZ 2083997](https://bugzilla.redhat.com/show_bug.cgi?id=2083997)).
- Fixed a bug where, on `podman machine` VMs on Windows, containers could be prematurely terminated with API forwarding was not running ([#13965](https://github.com/containers/podman/issues/13965)).
- Fixed a bug where removing a container with a zombie exec session would fail the first time, but succeed for subsequent calls ([#14252](https://github.com/containers/podman/issues/14252)).
- Fixed a bug where a dangling ID in the database could render Podman unusable.
- Fixed a bug where containers with memory limits could not be created when Podman was run in a root cgroup ([#14236](https://github.com/containers/podman/issues/14236)).
- Fixed a bug where the `--security-opt` option to `podman run` and `podman create` did not support the `no-new-privileges:true` and `no-new-privileges:false` options (the only supported separator was `=`, not `:`) ([#14133](https://github.com/containers/podman/issues/14133)).
- Fixed a bug where containers that did not create a network namespace (e.g. containers created with `--network none` or `--network ns:/path/to/ns`) could not be restored from checkpoints ([#14389](https://github.com/containers/podman/issues/14389)).
- Fixed a bug where `podman-restart.service` could, if enabled, cause system shutdown to hang for 90 seconds ([#14434](https://github.com/containers/podman/issues/14434)).
- Fixed a bug where the `podman stats` command would, when run as root on a container that had the `podman network disconnect` command run on it or that set a custom network interface name, return an error ([#13824](https://github.com/containers/podman/issues/13824)).
- Fixed a bug where the remote Podman client's `podman pod create` command would error when the `--uidmap` option was used ([#14233](https://github.com/containers/podman/issues/14233)).
- Fixed a bug where cleaning up systemd units and timers related to healthchecks was subject to race conditions and could fail.
- Fixed a bug where the default network mode of containers created by the remote Podman client was assigned by the client, not the server ([#14368](https://github.com/containers/podman/issues/14368)).
- Fixed a bug where containers joining a pod that was created with `--network=host` would receive a private network namespace ([#13763](https://github.com/containers/podman/issues/13763)).
- Fixed a bug where `podman machine rm --force` would remove files related to the VM before stopping it, causing issues if removal was interrupted.
- Fixed a bug where `podman logs` would omit the last line of a container's logs if the log did not end in a newline ([#14458](https://github.com/containers/podman/issues/14458)).
- Fixed a bug where network cleanup was nonfunctional for containers which used a custom user namespace and were initialized via API ([#14465](https://github.com/containers/podman/issues/14465)).
- Fixed a bug where some options (including volumes) for containers that joined pods were overwritten by the infra container ([#14454](https://github.com/containers/podman/issues/14454)).
- Fixed a bug where the `--file-locks` option to `podman container restore` was ignored, such that file locks checkpointed by `podman container checkpoint --file-locks` were not restored.
- Fixed a bug where signals sent to a Podman attach session with `--sig-proxy` enabled at the exact moment the container that was attached to exited could cause error messages to be printed.
- Fixed a bug where running the `podman machine start` command more than once (simultaneously) on the same machine would cause errors.
- Fixed a bug where the `podman stats` command could not be run on containers that were not running (it now reports all-0s statistics for Docker compatibility) ([#14498](https://github.com/containers/podman/issues/14498)).

### API
- Fixed a bug where images pulled from a private registry could not be accessed via shortname using the Compat API endpoints ([#14291](https://github.com/containers/podman/issues/14291)).
- Fixed a bug where the Compat Delete API for Images would return an incorrect status code (500) when attempting to delete images that are in use ([#14208](https://github.com/containers/podman/issues/14208)).
- Fixed a bug where the Compat Build API for Images would include the build's `STDERR` output even if the `quiet` parameter was true.
- Fixed a bug where the Libpod Play Kube API would overwrite any log driver specified by query parameter with the system default.

### Misc
- The `podman auto-update` command now creates an event when it is run.
- Error messages printed when Podman's temporary files directory is not writable have been improved.
- Units for memory limits accepted by Podman commands were incorrectly stated by documentation as megabytes, instead of mebibytes; this has now been corrected ([#14187](https://github.com/containers/podman/issues/14187)).

## 4.1.0
### Features
- Podman now supports Docker Compose v2.2 and higher ([#11822](https://github.com/containers/podman/issues/11822)). Please note that it may be necessary to disable the use of Buildkit by setting the environment variable `DOCKER_BUILDKIT=0`.
- A new container command has been added, `podman container clone`. This command makes a copy of an existing container, with the ability to change some settings (e.g. resource limits) while doing so.
- A new machine command has been added, `podman machine inspect`. This command provides details on the configuration of machine VMs.
- The `podman machine set` command can now change the CPUs, memory, and disk space available to machines after they were initially created, using the new `--cpus`, `--disk-size`, and `--memory` options ([#13633](https://github.com/containers/podman/issues/13633)).
- Podman now supports sending JSON events related to machines to a Unix socket named `machine_events.*\.sock` in `XDG_RUNTIME_DIR/podman` or to a socket whose path is set in the `PODMAN_MACHINE_EVENTS_SOCK` environment variable.
- Two new volume commands have been added, `podman volume mount` and `podman volume unmount`. These allow for Podman-managed named volumes to be mounted and accessed from outside containers ([#12768](https://github.com/containers/podman/issues/12768)).
- VMs created by `podman machine` now automatically mount the host's `$HOME` into the VM, to allow mounting volumes from the host into containers.
- The `podman container checkpoint` and `podman container restore` options now support checkpointing to and restoring from OCI images. This allows checkpoints to be distributed via standard image registries.
- The `podman play kube` command now supports environment variables that are specified using the `fieldRef` and `resourceFieldRef` sources.
- The `podman play kube` command will now set default resource limits when the provided YAML does not include them ([#13115](https://github.com/containers/podman/issues/13115)).
- The `podman play kube` command now supports a new option, `--annotation`, to add annotations to created containers ([#12968](https://github.com/containers/podman/issues/12968)).
- The `podman play kube --build` command now supports a new option, `--context-dir`, which allows the user to specify the context directory to use when building the Containerfile ([#12485](https://github.com/containers/podman/issues/12485)).
- The `podman container commit` command now supports a new option, `--squash`, which squashes the generated image into a single layer ([#12889](https://github.com/containers/podman/issues/12889)).
- The `podman pod logs` command now supports two new options, `--names`, which identifies which container generated a log message by name, instead of ID ([#13261](https://github.com/containers/podman/issues/13261)) and `--color`, which colors messages based on what container generated them ([#13266](https://github.com/containers/podman/issues/13266)).
- The `podman rmi` command now supports a new option, `--ignore`, which will ignore errors caused by missing images.
- The `podman network create` command now features a new option, `--ipam-driver`, to specify details about how IP addresses are assigned to containers in the network ([#13521](https://github.com/containers/podman/issues/13521)).
- The `podman machine list` command now features a new option, `--quiet`, to print only the names of configured VMs and no other information.
- The `--ipc` option to the `podman create`, `podman run`, and `podman pod create` commands now supports three new modes: `none`, `private`, and `shareable`. The default IPC mode is now `shareable`, indicating the IPC namespace can be shared with other containers ([#13265](https://github.com/containers/podman/issues/13265)).
- The `--mount` option to the `podman create` and `podman run` commands can now set options for created named volumes via the `volume-opt` parameter ([#13387](https://github.com/containers/podman/issues/13387)).
- The `--mount` option to the `podman create` and `podman run` commands now allows parameters to be passed in CSV format ([#13922](https://github.com/containers/podman/issues/13922)).
- The `--userns` option to the `podman create` and `podman run` commands now supports a new option, `nomap`, that (only for rootless containers) does not map the UID of the user that started the container into the container, increasing security.
- The `podman import` command now supports three new options, `--arch`, `--os`, and `--variant`, to specify what system the imported image was built for.
- The `podman inspect` command now includes information on the network configuration of containers that joined a pre-configured network namespace with the `--net ns:` option to `podman run`, `podman create`, and `podman pod create`.
- The `podman run` and `podman create` commands now support a new option, `--chrootdirs`, which specifies additional locations where container-specific files managed by Podman (e.g. `/etc/hosts`, `/etc/resolv.conf, etc) will be mounted inside the container ([#12961](https://github.com/containers/podman/issues/12691)).
- The `podman run` and `podman create` commands now support a new option, `--passwd-entry`, allowing entries to be added to the container's `/etc/passwd` file.
- The `podman images --format` command now accepts two new format directives: `{{.CreatedAt}}` and `{{.CreatedSince}}` ([#14012](https://github.com/containers/podman/issues/14012)).
- The `podman volume create` command's `-o` option now accepts a new argument, `o=noquota`, to disable XFS quotas entirely and avoid potential issues when Podman is run on an XFS filesystem with existing quotas defined ([#14049](https://github.com/containers/podman/issues/14049)).
- The `podman info` command now includes additional information on the machine Podman is running on, including disk utilization on the drive Podman is storing containers and images on, and CPU utilization ([#13876](https://github.com/containers/podman/issues/13876)).

### Changes
- The `--net=container:` option to `podman run`, `podman create`, and `podman pod create` now conflicts with the `--add-host` option.
- As part of a deprecation of the SHA1 hash algorithm within Podman, the algorithm used to generate the filename of the rootless network namespace has been changed. As a result, rootless containers started before updating to Podman 4.1.0 will need to be restarted if they are joined to a network (and not just using `slirp4netns`) to ensure they can connect to containers started the upgrade.
- Podman's handling of the `/etc/hosts` file has been rewritten to improve its consistency and handling of edge cases ([#12003](https://github.com/containers/podman/issues/12003) and [#13224](https://github.com/containers/podman/issues/13224)). As part of this, two new options are available in `containers.conf`: `base_hosts_file` (to specify a nonstandard location to source the base contents of the container's `/etc/hosts`) and `host_containers_internal_ip` (to specify a specific IP address for containers' `host.containers.internal` entry to point to).
- The output of the `podman image trust show` command now includes information on the transport mechanisms allowed.
- Podman now exits cleanly (with exit code 0) after receiving SIGTERM.
- Containers running in systemd mode now set the `container_uuid` environment variable ([#13187](https://github.com/containers/podman/issues/13187)).
- Renaming a container now generates an event readable through `podman events`.
- The `--privileged` and `--cap-add` flags are no longer mutually exclusive ([#13449](https://github.com/containers/podman/issues/13449)).
- Fixed a bug where the `--mount` option to `podman create` and `podman run` could not create anonymous volumes ([#13756](https://github.com/containers/podman/issues/13756)).
- Fixed a bug where Podman containers where the user did not explicitly set an OOM score adjustment would implicitly set a value of 0, instead of not setting one at all ([#13731](https://github.com/containers/podman/issues/13731)).
- The `podman machine set` command can no longer be used while the VM being updated is running ([#13783](https://github.com/containers/podman/issues/13783)).
- Systemd service files created by `podman generate systemd` are now prettyprinted for increased readability.
- The `file` event log driver now automatically rotates the log file, preventing it from growing beyond a set size.
- The `--no-trunc` flag to `podman search` now defaults to `false`, to ensure output is not overly verbose.

### Bugfixes
- Fixed a bug where Podman could not add devices with a major or minor number over 256 to containers.
- Fixed a bug where containers created by the `podman play kube` command did not record the raw image name used to create containers.
- Fixed a bug where VMs created by `podman machine` could not start containers which forwarded ports when run on a host with a proxy configured ([#13628](https://github.com/containers/podman/issues/13628)).
- Fixed a bug where VMs created by the `podman machine` command could not be connected to when the username of the current user was sufficiently long ([#12751](https://github.com/containers/podman/issues/12751)).
- Fixed a bug where the `podman system reset` command on Linux did not fully remove virtual machines created by `podman machine`.
- Fixed a bug where the `podman machine rm` command would error when removing a VM that was never started ([#13834](https://github.com/containers/podman/issues/13834)).
- Fixed a bug where the remote Podman client's `podman manifest push` command could not push to registries that required authentication ([#13629](https://github.com/containers/podman/issues/13629)).
- Fixed a bug where containers joining a pod with volumes did not have the pod's volumes added ([#13548](https://github.com/containers/podman/issues/13548)).
- Fixed a bug where the `podman version --format` command could not return the OS of the server ([#13690](https://github.com/containers/podman/issues/13690)).
- Fixed a bug where the `podman play kube` command would error when a volume specified by a `configMap` already existed ([#13715](https://github.com/containers/podman/issues/13715)).
- Fixed a bug where the `podman play kube` command did not respect the `hostNetwork` setting in Pod YAML ([#14015](https://github.com/containers/podman/issues/14015)).
- Fixed a bug where the `podman play kube` command would, when the `--log-driver` flag was not specified, ignore Podman's default log driver ([#13781](https://github.com/containers/podman/issues/13781)).
- Fixed a bug where the `podman generate kube` command could generate YAML with too-long labels ([#13962](https://github.com/containers/podman/issues/13962)).
- Fixed a bug where the `podman logs --tail=1` command would fail when the log driver was `journald` and the container was restarted ([#13098](https://github.com/containers/podman/issues/13098)).
- Fixed a bug where containers created from images with a healthcheck that did not specify an interval would never run their healthchecks ([#13912](https://github.com/containers/podman/issues/13912)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands could leave invalid entries in `/etc/hosts` ([#13533](https://github.com/containers/podman/issues/13533)).
- Fixed a bug where the `--tls-verify option to the `remote Podman client's `podman build` command was nonfunctional.
- Fixed a bug where the `podman pod inspect` command incorrectly reported whether the pod used the host's network ([#14028](https://github.com/containers/podman/issues/14028)).
- Fixed a bug where Podman would, when run on WSL2, ports specified without an IP address (e.g. `-p 8080:8080`) would be bound to IPv6 addresses ([#12292](https://github.com/containers/podman/issues/12292)).
- Fixed a bug where the remote Podman client's  `podman info` could report an incorrect path to the socket used to access the Podman service ([#12023](https://github.com/containers/podman/issues/12023)).

### API
- Containers created via the Libpod Create API that set a memory limit, but not a swap limit, will automatically have a swap limit set ([#13145](https://github.com/containers/podman/issues/13145)).
- The Compat and Libpod Attach APIs for Containers can now attach to Stopped containers.
- Fixed a bug where the Compat and Libpod Create APIs for Containers did not respect the `no_hosts` option in `containers.conf` ([#13719](https://github.com/containers/podman/issues/13719)).
- Fixed a bug where the default network mode for rootless containers created via the Compat Create API was not `bridge`.
- Fixed a bug where the Libpod List API for Containers did not allow filtering based on the `removing` status ([#13986](https://github.com/containers/podman/issues/13986)).
- Fixed a bug where the Libpod Modify endpoint for Manifests did not respect the `tlsVerify` parameter.

### Misc
- A number of dependencies have been pruned from the project, resulting in a significant reduction in the size of the Podman binary.
- Using `podman play kube` on a YAML that only includes `configMap` objects (and no pods or deployments) now prints a much clearer error message.
- Updated Buildah to v1.26.1
- Updated the containers/storage library to v1.40.2
- Updated the containers/image library to v5.21.1
- Updated the containers/common library to v0.48.0

## 4.0.3
### Security
- This release fixes CVE-2022-27649, where containers run by Podman would have excess inheritable capabilities set.

### Changes
- The `podman machine rm --force` command will now remove running machines as well (such machines are shut down first, then removed) ([#13448](https://github.com/containers/podman/issues/13448)).
- When a `podman machine` VM is started that is using a too-old VM image, it will now start in a reduced functionality mode, and provide instructions on how to recreate it (previously, VMs were effectively unusable) ([#13510](https://github.com/containers/podman/issues/13510)).

### Bugfixes
- Fixed a bug where devices added to containers by the `--device` option to `podman run` and `podman create` would not be accessible within the container.
- Fixed a bug where Podman would refuse to create containers when the working directory in the container was a symlink ([#13346](https://github.com/containers/podman/issues/13346)).
- Fixed a bug where pods would be created with cgroups even if cgroups were disabled in `containers.conf` ([#13411](https://github.com/containers/podman/issues/13411)).
- Fixed a bug where the `podman play kube` command would produce confusing errors if invalid YAML with duplicated container named was passed ([#13332](https://github.com/containers/podman/issues/13332)).
- Fixed a bug where the `podman machine rm` command would not remove the Podman API socket on the host that was associated with the VM.
- Fixed a bug where the remote Podman client was unable to properly resize the TTYs of containers on non-Linux OSes.
- Fixed a bug where rootless Podman could hang indefinitely when starting containers on systems with IPv6 disabled ([#13388](https://github.com/containers/podman/issues/13388)).
- Fixed a bug where the `podman version` command could sometimes print excess blank lines as part of its output.
- Fixed a bug where the `podman generate systemd` command would sometimes generate systemd services with names beginning with a hyphen ([#13272](https://github.com/containers/podman/issues/13272)).
- Fixed a bug where locally building the pause image could fail if the current directory contained a `.dockerignore` file ([#13529](https://github.com/containers/podman/issues/13529)).
- Fixed a bug where root containers in VMs created by `podman machine` could not bind ports to specific IPs on the host ([#13543](https://github.com/containers/podman/issues/13543)).
- Fixed a bug where the storage utilization percentages displayed by `podman system df` were incorrect ([#13516](https://github.com/containers/podman/issues/13516)).
- Fixed a bug where the CPU utilization percentages displayed by `podman stats` were incorrect ([#13597](https://github.com/containers/podman/pull/13597)).
- Fixed a bug where containers created with the `--no-healthcheck` option would still display healthcheck status in `podman inspect` ([#13578](https://github.com/containers/podman/issues/13578)).
- Fixed a bug where the `podman pod rm` command could print a warning about a missing cgroup ([#13382](https://github.com/containers/podman/issues/13382)).
- Fixed a bug where the `podman exec` command could sometimes print a `timed out waiting for file` error after the process in the container exited ([#13227](https://github.com/containers/podman/issues/13227)).
- Fixed a bug where virtual machines created by `podman machine` were not tolerant of changes to the path to the qemu binary on the host ([#13394](https://github.com/containers/podman/issues/13394)).
- Fixed a bug where the remote Podman client's `podman build` command did not properly handle the context directory if a Containerfile was manually specified using `-f` ([#13293](https://github.com/containers/podman/issues/13293)).
- Fixed a bug where Podman would not properly detect the use of `systemd` as PID 1 in a container when the entrypoint was prefixed with `/bin/sh -c` ([#13324](https://github.com/containers/podman/issues/13324)).
- Fixed a bug where rootless Podman could, on systems that do not use `systemd` as init, print a warning message about the rootless network namespace ([#13703](https://github.com/containers/podman/issues/13703)).
- Fixed a bug where the default systemd unit file for `podman system service` did not delegate all cgroup controllers, resulting in `podman info` queries against the remote API returning incorrect cgroup controllers ([#13710](https://github.com/containers/podman/issues/13710)).
- Fixed a bug where the `slirp4netns` port forwarder for rootless Podman would only publish the first port of a range ([#13643](https://github.com/containers/podman/issues/13643)).

### API
- Fixed a bug where the Compat Create API for containers did not properly handle permissions for tmpfs mounts ([#13108](https://github.com/containers/podman/issues/13108)).

### Misc
- The static binary for Linux is now built with CGo disabled to avoid panics due to a Golang bug ([#13557](https://github.com/containers/podman/issues/13557)).
- Updated Buildah to v1.24.3
- Updated the containers/storage library to v1.38.3
- Updated the containers/image library to v5.19.2
- Updated the containers/common library to v0.47.5

## 4.0.2
### Bugfixes
- Revert "use GetRuntimeDir() from c/common"

## 4.0.1
### Bugfixes
- Fixed a bug where the `podman play kube` command did not honor the `mountPropagation` field in Pod YAML ([#13322](https://github.com/containers/podman/issues/13322)).
- Fixed a bug where the `--build=false` option to `podman play kube` was not honored ([#13285](https://github.com/containers/podman/issues/13285)).
- Fixed a bug where a container using volumes from another container (via `--volumes-from`) could, under certain circumstances, exit with errors that it could not delete some volumes if the other container did not exit before it ([#12808](https://github.com/containers/podman/issues/12808)).
- Fixed a bug where the `CONTAINERS_CONF` environment variable was not propagated to Conmon, which could result in Podman cleanup processes being run with incorrect configurations.

## 4.0.0
### Security
- This release addresses CVE-2022-1227, where running `podman top` on a container made from a maliciously-crafted image and using a user namespace could allow for code execution in the host context.

### Features
- Podman has seen an extensive rewrite of its network stack to add support for Netavark, a new tool for configuring container networks, in addition to the existing CNI stack. Netavark will be default on new installations when it is available.
- The `podman network connect` command now supports three new options, `--ip`, `--ip6`, and `--mac-address`, to specify configuration for the new network that will be attached.
- The `podman network create` command now allows the `--subnet`, `--gateway`, and `--ip-range` options to be specified multiple times, to allow for the creation of dual-stack IPv4 and IPv6 networks with user-specified subnets.
- The `--network` option to `podman create`, `podman pod create`, `podman run`, and `podman play kube` can now, when specifying a network name, also specify advanced network options such as `alias`, `ip`, `mac`, and `interface_name`, allowing advanced configuration of networks when creating containers connected to more than one network.
- The `podman play kube` command can now specify the `--net` option multiple times, to connect created containers and pods to multiple networks.
- The `podman create`, `podman pod create`, and `podman run` commands now support a new option, `--ip6`, to specify a static IPv6 address for the created container or pod to use.
- Macvlan networks can now configure the mode of the network via the `-o mode=` option.
- When using the CNI network stack, a new network driver, `ipvlan`, is now available.
- The `podman info` command will now print the network backend in use (Netavark or CNI).
- The network backend to use can be now be specified in `containers.conf` via the `network_backend` field. Please note that it is not recommended to switch backends while containers exist, and a system reboot is recommended after doing so.
- All Podman commands now support a new option, `--noout`, that suppresses all output to STDOUT.
- All commands that can remove containers (`podman rm --force`, `podman pod rm --force`, `podman volume rm --force`, `podman network rm --force`) now accept a `--time` option to specify the timeout on stopping the container before resorting to `SIGKILL` (identical to the `--time` flag to `podman stop`).
- The `podman run` and `podman create` commands now support a new option, `--passwd`, that uses the `/etc/passwd` and `/etc/groups` files from the image in the created container without changes by Podman ([#11805](https://github.com/containers/podman/issues/11805)).
- The `podman run` and `podman create` commands now support a new option, `--hostuser`, that creates one or more users in the container based on users from the host (e.g. with matching username, UID, and GID).
- The `podman create` and `podman run` commands now support two new options, `--unsetenv` and `--unsetenv-all`, to clear default environment variables set by Podman and by the container image ([#11836](https://github.com/containers/podman/issues/11836)).
- The `podman rm` command now supports a new option, `--depend`, which recursively removes a given container and all containers that depend on it ([#10360](https://github.com/containers/podman/issues/10360)).
- All commands that support filtering their output based on labels (e.g. `podman volume ls`, `podman ps`) now support labels specified using glob matching (e.g. `--filter label=some.prefix.com/key/*`).
- The `podman pod create` command now supports the `--volume` option, allowing volumes to be specified that will be mounted automatically to all containers in the pod ([#10379](https://github.com/containers/podman/issues/10379)).
- The `podman pod create` command now supports the `--device` option, allowing devices to be specified that will be mounted automatically to all containers in the pod.
- The `podman pod create` command now supports the `--volumes-from` option, allowing volumes from an existing Podman container to be mounted automatically to all containers in the pod.
- The `podman pod create` command now supports the `--security-opt` option, allowing security settings (e.g. disabling SELinux or Seccomp) to be configured automatically for all containers in the pod ([#12173](https://github.com/containers/podman/issues/12173)).
- The `podman pod create` command now supports the `--share-parent` option, which defaults to true, controlling whether containers in the pod will use a shared cgroup parent.
- The `podman pod create` command now supports the `--sysctl` option, allowing sysctls to be configured automatically for all containers in the pod.
- The `podman events` command now supports the `--no-trunc` option, which will allow short container IDs to be displayed instead of the default full IDs. The flag defaults to true, so full IDs remain the default ([#8941](https://github.com/containers/podman/issues/8941)).
- The `podman machine init` command now supports a new VM type, `wsl`, available only on Windows; this uses WSL as a backend for `podman machine`, instead of creating a separate VM and managing it via QEMU ([#12503](https://github.com/containers/podman/pull/12503)).
- The `podman machine init` command now supports a new option, `--now`, to start the VM immediately after creating it.
- The `podman machine init` command now supports a new option, `--volume`, to mount contents from the host into the created virtual machine.
- Virtual machines created by `podman machine` now automatically mount the Podman API socket to the host, so consumers of the Podman or Docker APIs can use them directly from the host machine ([#11462](https://github.com/containers/podman/issues/11462)).
- Virtual machines created by `podman machine` now automatically mount certificates from the host's keychain into the virtual machine ([#11507](https://github.com/containers/podman/issues/11507)).
- Virtual machines created by `podman machine` now automatically propagate standard proxy environment variables from the host into the virtual machine, including copying any required certificates from `SSL_FILE_CERT` into the VM.
- The `podman machine ssh` command now supports a new option, `--username`, to specify the username to connect to the VM with.
- Port forwarding from VMs created using `podman machine` now supports ports specified using custom host IPs (e.g. `-p 127.0.0.1:8080:80`), the UDP protocol, and containers created using the `slirp4netns` network mode ([#11528](https://github.com/containers/podman/issues/11528) and [#11728](https://github.com/containers/podman/issues/11728)).
- The `podman system connection rm` command supports a new option, `--all`, to remove all available connections ([#12018](https://github.com/containers/podman/issues/12018)).
- The `podman system service` command's default timeout is now configured via `containers.conf` (using the `service_timeout` field) instead of hardcoded to 5 seconds.
- The `--mount type=devpts` option to `podman create` and `podman run` now supports new options: `uid`, `gid`, `mode`, and `max`.
- The `--volume` option to `podman create` and `podman run` now supports a new option, `:idmap`, which using an ID mapping filesystem to allow multiple containers with disjoint UID and GID ranges mapped into them access the same volume ([#12154](https://github.com/containers/podman/issues/12154)).
- The `U` option for volumes, which changes the ownership of the mounted volume to ensure the user running in the container can access it, can now be used with the `--mount` option to `podman create` and `podman run`, as well as the `--volume` option where it was already available.
- The `:O` option for volumes, which specifies that an overlay filesystem will be mounted over the volume and ensures changes do not persist, is now supported with named volumes as well as bind mounts.
- The `:O` option for volumes now supports two additional options, `upperdir` and `workdir`, which allow for specifying custom upper directories and work directories for the created overlay filesystem.
- Podman containers created from a user-specified root filesystem (via `--rootfs`) can now create an overlay filesystem atop the user-specified rootfs which ensures changes will not persist by suffixing the user-specified root filesystem with `:O`.
- The `podman save` command has a new option, `--uncompressed`, which saves the layers of the image without compression ([#11613](https://github.com/containers/podman/issues/11613)).
- Podman supports a new log driver for containers, `passthrough`, which logs all output directly to the STDOUT and STDERR of the `podman` command; it is intended for use in systemd-managed containers.
- The `podman build` command now supports two new options, `--unsetenv` and `--all-platforms`.
- The `podman image prune` command now supports a new option, `--external`, which allows containers not created by Podman (e.g. temporary containers from Buildah builds) to be pruned ([#11472](https://github.com/containers/podman/issues/11472)).
- Two new aliases for `podman image prune` have been added for Docker compatibility: `podman builder prune` and `podman buildx prune`.
- The `podman play kube` command now supports a new option, `--no-hosts`, which uses the `/etc/hosts` file from the image in all generated containers, preventing any modifications to the hosts file from Podman ([#9500](https://github.com/containers/podman/issues/9500)).
- The `podman play kube` command now supports a new option, `--replace`, which will replace any existing containers and pods with the same names as the containers and pods that will be created by the command ([#11481](https://github.com/containers/podman/issues/11481)).
- The `podman play kube` command now supports a new option, `--log-opt`, which allows the logging configuration of generated containers and pods to be adjusted ([#11727](https://github.com/containers/podman/issues/11727)).
- The `podman play kube` command now supports Kubernetes YAML that specifies volumes from a configmap.
- The `podman generate systemd` command now supports a new option, `--template`, to generate template unit files.
- The `podman generate systemd` command now supports a new option, `--start-timeout`, to override the default start timeout for generated unit files ([#11618](https://github.com/containers/podman/issues/11618)).
- The `podman generate systemd` command now supports a new option, `--restart-sec`, to override the default time before a failed unit is restarted by systemd for generated unit files.
- The `podman generate systemd` command now supports three new options, `--wants`, `--after`, and `--requires`, which allow detailed control of systemd dependencies in generated unit files.
- The `podman container checkpoint` and `podman container restore` commands can now print statistics about the checkpoint operation via a new option, `--print-stats`.
- The `podman container checkpoint` and `podman container restore` commands can now checkpoint and restore containers which make use of file locks via a new option, `--file-locks`.
- The `podman container restore` command can now be used with containers created using the host IPC namespace (`--ipc=host`).
- The `podman container checkpoint` and `podman container restore` commands now handle checkpointing and restoring the contents of `/dev/shm`.
- The `podman container checkpoint` and `podman container restore` commands are now supported with the remote Podman client ([#12007](https://github.com/containers/podman/issues/12007)).
- The `podman inspect` command on containers now includes additional output fields for checkpointed and restored containers, including information about when the container was checkpointed or restored, and the path to the checkpoint/restore log.
- The `podman secret list` command now supports a new option, `--filter`, to filter what secrets are returned.
- The `podman image scp` command can now be used to transfer images between users (both root and rootless) on the same system, without requiring `sshd`.
- The `podman image sign` command now supports a new option, `--authfile`, to specify an alternative path to authentication credentials ([#10866](https://github.com/containers/podman/issues/10866)).
- The `podman load` command now supports downloading files via HTTP and HTTPS if a URL is given ([#11970](https://github.com/containers/podman/issues/11970)).
- The `podman push` command now supports a new option, `--compression-format`, to choose the compression algorithm used to compress image layers.
- The `podman volume create` command now allows volumes using the `local` driver that require mounting to be used by non-root users. This allows `tmpfs` and `bind` volumes to be created by non-root users ([#12013](https://github.com/containers/podman/issues/12013)).
- A new command, `podman dial-stdio`, has been added; this command should not be invoked directly, but is used by some clients of the Docker Remote API, and is provided for Docker compatibility ([#11668](https://github.com/containers/podman/issues/11668)).

### Breaking Changes
- Podman v4.0 will perform several schema migrations in the Podman database when it is first run. These schema migrations will cause Podman v3.x and earlier to be unable to read certain network configuration information from the database, so downgrading from Podman v4.0 to an earlier version will cause containers to lose their static IP, MAC address, and port bindings.
- All endpoints of the Docker-compatible API now enforce that all image shortnames will be resolved to the Docker Hub for improved Docker compatibility. This behavior can be turned off via the `compat_api_enforce_docker_hub` option in `containers.conf` ([#12320](https://github.com/containers/podman/issues/12320)).
- The Podman APIs for Manifest List and Network operations have been completely rewritten to address issues and inconsistencies in the previous APIs. Incompatible APIs should warn if they are used with an older Podman client.
- The `make install` makefile target no longer implicitly builds Podman, and will fail if `make` was not run prior to it.
- The `podman rm --depends`, `podman rmi --force`, and `podman network rm --force` commands can now remove pods if a they need to remove an infra container (e.g. `podman rmi --force` on the infra image will remove all pods and infra containers). Previously, any command that tried to remove an infra container would error.
- The `podman system reset` command now removes all networks on the system, in addition to all volumes, pods, containers, and images.
- If the `CONTAINER_HOST` environment variable is set, Podman will default to connecting to the remote Podman service specified by the environment variable, instead of running containers locally ([#11196](https://github.com/containers/podman/issues/11196)).
- Healthcheck information from `podman inspect` on a container has had its JSON tag renamed from `Healthcheck` to `Health` for improved Docker compatibility. An alias has been added so that using the old name with the `--format` option will still work ([#11645](https://github.com/containers/podman/issues/11645)).
- Secondary IP and IPv6 addresses from `podman inspect` on a container (`SecondaryIPAddresses` and `SecondaryIPv6Addresses`) have been changed from arrays of strings to arrays of structs for improved Docker compatibility (the struct now includes IP address and prefix length).
- The `podman volume rm --force` command will now remove containers that depend on the volume that are running (previously, it would only remove stopped containers).
- The output of the `podman search` command has been altered to remove the Index, Stars, and Automated columns, as these were not used by registries that are not Dockerhub.
- The `host.containers.internal` entry in `/etc/hosts` for rootless containers now points to a public IP address of the host machine, to ensure the container can reach the host (the previous value, a slirp4netns address, did not actually point to the host) ([#12000](https://github.com/containers/podman/issues/12000)).
- Containers created in pods that have an infra container can no longer independently configure a user namespace via `--uidmap` and `--gidmap` ([#12669](https://github.com/containers/podman/issues/12669)).
- Several container states have been renamed internally - for example, the previous `Configured` state is now named `Created`, and the previous `Created` state is now `Initialized`. The `podman ps` command already normalized these names for Docker compatibility, so this will only be visible when inspecting containers with `podman inspect`.

### Changes
- Podman containers will now automatically add the container's short ID as a network alias when connected to a supporting network ([#11748](https://github.com/containers/podman/issues/11748)).
- The `podman machine stop` command will now log when machines are successfully stopped ([#11542](https://github.com/containers/podman/issues/11542)).
- The `podman machine stop` command now waits until the VM has stopped to return; previously, it returned immediately after the shutdown command was sent, without waiting for the VM to shut down.
- VMs created by `podman machine` now delegate more cgroup controllers to the rootless user used to run containers, allowing for additional resource limits to be used ([#13054](https://github.com/containers/podman/issues/13054)).
- The `podman stop` command will now log a warning to the console if the stop timeout expires and `SIGKILL` must be used to stop the container ([#11854](https://github.com/containers/podman/issues/11854)).
- Several performance optimizations have been implemented that should speed up container and pod creation, and running containers and pods that forward large ranges of ports.
- The `--no-trunc` argument to the `podman search` command now defaults to true.
- Rootless port forwarding using the `rootlessport` port forwarder is now handled by a separate binary, not Podman itself, which results in significantly reduced memory usage ([#10790](https://github.com/containers/podman/issues/10790)).
- The `podman system connection ls` command now has a separate output column to show which connection is currently the default (instead appending `*` to the default connection's name) ([#12019](https://github.com/containers/podman/issues/12019)).
- The `--kernel-memory` option to `podman run` and `podman create` has been deprecated in the upstream OCI runtime specification, and is now also deprecated in Podman and will be removed in a future release. Use of the flag will result in a warning.
- Podman will now ship build the pause image used by pods locally, instead of pulling it from the network (using the existing `catatoinit` binary used for `podman run --init`). This allows pods to be easily used on systems without an internet connection.
- The `--rootless-cni` option to `podman unshare` has been renamed to `--rootless-netns`. The old name has been aliased to the new one and will still function, but may be removed in a future release.
- The `--cni-config-dir` option to all Podman commands has been renamed to `--network-config-dir` as it will not be used with Netavark as well as CNI. The old name has been aliased to the new one and will still function, but may be removed in a future release.
- The `--format` option to all Podman commands has been changed to improved functionality and Docker compatibility ([#10974](https://github.com/containers/podman/issues/10974)).
- The `podman ps --external` flag previously required `--all` to also be specified; this is no longer true
- The port-forwarding logic previously contined in the `podman-machine-cni` CNI plugin has been integrated directly into Podman. The `podman-machine-cni` plugin is no longer necessary and should be removed.
- The `--device` flag to `podman create`, `podman run`, and `podman pod create` would previously refuse to mount devices when Podman was run as a non-root user and no permission to access the device was available; it will now mount these devices without checking permissions ([#12704](https://github.com/containers/podman/issues/12704)).

### Bugfixes
- Fixed a bug where networks could be created with the same name as a container network mode (e.g. `host`) ([#11448](https://github.com/containers/podman/issues/11448)).
- Fixed a bug where the `podman save` command was not automatically removing signatures from saved images.
- Fixed a bug where a rare race condition could cause `podman run --rm` to return an error that a given container did not exist when trying to remove it, despite it having been safely removed ([#11775](https://github.com/containers/podman/issues/11775)).
- Fixed a bug where a rare race condition could cause `podman ps` to return an error if a container was removed while the command was running ([#11810](https://github.com/containers/podman/issues/11810)).
- Fixed a bug where running Kube YAML with a CPU limit would using `podman play kube` would result in errors ([#11803](https://github.com/containers/podman/issues/11803)).
- Fixed a bug where creating a pod without an infra container would not generate an Pod Create event.
- Fixed a bug where volumes created with the `:z` and `:Z` options would be relabelled every time a container was started, not just the first time.
- Fixed a bug where the `podman tag` command on a manifest list could tag an image in the manifest, and not the manifest list itself.
- Fixed a bug where creating a volume using an invalid volume option that contained a format string would print a nonsensical error.
- Fixed a bug where Podman would not create a healthcheck for containers created from images that specified a healthcheck in their configuration ([#12226](https://github.com/containers/podman/issues/12226)).
- Fixed a bug where the output of healthchecks was not shown in `podman inspect` ([#13083](https://github.com/containers/podman/issues/13083)).
- Fixed a bug where rootless containers that used a custom user namespace (e.g. `--userns=keep-id`) could not have any ports forwarded to them.
- Fixed a bug where the `podman system connection ls` command would not print any output (including headers) if no connections were present.
- Fixed a bug where the `--memory-swappiness` option to `podman create` and `podman run` did not accept 0 as a valid value.
- Fixed a bug where environment variables specified in `containers.conf` for Podman would sometimes not be applied ([#12296](https://github.com/containers/podman/issues/12296)).
- Fixed a bug where running multiple rootless Podman instances with different configurations on the same system could cause networking issues due to the use of a single, shared rootless network namespace ([#12306](https://github.com/containers/podman/issues/12306)).
- Fixed a bug where rootless containers using bridge networking would fail if `/etc/resolv.conf` was a symlink to a directory ([#12461](https://github.com/containers/podman/issues/12461)).
- Fixed a bug where `podman container restore` could sometimes restore containers with a different OCI runtime than they had been using before they were checkpointed.
- Fixed a bug where some commands of the remote Podman client allowed the `--signature-policy` option to be used (with no effect); `--signature-policy` is not supported by the remote client ([#12357](https://github.com/containers/podman/issues/12357)).
- Fixed a bug where images which specified a port range in `EXPOSE` could not be run ([#12293](https://github.com/containers/podman/issues/12293)).
- Fixed a bug where Podman would resolve image names without a tag to any tag of that image available on the local system, instead of the `:latest` tag ([#11964](https://github.com/containers/podman/issues/11964)).
- Fixed a bug where the `--blkio-weight-device` option to `podman create` and `podman run` was nonfunctional.
- Fixed a bug where the `podman generate systemd` command did not support container entrypoints that were specified as JSON arrays ([#12477](https://github.com/containers/podman/issues/12477)).
- Fixed a bug where rootless Podman could, under some circumstances, exhaust all available inotify watches ([#11825](https://github.com/containers/podman/issues/11825)).
- Fixed a bug where, when a container was created with both the `--hostname` and `--pod new:` options, the hostname would be discarded; it is now set as the hostname of the created pod, which will be used by the container.
- Fixed a bug where the order in which `podman network ls` printed networks was not deterministic.
- Fixed a bug where the `podman kill` command would sometimes not print the ID of containers that were killed.
- Fixed a bug where VMs created by `podman machine` did not match their timezone to the host system ([#11895](https://github.com/containers/podman/issues/11895)).
- Fixed a bug where container healthchecks were not properly cleaning up generated systemd services, leading to healthcheck failures after containers were restarted.
- Fixed a bug where the `podman build` command did not properly propagate non-0 exit codes from Buildah when builds failed.
- Fixed a bug where the remote Podman client's `podman build` command could fail to build images when the remote client was run on Windows and the Containerfile contained `COPY` instructions ([#13119](https://github.com/containers/podman/issues/13119)).
- Fixed a bug where the remote Podman client's `--secret` option to the `podman build` command was nonfunctional.
- Fixed a bug where the remote Podman client's `podman build` command would error if given a relative path to a Containerfile ([#12841](https://github.com/containers/podman/issues/12841) and [#12763](https://github.com/containers/podman/issues/12763)).
- Fixed a bug where the `podman generate kube` command would sometimes omit environment variables set in containers from generated YAML.
- Fixed a bug where setting `userns=auto` in `containers.conf` was not respected ([#12615](https://github.com/containers/podman/issues/12615)).
- Fixed a bug where the `podman run` command would fail if the host machine did not have a `/etc/hosts` file ([#12667](https://github.com/containers/podman/issues/12667)).
- Fixed a bug where certain annotations used internally by Podman could be set by images, resulting in `podman inspect` reporting incorrect information ([#12671](https://github.com/containers/podman/issues/12671)).
- Fixed a bug where named volumes would not copy-up after being mounted over an empty directory, then subsequently mounted over a non-empty directory in another container ([#12714](https://github.com/containers/podman/issues/12714)).
- Fixed a bug where the `podman inspect` command on containers was URL-encoding special characters in strings (e.g. healthcheck commands).
- Fixed a bug where the `podman generate kube` command would generate YAML including optional environment variables from secrets and configmaps that are not included ([#12553](https://github.com/containers/podman/issues/12553)).
- Fixed a bug where the `podman pod create` command would ignore the default infra image specified in `containers.conf` ([#12771](https://github.com/containers/podman/issues/12771)).
- Fixed a bug where the `host.containers.internal` entry in `/etc/hosts` was set incorrectly to an inaccessible host IP for `macvlan` networks ([#11351](https://github.com/containers/podman/issues/11351)).
- Fixed a bug where secrets could not be mounted into containers that joined a user namespace (e.g. `--userns=auto`) ([#12779](https://github.com/containers/podman/issues/12779)).
- Fixed a bug where rootless Podman could produce an error about cgroups when containers were created inside existing pods ([#10800](https://github.com/containers/podman/issues/10800)).
- Fixed a bug where Podman could error that a systemd session was not available despite having the cgroup manager set to `cgroupfs` ([#12802](https://github.com/containers/podman/issues/12802)).
- Fixed a bug where the remote Podman client on Windows would ignore environment variables from the `--env` option to `podman create` and `podman run` ([#12056](https://github.com/containers/podman/issues/12056)).
- Fixed a bug where Podman could segfault when an error occurred trying to set up rootless mode.
- Fixed a bug where Podman could segfault when reading an image layer that did not have a creation timestamp set.
- Fixed a bug where, when Podman's storage directories were on an NFS filesystem, Podman would leave some unneeded file descriptors open, causing errors when containers were removed.
- Fixed a bug where, when Podman's storage directories were on an NFS filesystem, cleaning up a container's exec sessions could fail.
- Fixed a bug where Podman commands that operate on a container could give an incorrect error message if given a partial ID that could refer to 2 or more containers ([#12963](https://github.com/containers/podman/issues/12963)).
- Fixed a bug where the `podman stats` command would not show network usage statistics on containers using `slirp4netns` for networking ([#11695](https://github.com/containers/podman/issues/11695)).
- Fixed a bug where the `/dev/shm` mount in the container was not mounted with `nosuid`, `noexec`, and `nodev` mount options.
- Fixed a bug where the `--shm-size` option to `podman create` and `podman run` interpeted human-readable sizes as KB instead of KiB, and GB instead of GiB (such that a kilobyte was interpreted as 1000 bytes, instead of 1024 bytes) ([#13096](https://github.com/containers/podman/issues/13096)).
- Fixed a bug where the `--share=cgroup` option to `podman pod create` controlled whether the pod used a shared Cgroup parent, not whether the Cgroup namespace was shared ([#12765](https://github.com/containers/podman/issues/12765)).
- Fixed a bug where, when a Podman container using the `slirp4netns` network mode was run inside a systemd unit file, systemd could kill the `slirp4netns` process, which is shared between all containers for a given user (thus causing all `slirp4netns`-mode containers for that user to be unable to connect to the internet) ([#13153](https://github.com/containers/podman/issues/13153)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands would not update `/etc/resolv.conf` in the container to add or remove the DNS servers of the networks that were connected or disconnected ([#9603](https://github.com/containers/podman/issues/9603)).

### API
- The Podman remote API version has been bumped to v4.0.0.
- The Compat and Libpod Search endpoints for Images now will never truncate the returned image description. The `noTrunc` query parameter is now ignored as such ([#11894](https://github.com/containers/podman/issues/11894)).
- The Libpod Top endpoints for Containers and Pods now support streaming output using the `stream=true` query parameter ([#12115](https://github.com/containers/podman/issues/12115)).
- The Libpod Create endpoint for Volumes now supports specifying labels for the volume both as `Label` and `Labels` in the provided JSON configuration ([#12102](https://github.com/containers/podman/issues/12102)).
- The Compat Create endpoint for Containers now respects cgroup configuration from `containers.conf` ([#12550](https://github.com/containers/podman/issues/12550)).
- The Compat Create endpoint for Containers now respects user namespace configuration from the `PODMAN_USERNS` environment variable ([#11350](https://github.com/containers/podman/issues/11350)).
- Fixed a bug where the Compat Create endpoint for Containers was ignoring the `HostConfig.StorageOpt` field ([#11016](https://github.com/containers/podman/issues/11016)).
- Fixed a bug where the Compat List endpoint for Containers did not populate the `Mounts` field ([#12734](https://github.com/containers/podman/issues/12734)).
- Fixed a bug where a race condition could cause a crash in the server when the Compat or Libpod Attach endpoints for Containers were invoked ([#12904](https://github.com/containers/podman/issues/12904)).
- Fixed a bug where the Libpod Prune endpoint for Images would return nothing, instead of an empty array, when nothing was pruned.
- Fixed a bug where the Compat List endpoint for Images did not prefix image IDs with `sha256:`.
- Fixed a bug where the Compat Push endpoint for Images would return JSON which did not include the `size` field ([#12468](https://github.com/containers/podman/issues/12468)).
- Fixed a bug where the Compat Load endpoint for Images would refuse to accept input archives that contained more than one image.
- Fixed a bug where the Compat Build endpoint for Images ignored the `quiet` query parameter ([#12566](https://github.com/containers/podman/issues/12566)).
- Fixed a bug where the Compat Build endpoint for Images did not include `aux` JSON (which included the ID of built images) in returned output ([#12063](https://github.com/containers/podman/issues/12063)).
- Fixed a bug where the Compat Build endpoint for Images did not set the correct `Content-Type` in its responses ([#13148](https://github.com/containers/podman/issues/13148)).
- Fixed a bug where the Compat and Libpod List endpoints for Networks would sometimes not return networks created on the server by the Podman CLI after the API server had been started ([#11828](https://github.com/containers/podman/issues/11828)).
- Fixed a bug where the Compat Inspect endpoint for Networks did not include the subnet CIDR in the returned IPv4 and IPv6 addresses.
- Fixed a bug where the Compat Events endpoint did not properly set the Action field of `Died` events for containers to `die` (previously, `died` was used; this was incompatible with Docker's output).
- Fixed a bug where the Compat Info endpoint did not properly populate information on configured registries.
- Fixed a bug where the Compat Events endpoint did not properly set the exit code of the container in the `exitCode` field in `Died` events for containers.
- Fixed a bug where the Compat Events endpoint did not properly populate the `TimeNano` field.
- Numerous small changes have been made to ensure that the API matches its Swagger documentation

### Misc
- The Windows installer MSI distributed through GitHub releases no longer supports 32-bit systems, as Podman is built only for 64-bit machines.
- Updated Buildah to v1.24.0
- Updated the containers/image library to v5.19.0
- Updated the containers/storage library to v1.38.1
- Updated the containers/common library to v0.47.1
- Updated the containers/psgo library to v1.7.2

## 3.4.7
### Security
- This release addresses CVE-2022-1227, where running `podman top` on a container made from a maliciously-crafted image and using a user namespace could allow for code execution in the host context.

## 3.4.6
### Security
- This release addresses CVE-2022-27191, where an attacker could potentially cause crashes in remote Podman by using incorrect SSH ciphers.

## 3.4.5
### Security
- This release addresses CVE-2022-27649, where Podman would set excess inheritable capabilities for processes in containers.

### Bugfixes
- Fixed a bug where the `podman images` command could, under some circumstances, take an excessive amount of time to list images ([#11997](https://github.com/containers/podman/issues/11997)).

### Misc
- Updates the containers/common library to v0.44.5

## 3.4.4
### Bugfixes
- Fixed a bug where the `podman exec` command would, under some circumstances, print a warning message about failing to move `conmon` to the appropriate cgroup ([#12535](https://github.com/containers/podman/issues/12535)).
- Fixed a bug where named volumes created as part of container creation (e.g. `podman run --volume avolume:/a/mountpoint` or similar) would be mounted with incorrect permissions ([#12523](https://github.com/containers/podman/issues/12523)).
- Fixed a bug where the `podman-remote create` and `podman-remote run` commands did not properly handle the `--entrypoint=""` option (to clear the container's entrypoint) ([#12521](https://github.com/containers/podman/issues/12521)).

## 3.4.3
### Security
- This release addresses CVE-2021-4024, where the `podman machine` command opened the `gvproxy` API (used to forward ports to `podman machine` VMs) to the public internet on port 7777.
- This release addresses CVE-2021-41190, where incomplete specification of behavior regarding image manifests could lead to inconsistent decoding on different clients.

### Features
- The `--secret type=mount` option to `podman create` and `podman run` supports a new option, `target=`, which specifies where in the container the secret will be mounted ([#12287](https://github.com/containers/podman/issues/12287)).

### Bugfixes
- Fixed a bug where rootless Podman would occasionally print warning messages about failing to move the pause process to a new cgroup ([#12065](https://github.com/containers/podman/issues/12065)).
- Fixed a bug where the `podman run` and `podman create` commands would, when pulling images, still require TLS even with registries set to Insecure via config file ([#11933](https://github.com/containers/podman/issues/11933)).
- Fixed a bug where the `podman generate systemd` command generated units that depended on `multi-user.target`, which has been removed from some distributions ([#12438](https://github.com/containers/podman/issues/12438)).
- Fixed a bug where Podman could not run containers with images that had `/etc/` as a symlink ([#12189](https://github.com/containers/podman/issues/12189)).
- Fixed a bug where the `podman logs -f` command would, when using the `journald` logs backend, exit immediately if the container had previously been restarted ([#12263](https://github.com/containers/podman/issues/12263)).
- Fixed a bug where, in containers on VMs created by `podman machine`, the `host.containers.internal` name pointed to the VM, not the host system ([#11642](https://github.com/containers/podman/issues/11642)).
- Fixed a bug where containers and pods created by the `podman play kube` command in VMs managed by `podman machine` would not automatically forward ports from the host machine ([#12248](https://github.com/containers/podman/issues/12248)).
- Fixed a bug where `podman machine init` would fail on OS X when GNU Coreutils was installed ([#12329](https://github.com/containers/podman/issues/12329)).
- Fixed a bug where `podman machine start` would exit before SSH on the started VM was accepting connections ([#11532](https://github.com/containers/podman/issues/11532)).
- Fixed a bug where the `podman run` command with signal proxying (`--sig-proxy`) enabled could print an error if it attempted to send a signal to a container that had just exited ([#8086](https://github.com/containers/podman/issues/8086)).
- Fixed a bug where the `podman stats` command would not return correct information for containers running Systemd as PID1 ([#12400](https://github.com/containers/podman/issues/12400)).
- Fixed a bug where the `podman image save` command would fail on OS X when writing the image to STDOUT ([#12402](https://github.com/containers/podman/issues/12402)).
- Fixed a bug where the `podman ps` command did not properly handle PS arguments which contained whitespace ([#12452](https://github.com/containers/podman/issues/12452)).
- Fixed a bug where the `podman-remote wait` command could fail to detect that the container exited and return an error under some circumstances ([#12457](https://github.com/containers/podman/issues/12457)).
- Fixed a bug where the Windows MSI installer for `podman-remote` would break the PATH environment variable by adding an extra `"` ([#11416](https://github.com/containers/podman/issues/11416)).

### API
- Updated the containers/image library to v5.17.0
- The Libpod Play Kube endpoint now also accepts `ConfigMap` YAML as part of its payload, and will use provided any `ConfigMap` to configure provided pods and services.
- Fixed a bug where the Compat Create endpoint for Containers would not always create the container's working directory if it did not exist ([#11842](https://github.com/containers/podman/issues/11842)).
- Fixed a bug where the Compat Create endpoint for Containers returned an incorrect error message with 404 errors when the requested image was not found ([#12315](https://github.com/containers/podman/pull/12315)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the `HostConfig.Mounts` field ([#12419](https://github.com/containers/podman/issues/12419)).
- Fixed a bug where the Compat Archive endpoint for Containers did not properly report errors when the operation failed ([#12420](https://github.com/containers/podman/issues/12420)).
- Fixed a bug where the Compat Build endpoint for Images ignored the `layers` query parameter (for caching intermediate layers from the build) ([#12378](https://github.com/containers/podman/issues/12378)).
- Fixed a bug where the Compat Build endpoint for Images did not report errors in a manner compatible with Docker ([#12392](https://github.com/containers/podman/issues/12392)).
- Fixed a bug where the Compat Build endpoint for Images would fail to build if the context directory was a symlink ([#12409](https://github.com/containers/podman/issues/12409)).
- Fixed a bug where the Compat List endpoint for Images included manifest lists (and not just images) in returned results ([#12453](https://github.com/containers/podman/issues/12453)).

### Misc
- Updated the containers/image library to v5.17.0
- Updated the containers/storage library to v1.37.0
- Podman now builds by default with cgo enabled on OS X, resolving some issues with SSH ([#10737](https://github.com/containers/podman/issues/10737)).

## 3.4.2
### Bugfixes
- Fixed a bug where `podman tag` could not tag manifest lists ([#12046](https://github.com/containers/podman/issues/12046)).
- Fixed a bug where built-in volumes specified by images would not be created correctly under some circumstances.
- Fixed a bug where, when using Podman Machine on OS X, containers in pods did not have working port forwarding from the host ([#12207](https://github.com/containers/podman/issues/12207)).
- Fixed a bug where the `podman network reload` command command on containers using the `slirp4netns` network mode and the `rootlessport` port forwarding driver would make an unnecessary attempt to restart `rootlessport` on containers that did not forward ports.
- Fixed a bug where the `podman generate kube` command would generate YAML including some unnecessary (set to default) fields (e.g. empty SELinux and DNS configuration blocks, and the `privileged` flag when set to false) ([#11995](https://github.com/containers/podman/issues/11995)).
- Fixed a bug where the `podman pod rm` command could, if interrupted at the right moment, leave a reference to an already-removed infra container behind ([#12034](https://github.com/containers/podman/issues/12034)).
- Fixed a bug where the `podman pod rm` command would not remove pods with more than one container if all containers save for the infra container were stopped unless `--force` was specified ([#11713](https://github.com/containers/podman/issues/11713)).
- Fixed a bug where the `--memory` flag to `podman run` and `podman create` did not accept a limit of 0 (which should specify unlimited memory) ([#12002](https://github.com/containers/podman/issues/12002)).
- Fixed a bug where the remote Podman client's `podman build` command could attempt to build a Dockerfile in the working directory of the `podman system service` instance instead of the Dockerfile specified by the user ([#12054](https://github.com/containers/podman/issues/12054)).
- Fixed a bug where the `podman logs --tail` command could function improperly (printing more output than requested) when the `journald` log driver was used.
- Fixed a bug where containers run using the `slirp4netns` network mode with IPv6 enabled would not have IPv6 connectivity until several seconds after they started ([#11062](https://github.com/containers/podman/issues/11062)).
- Fixed a bug where some Podman commands could cause an extra `dbus-daemon` process to be created ([#9727](https://github.com/containers/podman/issues/9727)).
- Fixed a bug where rootless Podman would sometimes print warnings about a failure to move the pause process into a given CGroup ([#12065](https://github.com/containers/podman/issues/12065)).
- Fixed a bug where the `checkpointed` field in `podman inspect` on a container was not set to false after a container was restored.
- Fixed a bug where the `podman system service` command would print overly-verbose logs about request IDs ([#12181](https://github.com/containers/podman/issues/12181)).
- Fixed a bug where Podman could, when creating a new container without a name explicitly specified by the user, sometimes use an auto-generated name already in use by another container if multiple containers were being created in parallel ([#11735](https://github.com/containers/podman/issues/11735)).

## 3.4.1
### Bugfixes
- Fixed a bug where `podman machine init` could, under some circumstances, create invalid machine configurations which could not be started ([#11824](https://github.com/containers/podman/issues/11824)).
- Fixed a bug where the `podman machine list` command would not properly populate some output fields.
- Fixed a bug where `podman machine rm` could leave dangling sockets from the removed machine ([#11393](https://github.com/containers/podman/issues/11393)).
- Fixed a bug where `podman run --pids-limit=-1` was not supported (it now sets the PID limit in the container to unlimited) ([#11782](https://github.com/containers/podman/issues/11782)).
- Fixed a bug where `podman run` and `podman attach` could throw errors about a closed network connection when STDIN was closed by the client ([#11856](https://github.com/containers/podman/issues/11856)).
- Fixed a bug where the `podman stop` command could fail when run on a container that had another `podman stop` command run on it previously.
- Fixed a bug where the `--sync` flag to `podman ps` was nonfunctional.
- Fixed a bug where the Windows and OS X remote clients' `podman stats` command would fail ([#11909](https://github.com/containers/podman/issues/11909)).
- Fixed a bug where the `podman play kube` command did not properly handle environment variables whose values contained an `=` ([#11891](https://github.com/containers/podman/issues/11891)).
- Fixed a bug where the `podman generate kube` command could generate invalid annotations when run on containers with volumes that use SELinux relabelling (`:z` or `:Z`) ([#11929](https://github.com/containers/podman/issues/11929)).
- Fixed a bug where the `podman generate kube` command would generate YAML including some unnecessary (set to default) fields (e.g. user and group, entrypoint, default protocol for forwarded ports) ([#11914](https://github.com/containers/podman/issues/11914), [#11915](https://github.com/containers/podman/issues/11915), and [#11965](https://github.com/containers/podman/issues/11965)).
- Fixed a bug where the `podman generate kube` command could, under some circumstances, generate YAML including an invalid `targetPort` field for forwarded ports ([#11930](https://github.com/containers/podman/issues/11930)).
- Fixed a bug where rootless Podman's `podman info` command could, under some circumstances, not read available CGroup controllers ([#11931](https://github.com/containers/podman/issues/11931)).
- Fixed a bug where `podman container checkpoint --export` would fail to checkpoint any container created with `--log-driver=none` ([#11974](https://github.com/containers/podman/issues/11974)).

### API
- Fixed a bug where the Compat Create endpoint for Containers could panic when no options were passed to a bind mount of tmpfs ([#11961](https://github.com/containers/podman/issues/11961)).

## 3.4.0
### Features
- Pods now support init containers! Init containers are containers which run before the rest of the pod starts. There are two types of init containers: "always", which always run before the pod is started, and "once", which only run the first time the pod starts and are subsequently removed. They can be added using the `podman create` command's `--init-ctr` option.
- Support for init containers has also been added to `podman play kube` and `podman generate kube` - init containers contained in Kubernetes YAML will be created as Podman init containers, and YAML generated by Podman will include any init containers created.
- The `podman play kube` command now supports building images. If the `--build` option is given and a directory with the name of the specified image exists in the current working directory and contains a valid Containerfile or Dockerfile, the image will be built and used for the container.
- The `podman play kube` command now supports a new option, `--teardown`, which removes any pods and containers created by the given Kubernetes YAML.
- The `podman generate kube` command now generates annotations for SELinux mount options on volume (`:z` and `:Z`) that are respected by the `podman play kube` command.
- A new command has been added, `podman pod logs`, to return logs for all containers in a pod at the same time.
- Two new commands have been added, `podman volume export` (to export a volume to a tar file) and `podman volume import`) (to populate a volume from a given tar file).
- The `podman auto-update` command now supports simple rollbacks. If a container fails to start after an automatic update, it will be rolled back to the previous image and restarted again.
- Pods now share their user namespace by default, and the `podman pod create` command now supports the `--userns` option. This allows rootless pods to be created with the `--userns=keep-id` option.
- The `podman pod ps` command now supports a new filter with its `--filter` option, `until`, which returns pods created before a given timestamp.
- The `podman image scp` command has been added. This command allows images to be transferred between different hosts.
- The `podman stats` command supports a new option, `--interval`, to specify the amount of time before the information is refreshed.
- The `podman inspect` command now includes ports exposed (but not published) by containers (e.g. ports from `--expose` when `--publish-all` is not specified).
- The `podman inspect` command now has a new boolean value, `Checkpointed`, which indicates that a container was stopped as a result of a `podman container checkpoint` operation.
- Volumes created by `podman volume create` now support setting quotas when run atop XFS. The `size` and `inode` options allow the maximum size and maximum number of inodes consumed by a volume to be limited.
- The `podman info` command now outputs information on what log drivers, network drivers, and volume plugins are available for use ([#11265](https://github.com/containers/podman/issues/11265)).
- The `podman info` command now outputs the current log driver in use, and the variant and codename of the distribution in use.
- The parameters of the VM created by `podman machine init` (amount of disk space, memory, CPUs) can now be set in `containers.conf`.
- The `podman machine ls` command now shows additional information (CPUs, memory, disk size) about VMs managed by `podman machine`.
- The `podman ps` command now includes healthcheck status in container state for containers that have healthchecks ([#11527](https://github.com/containers/podman/issues/11527)).

### Changes
- The `podman build` command has a new alias, `podman buildx`, to improve compatibility with Docker. We have already added support for many `docker buildx` flags to `podman build` and aim to continue to do so.
- Cases where Podman is run without a user session or a writable temporary files directory will now produce better error messages.
- The default log driver has been changed from `file` to `journald`. The `file` driver did not properly support log rotation, so this should lead to a better experience. If journald is not available on the system, Podman will automatically revert to the `file`.
- Podman no longer depends on `ip` for removing networks ([#11403](https://github.com/containers/podman/issues/11403)).
- The deprecated `--macvlan` flag to `podman network create` now warns when it is used. It will be removed entirely in the Podman 4.0 release.
- The `podman machine start` command now prints a message when the VM is successfully started.
- The `podman stats` command can now be used on containers that are paused.
- The `podman unshare` command will now return the exit code of the command that was run in the user namespace (assuming the command was successfully run).
- Successful healthchecks will no longer add a `healthy` line to the system log to reduce log spam.
- As a temporary workaround for a lack of shortname prompts in the Podman remote client, VMs created by `podman machine` now default to only using the `docker.io` registry.

### Bugfixes
- Fixed a bug where whitespace in the definition of sysctls (particularly default sysctls specified in `containers.conf`) would cause them to be parsed incorrectly.
- Fixed a bug where the Windows remote client improperly validated volume paths ([#10900](https://github.com/containers/podman/issues/10900)).
- Fixed a bug where the first line of logs from a container run with the `journald` log driver could be skipped.
- Fixed a bug where images created by `podman commit` did not include ports exposed by the container.
- Fixed a bug where the `podman auto-update` command would ignore the `io.containers.autoupdate.authfile` label when pulling images ([#11171](https://github.com/containers/podman/issues/11171)).
- Fixed a bug where the `--workdir` option to `podman create` and `podman run` could not be set to a directory where a volume was mounted ([#11352](https://github.com/containers/podman/issues/11352)).
- Fixed a bug where systemd socket-activation did not properly work with systemd-managed Podman containers ([#10443](https://github.com/containers/podman/issues/10443)).
- Fixed a bug where environment variable secrets added to a container were not available to exec sessions launched in the container.
- Fixed a bug where rootless containers could fail to start the `rootlessport` port-forwarding service when `XDG_RUNTIME_DIR` was set to a long path.
- Fixed a bug where arguments to the `--systemd` option to `podman create` and `podman run` were case-sensitive ([#11387](https://github.com/containers/podman/issues/11387)).
- Fixed a bug where the `podman manifest rm` command would also remove images referenced by the manifest, not just the manifest itself ([#11344](https://github.com/containers/podman/issues/11344)).
- Fixed a bug where the Podman remote client on OS X would not function properly if the `TMPDIR` environment variable was not set ([#11418](https://github.com/containers/podman/issues/11418)).
- Fixed a bug where the `/etc/hosts` file was not guaranteed to contain an entry for `localhost` (this is still not guaranteed if `--net=host` is used; such containers will exactly match the host's `/etc/hosts`) ([#11411](https://github.com/containers/podman/issues/11411)).
- Fixed a bug where the `podman machine start` command could print warnings about unsupported CPU features ([#11421](https://github.com/containers/podman/issues/11421)).
- Fixed a bug where the `podman info` command could segfault when accessing cgroup information.
- Fixed a bug where the `podman logs -f` command could hang when a container exited ([#11461](https://github.com/containers/podman/issues/11461)).
- Fixed a bug where the `podman generate systemd` command could not be used on containers that specified a restart policy ([#11438](https://github.com/containers/podman/issues/11438)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build containers if the UID and GID on the client were higher than 65536 ([#11474](https://github.com/containers/podman/issues/11474)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build containers if the context directory was a symlink ([#11732](https://github.com/containers/podman/issues/11732)).
- Fixed a bug where the `--network` flag to `podman play kube` was not properly parsed when a non-bridge network configuration was specified.
- Fixed a bug where the `podman inspect` command could error when the container being inspected was removed as it was being inspected ([#11392](https://github.com/containers/podman/issues/11392)).
- Fixed a bug where the `podman play kube` command ignored the default pod infra image specified in `containers.conf`.
- Fixed a bug where the `--format` option to `podman inspect` was nonfunctional under some circumstances ([#8785](https://github.com/containers/podman/issues/8785)).
- Fixed a bug where the remote Podman client's `podman run` and `podman exec` commands could skip a byte of output every 8192 bytes ([#11496](https://github.com/containers/podman/issues/11496)).
- Fixed a bug where the `podman stats` command would print nonsensical results if the container restarted while it was running ([#11469](https://github.com/containers/podman/issues/11469)).
- Fixed a bug where the remote Podman client would error when STDOUT was redirected on a Windows client ([#11444](https://github.com/containers/podman/issues/11444)).
- Fixed a bug where the `podman run` command could return 0 when the application in the container exited with 125 ([#11540](https://github.com/containers/podman/issues/11540)).
- Fixed a bug where containers with `--restart=always` set using the rootlessport port-forwarding service could not be restarted automatically.
- Fixed a bug where the `--cgroups=split` option to `podman create` and `podman run` was silently discarded if the container was part of a pod.
- Fixed a bug where the `podman container runlabel` command could fail if the image name given included a tag.
- Fixed a bug where Podman could add an extra `127.0.0.1` entry to `/etc/hosts` under some circumstances ([#11596](https://github.com/containers/podman/issues/11596)).
- Fixed a bug where the remote Podman client's `podman untag` command did not properly handle tags including a digest ([#11557](https://github.com/containers/podman/issues/11557)).
- Fixed a bug where the `--format` option to `podman ps` did not properly support the `table` argument for tabular output.
- Fixed a bug where the `--filter` option to `podman ps` did not properly handle filtering by healthcheck status ([#11687](https://github.com/containers/podman/issues/11687)).
- Fixed a bug where the `podman run` and `podman start --attach` commands could race when retrieving the exit code of a container that had already been removed resulting in an error (e.g. by an external `podman rm -f`) ([#11633](https://github.com/containers/podman/issues/11633)).
- Fixed a bug where the `podman generate kube` command would add default environment variables to generated YAML.
- Fixed a bug where the `podman generate kube` command would add the default CMD from the image to generated YAML ([#11672](https://github.com/containers/podman/issues/11672)).
- Fixed a bug where the `podman rm --storage` command could fail to remove containers under some circumstances ([#11207](https://github.com/containers/podman/issues/11207)).
- Fixed a bug where the `podman machine ssh` command could fail when run on Linux ([#11731](https://github.com/containers/podman/issues/11731)).
- Fixed a bug where the `podman stop` command would error when used on a container that was already stopped ([#11740](https://github.com/containers/podman/issues/11740)).
- Fixed a bug where renaming a container in a pod using the `podman rename` command, then removing the pod using `podman pod rm`, could cause Podman to believe the new name of the container was permanently in use, despite the container being removed ([#11750](https://github.com/containers/podman/issues/11750)).

### API
- The Libpod Pull endpoint for Images now has a new query parameter, `quiet`, which (when set to true) suppresses image pull progress reports ([#10612](https://github.com/containers/podman/issues/10612)).
- The Compat Events endpoint now includes several deprecated fields from the Docker v1.21 API for improved compatibility with older clients.
- The Compat List and Inspect endpoints for Images now prefix image IDs with `sha256:` for improved Docker compatibility ([#11623](https://github.com/containers/podman/issues/11623)).
- The Compat Create endpoint for Containers now properly sets defaults for healthcheck-related fields ([#11225](https://github.com/containers/podman/issues/11225)).
- The Compat Create endpoint for Containers now supports volume options provided by the `Mounts` field ([#10831](https://github.com/containers/podman/issues/10831)).
- The Compat List endpoint for Secrets now supports a new query parameter, `filter`, which allows returned results to be filtered.
- The Compat Auth endpoint now returns the correct response code (500 instead of 400) when logging into a registry fails.
- The Version endpoint now includes information about the OCI runtime and Conmon in use ([#11227](https://github.com/containers/podman/issues/11227)).
- Fixed a bug where the X-Registry-Config header was not properly handled, leading to errors when pulling images ([#11235](https://github.com/containers/podman/issues/11235)).
- Fixed a bug where invalid query parameters could cause a null pointer dereference when creating error messages.
- Logging of API requests and responses at trace level has been greatly improved, including the addition of an X-Reference-Id header to correlate requests and responses ([#10053](https://github.com/containers/podman/issues/10053)).

### Misc
- Updated Buildah to v1.23.0
- Updated the containers/storage library to v1.36.0
- Updated the containers/image library to v5.16.0
- Updated the containers/common library to v0.44.0

## 3.3.1
### Bugfixes
- Fixed a bug where unit files created by `podman generate systemd` could not cleanup shut down containers when stopped by `systemctl stop` ([#11304](https://github.com/containers/podman/issues/11304)).
- Fixed a bug where `podman machine` commands would not properly locate the `gvproxy` binary in some circumstances.
- Fixed a bug where containers created as part of a pod using the `--pod-id-file` option would not join the pod's network namespace ([#11303](https://github.com/containers/podman/issues/11303)).
- Fixed a bug where Podman, when using the systemd cgroups driver, could sometimes leak dbus sessions.
- Fixed a bug where the `until` filter to `podman logs` and `podman events` was improperly handled, requiring input to be negated ([#11158](https://github.com/containers/podman/issues/11158)).
- Fixed a bug where rootless containers using CNI networking run on systems using `systemd-resolved` for DNS would fail to start if resolved symlinked `/etc/resolv.conf` to an absolute path ([#11358](https://github.com/containers/podman/issues/11358)).

### API
- A large number of potential file descriptor leaks from improperly closing client connections have been fixed.

## 3.3.0
### Features
- Containers inside VMs created by `podman machine` will now automatically handle port forwarding - containers in `podman machine` VMs that publish ports via `--publish` or `--publish-all` will have these ports not just forwarded on the VM, but also on the host system.
- The `podman play kube` command's `--network` option now accepts advanced network options (e.g. `--network slirp4netns:port_handler=slirp4netns`) ([#10807](https://github.com/containers/podman/issues/10807)).
- The `podman play kube` command now supports Kubernetes liveness probes, which will be created as Podman healthchecks.
- Podman now provides a systemd unit, `podman-restart.service`, which, when enabled, will restart all containers that were started with `--restart=always` after the system reboots.
- Rootless Podman can now be configured to use CNI networking by default by using the `rootless_networking` option in `containers.conf`.
- Images can now be pulled using `image:tag@digest` syntax (e.g. `podman pull fedora:34@sha256:1b0d4ddd99b1a8c8a80e885aafe6034c95f266da44ead992aab388e6aa91611a`) ([#6721](https://github.com/containers/podman/issues/6721)).
- The `podman container checkpoint` and `podman container restore` commands can now be used to checkpoint containers that are in pods, and restore those containers into pods.
- The `podman container restore` command now features a new option, `--publish`, to change the ports that are forwarded to a container that is being restored from an exported checkpoint.
- The `podman container checkpoint` command now features a new option, `--compress`, to specify the compression algorithm that will be used on the generated checkpoint.
- The `podman pull` command can now pull multiple images at once (e.g. `podman pull fedora:34 ubi8:latest` will pull both specified images).
- THe `podman cp` command can now copy files from one container into another directly (e.g. `podman cp containera:/etc/hosts containerb:/etc/`) ([#7370](https://github.com/containers/podman/issues/7370)).
- The `podman cp` command now supports a new option, `--archive`, which controls whether copied files will be chown'd to the UID and GID of the user of the destination container.
- The `podman stats` command now provides two additional metrics: Average CPU, and CPU time.
- The `podman pod create` command supports a new flag, `--pid`, to specify the PID namespace of the pod. If specified, containers that join the pod will automatically share its PID namespace.
- The `podman pod create` command supports a new flag, `--infra-name`, which allows the name of the pod's infra container to be set ([#10794](https://github.com/containers/podman/issues/10794)).
- The `podman auto-update` command has had its output reformatted - it is now much clearer what images were pulled and what containers were updated.
- The `podman auto-update` command now supports a new option, `--dry-run`, which reports what would be updated but does not actually perform the update ([#9949](https://github.com/containers/podman/issues/9949)).
- The `podman build` command now supports a new option, `--secret`, to mount secrets into build containers.
- The `podman manifest remove` command now has a new alias, `podman manifest rm`.
- The `podman login` command now supports a new option, `--verbose`, to print detailed information about where the credentials entered were stored.
- The `podman events` command now supports a new event, `exec_died`, which is produced when an exec session exits, and includes the exit code of the exec session.
- The `podman system connection add` command now supports adding connections that connect using the `tcp://` and `unix://` URL schemes.
- The `podman system connection list` command now supports a new flag, `--format`, to determine how the output is printed.
- The `podman volume prune` and `podman volume ls` commands' `--filter` option now support a new filter, `until`, that matches volumes created before a certain time ([#10579](https://github.com/containers/podman/issues/10579)).
- The `podman ps --filter` option's `network` filter now accepts a new value: `container:`, which matches containers that share a network namespace with a specific container ([#10361](https://github.com/containers/podman/issues/10361)).
- The `podman diff` command can now accept two arguments, allowing two images or two containers to be specified; the diff between the two will be printed ([#10649](https://github.com/containers/podman/issues/10649)).
- Podman can now optionally copy-up content from containers into volumes mounted into those containers earlier (at creation time, instead of at runtime) via the `prepare_on_create` option in `containers.conf` ([#10262](https://github.com/containers/podman/issues/10262)).
- A new option, `--gpus`, has been added to `podman create` and `podman run` as a no-op for better compatibility with Docker. If the nvidia-container-runtime package is installed, GPUs should be automatically added to containers without using the flag.
- If an invalid subcommand is provided, similar commands to try will now be suggested in the error message.

### Changes
- The `podman system reset` command now removes non-Podman (e.g. Buildah and CRI-O) containers as well.
- The new port forwarding offered by `podman machine` requires [gvproxy](https://github.com/containers/gvisor-tap-vsock) in order to function.
- Podman will now automatically create the default CNI network if it does not exist, for both root and rootless users. This will only be done once per user - if the network is subsequently removed, it will not be recreated.
- The `install.cni` makefile option has been removed. It is no longer required to distribute the default `87-podman.conflist` CNI configuration file, as Podman will now automatically create it.
- The `--root` option to Podman will not automatically clear all default storage options when set. Storage options can be set manually using `--storage-opt` ([#10393](https://github.com/containers/podman/issues/10393)).
- The output of `podman system connection list` is now deterministic, with connections being sorted alpabetically by their name.
- The auto-update service (`podman-auto-update.service`) has had its default timer adjusted so it now starts at a random time up to 15 minutes after midnight, to help prevent system congestion from numerous daily services run at once.
- Systemd unit files generated by `podman generate systemd` now depend on `network-online.target` by default ([#10655](https://github.com/containers/podman/issues/10655)).
- Systemd unit files generated by `podman generate systemd` now use `Type=notify` by default, instead of using PID files.
- The `podman info` command's logic for detecting package versions on Gentoo has been improved, and should be significantly faster.

### Bugfixes
- Fixed a bug where the `podman play kube` command did not perform SELinux relabelling of volumes specified with a `mountPath` that included the `:z` or `:Z` options ([#9371](https://github.com/containers/podman/issues/9371)).
- Fixed a bug where the `podman play kube` command would ignore the `USER` and `EXPOSE` directives in images ([#9609](https://github.com/containers/podman/issues/9609)).
- Fixed a bug where the `podman play kube` command would only accept lowercase pull policies.
- Fixed a bug where named volumes mounted into containers with the `:z` or `:Z` options were not appropriately relabelled for access from the container ([#10273](https://github.com/containers/podman/issues/10273)).
- Fixed a bug where the `podman logs -f` command, with the `journald` log driver, could sometimes fail to pick up the last line of output from a container ([#10323](https://github.com/containers/podman/issues/10323)).
- Fixed a bug where running `podman rm` on a container created with the `--rm` option would occasionally emit an error message saying the container failed to be removed, when it was successfully removed.
- Fixed a bug where starting a Podman container would segfault if the `LISTEN_PID` and `LISTEN_FDS` environment variables were set, but `LISTEN_FDNAMES` was not ([#10435](https://github.com/containers/podman/issues/10435)).
- Fixed a bug where exec sessions in containers were sometimes not cleaned up when run without `-d` and when the associated `podman exec` process was killed before completion.
- Fixed a bug where `podman system service` could, when run in a systemd unit file with sdnotify in use, drop some connections when it was starting up.
- Fixed a bug where containers run using the REST API using the `slirp4netns` network mode would leave zombie processes that were not cleaned up until `podman system service` exited ([#9777](https://github.com/containers/podman/issues/9777)).
- Fixed a bug where the `podman system service` command would leave zombie processes after its initial launch that were not cleaned up until it exited ([#10575](https://github.com/containers/podman/issues/10575)).
- Fixed a bug where VMs created by `podman machine` could not be started after the host system restarted ([#10824](https://github.com/containers/podman/issues/10824)).
- Fixed a bug where the `podman pod ps` command would not show headers for optional information (e.g. container names when the `--ctr-names` option was given).
- Fixed a bug where the remote Podman client's `podman create` and `podman run` commands would ignore timezone configuration from the server's `containers.conf` file ([#11124](https://github.com/containers/podman/issues/11124)).
- Fixed a bug where the remote Podman client's `podman build` command would only respect `.containerignore` and not `.dockerignore` files (when both are present, `.containerignore` will be preferred) ([#10907](https://github.com/containers/podman/issues/10907)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to send the Dockerfile being built to the server when it was excluded by the `.dockerignore` file, resulting in an error ([#9867](https://github.com/containers/podman/issues/9867)).
- Fixed a bug where the remote Podman client's `podman build` command could unexpectedly stop streaming the output of the build ([#10154](https://github.com/containers/podman/issues/10154)).
- Fixed a bug where the remote Podman client's `podman build` command would fail to build when run on Windows ([#11259](https://github.com/containers/podman/issues/11259)).
- Fixed a bug where the `podman manifest create` command accepted at most two arguments (an arbitrary number of images are allowed as arguments, which will be added to the manifest).
- Fixed a bug where named volumes would not be properly chowned to the UID and GID of the directory they were mounted over when first mounted into a container ([#10776](https://github.com/containers/podman/issues/10776)).
- Fixed a bug where named volumes created using a volume plugin would be removed from Podman, even if the plugin reported a failure to remove the volume ([#11214](https://github.com/containers/podman/issues/11214)).
- Fixed a bug where the remote Podman client's `podman exec -i` command would hang when input was provided via shell redirection (e.g. `podman --remote exec -i foo cat <<<"hello"`) ([#7360](https://github.com/containers/podman/issues/7360)).
- Fixed a bug where containers created with `--rm` were not immediately removed after being started by `podman start` if they failed to start ([#10935](https://github.com/containers/podman/issues/10935)).
- Fixed a bug where the `--storage-opt` flag to `podman create` and `podman run` was nonfunctional ([#10264](https://github.com/containers/podman/issues/10264)).
- Fixed a bug where the `--device-cgroup-rule` option to `podman create` and `podman run` was nonfunctional ([#10302](https://github.com/containers/podman/issues/10302)).
- Fixed a bug where the `--tls-verify` option to `podman manifest push` was nonfunctional.
- Fixed a bug where the `podman import` command could, in some circumstances, produce empty images ([#10994](https://github.com/containers/podman/issues/10994)).
- Fixed a bug where images pulled using the `docker-daemon:` transport had the wrong registry (`localhost` instead of `docker.io/library`) ([#10998](https://github.com/containers/podman/issues/10998)).
- Fixed a bug where operations that pruned images (`podman image prune` and `podman system prune`) would prune untagged images with children ([#10832](https://github.com/containers/podman/issues/10832)).
- Fixed a bug where dual-stack networks created by `podman network create` did not properly auto-assign an IPv4 subnet when one was not explicitly specified ([#11032](https://github.com/containers/podman/issues/11032)).
- Fixed a bug where port forwarding using the `rootlessport` port forwarder would break when a network was disconnected and then reconnected ([#10052](https://github.com/containers/podman/issues/10052)).
- Fixed a bug where Podman would ignore user-specified SELinux policies for containers using the Kata OCI runtime, or containers using systemd as PID 1 ([#11100](https://github.com/containers/podman/issues/11100)).
- Fixed a bug where Podman containers created using `--net=host` would add an entry to `/etc/hosts` for the container's hostname pointing to `127.0.1.1` ([#10319](https://github.com/containers/podman/issues/10319)).
- Fixed a bug where the `podman unpause --all` command would throw an error for every container that was not paused ([#11098](https://github.com/containers/podman/issues/11098)).
- Fixed a bug where timestamps for the `since` and `until` filters using Unix timestamps with a nanoseconds portion could not be parsed ([#11131](https://github.com/containers/podman/issues/11131)).
- Fixed a bug where the `podman info` command would sometimes print the wrong path for the `slirp4netns` binary.
- Fixed a bug where rootless Podman containers joined to a CNI network would not have functional DNS when the host used systemd-resolved without the resolved stub resolver being enabled ([#11222](https://github.com/containers/podman/issues/11222)).
- Fixed a bug where `podman network connect` and `podman network disconnect` of rootless containers could sometimes break port forwarding to the container ([#11248](https://github.com/containers/podman/issues/11248)).
- Fixed a bug where joining a container to a CNI network by ID and adding network aliases to this network would cause the container to fail to start ([#11285](https://github.com/containers/podman/issues/11285)).

### API
- Fixed a bug where the Compat List endpoint for Containers included healthcheck information for all containers, even those that did not have a configured healthcheck.
- Fixed a bug where the Compat Create endpoint for Containers would fail to create containers with the `NetworkMode` parameter set to `default` ([#10569](https://github.com/containers/podman/issues/10569)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle healthcheck commands ([#10617](https://github.com/containers/podman/issues/10617)).
- Fixed a bug where the Compat Wait endpoint for Containers would always send an empty string error message when no error occurred.
- Fixed a bug where the Libpod Stats endpoint for Containers would not error when run on rootless containers on cgroups v1 systems (nonsensical results would be returned, as this configuration cannot be supportable).
- Fixed a bug where the Compat List endpoint for Images omitted the `ContainerConfig` field ([#10795](https://github.com/containers/podman/issues/10795)).
- Fixed a bug where the Compat Build endpoint for Images was too strict when validating the `Content-Type` header, rejecting content that Docker would have accepted ([#11022](https://github.com/containers/podman/issues/11012)).
- Fixed a bug where the Compat Pull endpoint for Images could fail, but return a 200 status code, if an image name that could not be parsed was provided.
- Fixed a bug where the Compat Pull endpoint for Images would continue to pull images after the client disconnected.
- Fixed a bug where the Compat List endpoint for Networks would fail for non-bridge (e.g. macvlan) networks ([#10266](https://github.com/containers/podman/issues/10266)).
- Fixed a bug where the Libpod List endpoint for Networks would return nil, instead of an empty list, when no networks were present ([#10495](https://github.com/containers/podman/issues/10495)).
- The Compat and Libpod Logs endpoints for Containers now support the `until` query parameter ([#10859](https://github.com/containers/podman/issues/10859)).
- The Compat Import endpoint for Images now supports the `platform`, `message`, and `repo` query parameters.
- The Compat Pull endpoint for Images now supports the `platform` query parameter.

### Misc
- Updated Buildah to v1.22.3
- Updated the containers/storage library to v1.34.1
- Updated the containers/image library to v5.15.2
- Updated the containers/common library to v0.42.1

## 3.2.3
### Security
- This release addresses CVE-2021-3602, an issue with the `podman build` command with the `--isolation chroot` flag that results in environment variables from the host leaking into build containers.

### Bugfixes
- Fixed a bug where events related to images could occur before the relevant operation had completed (e.g. an image pull event could be written before the pull was finished) ([#10812](https://github.com/containers/podman/issues/10812)).
- Fixed a bug where `podman save` would refuse to save images with an architecture different from that of the host ([#10835](https://github.com/containers/podman/issues/10835)).
- Fixed a bug where the `podman import` command did not correctly handle images without tags ([#10854](https://github.com/containers/podman/issues/10854)).
- Fixed a bug where Podman's journald events backend would fail and prevent Podman from running when run on a host with systemd as PID1 but in an environment (e.g. a container) without systemd ([#10863](https://github.com/containers/podman/issues/10863)).
- Fixed a bug where containers using rootless CNI networking would fail to start when the `dnsname` CNI plugin was in use and the host system's `/etc/resolv.conf` was a symlink ([#10855](https://github.com/containers/podman/issues/10855) and [#10929](https://github.com/containers/podman/issues/10929)).
- Fixed a bug where containers using rootless CNI networking could fail to start due to a race in rootless CNI initialization ([#10930](https://github.com/containers/podman/issues/10930)).

### Misc
- Updated Buildah to v1.21.3
- Updated the containers/common library to v0.38.16

## 3.2.2
### Changes
- Podman's handling of the Architecture field of images has been relaxed. Since 3.2.0, Podman required that the architecture of the image match the architecture of the system to run containers based on an image, but images often incorrectly report architecture, causing Podman to reject valid images ([#10648](https://github.com/containers/podman/issues/10648) and [#10682](https://github.com/containers/podman/issues/10682)).
- Podman no longer uses inotify to monitor for changes to CNI configurations. This removes potential issues where Podman cannot be run because a user has exhausted their available inotify sessions ([#10686](https://github.com/containers/podman/issues/10686)).

### Bugfixes
- Fixed a bug where the `podman cp` would, when given a directory as its source and a target that existed and was a file, copy the contents of the directory into the parent directory of the file; this now results in an error.
- Fixed a bug where the `podman logs` command would, when following a running container's logs, not include the last line of output from the container when it exited when the `k8s-file` driver was in use ([#10675](https://github.com/containers/podman/issues/10675)).
- Fixed a bug where Podman would fail to run containers if `systemd-resolved` was incorrectly detected as the system's DNS server ([#10733](https://github.com/containers/podman/issues/10733)).
- Fixed a bug where the `podman exec -t` command would only resize the exec session's TTY after the session started, leading to a race condition where the terminal would initially not have a size set ([#10560](https://github.com/containers/podman/issues/10560)).
- Fixed a bug where Podman containers using the `slirp4netns` network mode would add an incorrect entry to `/etc/hosts` pointing the container's hostname to the wrong IP address.
- Fixed a bug where Podman would create volumes specified by images with incorrect permissions ([#10188](https://github.com/containers/podman/issues/10188) and [#10606](https://github.com/containers/podman/issues/10606)).
- Fixed a bug where Podman would not respect the `uid` and `gid` options to `podman volume create -o` ([#10620](https://github.com/containers/podman/issues/10620)).
- Fixed a bug where the `podman run` command could panic when parsing the system's cgroup configuration ([#10666](https://github.com/containers/podman/issues/10666)).
- Fixed a bug where the remote Podman client's `podman build -f - ...` command did not read a Containerfile from STDIN ([#10621](https://github.com/containers/podman/issues/10621)).
- Fixed a bug where the `podman container restore --import` command would fail to restore checkpoints created from privileged containers ([#10615](https://github.com/containers/podman/issues/10615)).
- Fixed a bug where Podman was not respecting the `TMPDIR` environment variable when pulling images ([#10698](https://github.com/containers/podman/issues/10698)).
- Fixed a bug where a number of Podman commands did not properly support using Go templates as an argument to the `--format` option.

### API
- Fixed a bug where the Compat Inspect endpoint for Containers did not include information on container healthchecks ([#10457](https://github.com/containers/podman/issues/10457)).
- Fixed a bug where the Libpod and Compat Build endpoints for Images did not properly handle the `devices` query parameter ([#10614](https://github.com/containers/podman/issues/10614)).

### Misc
- Fixed a bug where the Makefile's `make podman-remote-static` target to build a statically-linked `podman-remote` binary was instead producing dynamic binaries ([#10656](https://github.com/containers/podman/issues/10656)).
- Updated the containers/common library to v0.38.11

## 3.2.1
### Changes
- Podman now allows corrupt images (e.g. from restarting the system during an image pull) to be replaced by a `podman pull` of the same image (instead of requiring they be removed first, then re-pulled).

### Bugfixes
- Fixed a bug where Podman would fail to start containers if a Seccomp profile was not available at `/usr/share/containers/seccomp.json` ([#10556](https://github.com/containers/podman/issues/10556)).
- Fixed a bug where the `podman machine start` command failed on OS X machines with the AMD64 architecture and certain QEMU versions ([#10555](https://github.com/containers/podman/issues/10555)).
- Fixed a bug where Podman would always use the slow path for joining the rootless user namespace.
- Fixed a bug where the `podman stats` command would fail on Cgroups v1 systems when run on a container running systemd ([#10602](https://github.com/containers/podman/issues/10602)).
- Fixed a bug where pre-checkpoint support for `podman container checkpoint` did not function correctly.
- Fixed a bug where the remote Podman client's `podman build` command did not properly handle the `-f` option ([#9871](https://github.com/containers/podman/issues/9871)).
- Fixed a bug where the remote Podman client's `podman run` command would sometimes not resize the container's terminal before execution began ([#9859](https://github.com/containers/podman/issues/9859)).
- Fixed a bug where the `--filter` option to the `podman image prune` command was nonfunctional.
- Fixed a bug where the `podman logs -f` command would exit before all output for a container was printed when the `k8s-file` log driver was in use ([#10596](https://github.com/containers/podman/issues/10596)).
- Fixed a bug where Podman would not correctly detect that systemd-resolved was in use on the host and adjust DNS servers in the container appropriately under some circumstances ([#10570](https://github.com/containers/podman/issues/10570)).
- Fixed a bug where the `podman network connect` and `podman network disconnect` commands acted improperly when containers were in the Created state, marking the changes as done but not actually performing them.

### API
- Fixed a bug where the Compat and Libpod Prune endpoints for Networks returned null, instead of an empty array, when nothing was pruned.
- Fixed a bug where the Create API for Images would continue to pull images even if a client closed the connection mid-pull ([#7558](https://github.com/containers/podman/issues/7558)).
- Fixed a bug where the Events API did not include some information (including labels) when sending events.
- Fixed a bug where the Events API would, when streaming was not requested, send at most one event ([#10529](https://github.com/containers/podman/issues/10529)).

### Misc
- Updated the containers/common library to v0.38.9

## 3.2.0
### Features
- Docker Compose is now supported with rootless Podman ([#9169](https://github.com/containers/podman/issues/9169)).
- The `podman network connect`, `podman network disconnect`, and `podman network reload` commands have been enabled for rootless Podman.
- An experimental new set of commands, `podman machine`, was added to assist in managing virtual machines containing a Podman server. These are intended for easing the use of Podman on OS X by handling the creation of a Linux VM for running Podman.
- The `podman generate kube` command can now be run on Podman named volumes (generating `PersistentVolumeClaim` YAML), in addition to pods and containers.
- The `podman play kube` command now supports two new options, `--ip` and `--mac`, to set static IPs and MAC addresses for created pods ([#8442](https://github.com/containers/podman/issues/8442) and [#9731](https://github.com/containers/podman/issues/9731)).
- The `podman play kube` command's support for `PersistentVolumeClaim` YAML has been greatly improved.
- The `podman generate kube` command now preserves the label used by `podman auto-update` to identify containers to update as a Kubernetes annotation, and the `podman play kube` command will convert this annotation back into a label. This allows `podman auto-update` to be used with containers created by `podman play kube`.
- The `podman play kube` command now supports Kubernetes `secretRef` YAML (using the secrets support from `podman secret`) for environment variables.
- Secrets can now be added to containers as environment variables using the `type=env` option to the `--secret` flag to `podman create` and `podman run`.
- The `podman start` command now supports the `--all` option, allowing all containers to be started simultaneously with a single command. The `--filter` option has also been added to filter which containers to start when `--all` is used.
- Filtering containers with the `--filter` option to `podman ps` and `podman start` now supports a new filter, `restart-policy`, to filter containers based on their restart policy.
- The `--group-add` option to rootless `podman run` and `podman create` now accepts a new value, `keep-groups`, which instructs Podman to retain the supplemental groups of the user running Podman in the created container. This is only supported with the `crun` OCI runtime.
- The `podman run` and `podman create` commands now support a new option, `--timeout`. This sets a maximum time the container is allowed to run, after which it is killed ([#6412](https://github.com/containers/podman/issues/6412)).
- The `podman run` and `podman create` commands now support a new option, `--pidfile`. This will create a file when the container is started containing the PID of the first process in the container.
- The `podman run` and `podman create` commands now support a new option, `--requires`. The `--requires` option adds dependency containers - containers that must be running before the current container. Commands like `podman start` will automatically start the requirements of a container before starting the container itself.
- Auto-updating containers can now be done with locally-built images, not just images hosted on a registry, by creating containers with the `io.containers.autoupdate` label set to `local`.
- Podman now supports the [Container Device Interface](https://tags.cncf.io/container-device-interface) (CDI) standard.
- Podman now adds an entry to `/etc/hosts`, `host.containers.internal`, pointing to the current gateway (which, for root containers, is usually a bridge interface on the host system) ([#5651](https://github.com/containers/podman/issues/5651)).
- The `podman ps`, `podman pod ps`, `podman network list`, `podman secret list`, and `podman volume list` commands now support a `--noheading` option, which will cause Podman to omit the heading line including column names.
- The `podman unshare` command now supports a new flag, `--rootless-cni`, to join the rootless network namespace. This allows commands to be run in the same network environment as rootless containers with CNI networking.
- The `--security-opt unmask=` option to `podman run` and `podman create` now supports glob operations to unmask a group of paths at once (e.g. `podman run --security-opt unmask=/proc/* ...` will unmask all paths in `/proc` in the container).
- The `podman network prune` command now supports a `--filter` option to filter which networks will be pruned.

### Changes
- The change in Podman 3.1.2 where the `:z` and `:Z` mount options for volumes were ignored for privileged containers has been reverted after discussion in [#10209](https://github.com/containers/podman/issues/10209).
- Podman's rootless CNI functionality no longer requires a sidecar container! The removal of the requirement for the `rootless-cni-infra` container means that rootless CNI is now usable on all architectures, not just AMD64, and no longer requires pulling an image ([#8709](https://github.com/containers/podman/issues/8709)).
- The Image handling code used by Podman has seen a major rewrite to improve code sharing with our other projects, Buildah and CRI-O. This should result in fewer bugs and performance gains in the long term. Work on this is still ongoing.
- The `podman auto-update` command now prunes previous versions of images after updating if they are unused, to prevent disk exhaustion after repeated updates ([#10190](https://github.com/containers/podman/issues/10190)).
- The `podman play kube` now treats environment variables configured as references to a `ConfigMap` as mandatory unless the `optional` parameter was set; this better matches the behavior of Kubernetes.
- Podman now supports the `--context=default` flag from Docker as a no-op for compatibility purposes.
- When Podman is run as root, but without `CAP_SYS_ADMIN` being available, it will run in a user namespace using the same code as rootless Podman (instead of failing outright).
- The `podman info` command now includes the path of the Seccomp profile Podman is using, available cgroup controllers, and whether Podman is connected to a remote service or running containers locally.
- Containers created with the `--rm` option now automatically use the `volatile` storage flag when available for their root filesystems, causing them not to write changes to disk as often as they will be removed at completion anyways. This should result in improved performance.
- The `podman generate systemd --new` command will now include environment variables referenced by the container in generated unit files if the value would be looked up from the system environment.
- Podman now requires that Conmon v2.0.24 be available.

### Bugfixes
- Fixed a bug where the remote Podman client's `podman build` command did not support the `--arch`, `--platform`, and `--os`, options.
- Fixed a bug where the remote Podman client's `podman build` command ignored the `--rm=false` option ([#9869](https://github.com/containers/podman/issues/9869)).
- Fixed a bug where the remote Podman client's `podman build --iidfile` command could include extra output (in addition to just the image ID) in the image ID file written ([#10233](https://github.com/containers/podman/issues/10233)).
- Fixed a bug where the remote Podman client's `podman build` command did not preserve hardlinks when moving files into the container via `COPY` instructions ([#9893](https://github.com/containers/podman/issues/9893)).
- Fixed a bug where the `podman generate systemd --new` command could generate extra `--iidfile` arguments if the container was already created with one.
- Fixed a bug where the `podman generate systemd --new` command would generate unit files that did not include `RequiresMountsFor` lines ([#10493](https://github.com/containers/podman/issues/10493)).
- Fixed a bug where the `podman generate kube` command produced incorrect YAML for containers which bind-mounted both `/` and `/root` from the host system into the container ([#9764](https://github.com/containers/podman/issues/9764)).
- Fixed a bug where pods created by `podman play kube` from YAML that specified `ShareProcessNamespace` would only share the PID namespace (and not also the UTS, Network, and IPC namespaces) ([#9128](https://github.com/containers/podman/issues/9128)).
- Fixed a bug where the `podman network reload` command could generate spurious error messages when `iptables-nft` was in use.
- Fixed a bug where rootless Podman could fail to attach to containers when the user running Podman had a large UID.
- Fixed a bug where the `podman ps` command could fail with a `no such container` error due to a race condition with container removal ([#10120](https://github.com/containers/podman/issues/10120)).
- Fixed a bug where containers using the `slirp4netns` network mode and setting a custom `slirp4netns` subnet while using the `rootlesskit` port forwarder would not be able to forward ports ([#9828](https://github.com/containers/podman/issues/9828)).
- Fixed a bug where the `--filter ancestor=` option to `podman ps` did not require an exact match of the image name/ID to include a container in its results.
- Fixed a bug where the `--filter until=` option to `podman image prune` would prune images created after the specified time (instead of before).
- Fixed a bug where setting a custom Seccomp profile via the `seccomp_profile` option in `containers.conf` had no effect, and the default profile was used instead.
- Fixed a bug where the `--cgroup-parent` option to `podman create` and `podman run` was ignored in rootless Podman on cgroups v2 systems with the `cgroupfs` cgroup manager ([#10173](https://github.com/containers/podman/issues/10173)).
- Fixed a bug where the `IMAGE` and `NAME` variables in `podman container runlabel` were not being correctly substituted ([#10192](https://github.com/containers/podman/issues/10192)).
- Fixed a bug where Podman could freeze when creating containers with a specific combination of volumes and working directory ([#10216](https://github.com/containers/podman/issues/10216)).
- Fixed a bug where rootless Podman containers restarted by restart policy (e.g. containers created with `--restart=always`) would lose networking after being restarted ([#8047](https://github.com/containers/podman/issues/8047)).
- Fixed a bug where the `podman cp` command could not copy files into containers created with the `--pid=host` flag ([#9985](https://github.com/containers/podman/issues/9985)).
- Fixed a bug where filters to the `podman events` command could not be specified twice (if a filter is specified more than once, it will match if any of the given values match - logical or) ([#10507](https://github.com/containers/podman/issues/10507)).
- Fixed a bug where Podman would include IPv6 nameservers in `resolv.conf` in containers without IPv6 connectivity ([#10158](https://github.com/containers/podman/issues/10158)).
- Fixed a bug where containers could not be created with static IP addresses when connecting to a network using the `macvlan` driver ([#10283](https://github.com/containers/podman/issues/10283)).

### API
- Fixed a bug where the Compat Create endpoint for Containers did not allow advanced network options to be set ([#10110](https://github.com/containers/podman/issues/10110)).
- Fixed a bug where the Compat Create endpoint for Containers ignored static IP information provided in the `IPAMConfig` block ([#10245](https://github.com/containers/podman/issues/10245)).
- Fixed a bug where the Compat Inspect endpoint for Containers returned null (instead of an empty list) for Networks when the container was not joined to a CNI network ([#9837](https://github.com/containers/podman/issues/9837)).
- Fixed a bug where the Compat Wait endpoint for Containers could miss containers exiting if they were immediately restarted.
- Fixed a bug where the Compat Create endpoint for Volumes required that the user provide a name for the new volume ([#9803](https://github.com/containers/podman/issues/9803)).
- Fixed a bug where the Libpod Info handler would sometimes not return the correct path to the Podman API socket.
- Fixed a bug where the Compat Events handler used the wrong name for container exited events (`died` instead of `die`) ([#10168](https://github.com/containers/podman/issues/10168)).
- Fixed a bug where the Compat Push endpoint for Images could leak goroutines if the remote end closed the connection prematurely.

### Misc
- Updated Buildah to v1.21.0
- Updated the containers/common library to v0.38.5
- Updated the containers/storage library to v1.31.3

## 3.1.2
### Bugfixes
- The Compat Export endpoint for Images now supports exporting multiple images at the same time to a single archive.
- Fixed a bug where images with empty layers were stored incorrectly, causing them to be unable to be pushed or saved.
- Fixed a bug where the `podman rmi` command could fail to remove corrupt images from storage.
- Fixed a bug where the remote Podman client's `podman save` command did not support the `oci-dir` and `docker-dir` formats ([#9742](https://github.com/containers/podman/issues/9742)).
- Fixed a bug where volume mounts from `podman play kube` created with a trailing `/` in the container path were were not properly superseding named volumes from the image ([#9618](https://github.com/containers/podman/issues/9618)).
- Fixed a bug where Podman could fail to build on 32-bit architectures.

### Misc
- Updated the containers/image library to v5.11.1

## 3.1.1
### Changes
- Podman now recognizes `trace` as a valid argument to the `--log-level` command. Trace logging is now the most verbose level of logging available.
- The `:z` and `:Z` options for volume mounts are now ignored when the container is privileged or is run with SELinux isolation disabled (`--security-opt label=disable`). This matches better matches Docker's behavior in this case.

### Bugfixes
- Fixed a bug where pruning images with the `podman image prune` or `podman system prune` commands could cause Podman to panic.
- Fixed a bug where the `podman save` command did not properly error when the `--compress` flag was used with incompatible format types.
- Fixed a bug where the `--security-opt` and `--ulimit` options to the remote Podman client's `podman build` command were nonfunctional.
- Fixed a bug where the `--log-rusage` option to the remote Podman client's `podman build` command was nonfunctional ([#9489](https://github.com/containers/podman/issues/9889)).
- Fixed a bug where the `podman build` command could, in some circumstances, use the wrong OCI runtime ([#9459](https://github.com/containers/podman/issues/9459)).
- Fixed a bug where the remote Podman client's `podman build` command could return 0 despite failing ([#10029](https://github.com/containers/podman/issues/10029)).
- Fixed a bug where the `podman container runlabel` command did not properly expand the `IMAGE` and `NAME` variables in the label ([#9405](https://github.com/containers/podman/issues/9405)).
- Fixed a bug where poststop OCI hooks would be executed twice on containers started with the `--rm` argument ([#9983](https://github.com/containers/podman/issues/9983)).
- Fixed a bug where rootless Podman could fail to launch containers on cgroups v2 systems when the `cgroupfs` cgroup manager was in use.
- Fixed a bug where the `podman stats` command could error when statistics tracked exceeded the maximum size of a 32-bit signed integer ([#9979](https://github.com/containers/podman/issues/9979)).
- Fixed a bug where rootless Podman containers run with `--userns=keepid` (without a `--user` flag in addition) would grant exec sessions run in them too many capabilities ([#9919](https://github.com/containers/podman/issues/9919)).
- Fixed a bug where the `--authfile` option to `podman build` did not validate that the path given existed ([#9572](https://github.com/containers/podman/issues/9572)).
- Fixed a bug where the `--storage-opt` option to Podman was appending to, instead of overriding (as is documented), the default storage options.
- Fixed a bug where the `podman system service` connection did not function properly when run in a socket-activated systemd unit file as a non-root user.
- Fixed a bug where the `--network` option to the `podman play kube` command of the remote Podman client was being ignored ([#9698](https://github.com/containers/podman/issues/9698)).
- Fixed a bug where the `--log-driver` option to the `podman play kube` command was nonfunctional ([#10015](https://github.com/containers/podman/issues/10015)).

### API
- Fixed a bug where the Libpod Create endpoint for Manifests did not properly validate the image the manifest was being created with.
- Fixed a bug where the Libpod DF endpoint could, in error cases, append an extra null to the JSON response, causing decode errors.
- Fixed a bug where the Libpod and Compat Top endpoint for Containers would return process names that included extra whitespace.
- Fixed a bug where the Compat Prune endpoint for Containers accepted too many types of filter.

### Misc
- Updated Buildah to v1.20.1
- Updated the containers/storage library to v1.29.0
- Updated the containers/image library to v5.11.0
- Updated the containers/common library to v0.36.0

## 3.1.0
### Features
- A set of new commands has been added to manage secrets! The `podman secret create`, `podman secret inspect`, `podman secret ls` and `podman secret rm` commands have been added to handle secrets, along with the `--secret` option to `podman run` and `podman create` to add secrets to containers. The initial driver for secrets does not support encryption - this will be added in a future release.
- A new command to prune networks, `podman network prune`, has been added ([#8673](https://github.com/containers/podman/issues/8673)).
- The `-v` option to `podman run` and `podman create` now supports a new volume option, `:U`, to chown the volume's source directory on the host to match the UID and GID of the container and prevent permissions issues ([#7778](https://github.com/containers/podman/issues/7778)).
- Three new commands, `podman network exists`, `podman volume exists`, and `podman manifest exists`, have been added to check for the existence of networks, volumes, and manifest lists.
- The `podman cp` command can now copy files into directories mounted as `tmpfs` in a running container.
- The `podman volume prune` command will now list volumes that will be pruned when prompting the user whether to continue and perform the prune ([#8913](https://github.com/containers/podman/issues/8913)).
- The Podman remote client's `podman build` command now supports the `--disable-compression`, `--excludes`, and `--jobs` options.
- The Podman remote client's `podman push` command now supports the `--format` option.
- The Podman remote client's `podman rm` command now supports the `--all` and `--ignore` options.
- The Podman remote client's `podman search` command now supports the `--no-trunc` and `--list-tags` options.
- The `podman play kube` command can now read in Kubernetes YAML from `STDIN` when `-` is specified as file name (`podman play kube -`), allowing input to be piped into the command for scripting ([#8996](https://github.com/containers/podman/issues/8996)).
- The `podman generate systemd` command now supports a `--no-header` option, which disables creation of the header comment automatically added by Podman to generated unit files.
- The `podman generate kube` command can now generate `PersistentVolumeClaim` YAML for Podman named volumes ([#5788](https://github.com/containers/podman/issues/5788)).
- The `podman generate kube` command can now generate YAML files containing multiple resources (pods or deployments) ([#9129](https://github.com/containers/podman/issues/9129)).

### Security
- This release resolves CVE-2021-20291, a deadlock vulnerability in the storage library caused by pulling a specially-crafted container image.

### Changes
- The Podman remote client's `podman build` command no longer allows the `-v` flag to be used. Volumes are not yet supported with remote Podman when the client and service are on different machines.
- The `podman kill` and `podman stop` commands now print the name given by the user for each container, instead of the full ID.
- When the `--security-opt unmask=ALL` or `--security-opt unmask=/sys/fs/cgroup` options to `podman create` or `podman run` are given, Podman will mount cgroups into the container as read-write, instead of read-only ([#8441](https://github.com/containers/podman/issues/8441)).
- The `podman rmi` command has been changed to better handle cases where an image is incomplete or corrupted, which can be caused by interrupted image pulls.
- The `podman rename` command has been improved to be more atomic, eliminating many race conditions that could potentially render a renamed container unusable.
- Detection of which OCI runtimes run using virtual machines and thus require custom SELinux labelling has been improved ([#9582](https://github.com/containers/podman/issues/9582)).
- The hidden `--trace` option to `podman` has been turned into a no-op. It was used in very early versions for performance tracing, but has not been supported for some time.
- The `podman generate systemd` command now generates `RequiresMountsFor` lines to ensure necessary storage directories are mounted before systemd starts Podman.
- Podman will now emit a warning when `--tty` and `--interactive` are both passed, but `STDIN` is not a TTY. This will be made into an error in the next major Podman release some time next year.

### Bugfixes
- Fixed a bug where rootless Podman containers joined to CNI networks could not receive traffic from forwarded ports ([#9065](https://github.com/containers/podman/issues/9065)).
- Fixed a bug where `podman network create` with the `--macvlan` flag did not honor the `--gateway`, `--subnet`, and `--opt` options ([#9167](https://github.com/containers/podman/issues/9167)).
- Fixed a bug where the `podman generate kube` command generated invalid YAML for privileged containers ([#8897](https://github.com/containers/podman/issues/8897)).
- Fixed a bug where the `podman generate kube` command could not be used with containers that were not running.
- Fixed a bug where the `podman generate systemd` command could duplicate some parameters to Podman in generated unit files ([#9776](https://github.com/containers/podman/issues/9776)).
- Fixed a bug where Podman did not add annotations specified in `containers.conf` to containers.
- Foxed a bug where Podman did not respect the `no_hosts` default in `containers.conf` when creating containers.
- Fixed a bug where the `--tail=0`, `--since`, and `--follow` options to the `podman logs` command did not function properly when using the `journald` log backend.
- Fixed a bug where specifying more than one container to `podman logs` when the `journald` log backend was in use did not function correctly.
- Fixed a bug where the `podman run` and `podman create` commands would panic if a memory limit was set, but the swap limit was set to unlimited ([#9429](https://github.com/containers/podman/issues/9429)).
- Fixed a bug where the `--network` option to `podman run`, `podman create`, and `podman pod create` would error if the user attempted to specify CNI networks by ID, instead of name ([#9451](https://github.com/containers/podman/issues/9451)).
- Fixed a bug where Podman's cgroup handling for cgroups v1 systems did not properly handle cases where a cgroup existed on some, but not all, controllers, resulting in errors from the `podman stats` command ([#9252](https://github.com/containers/podman/issues/9252)).
- Fixed a bug where the `podman cp` did not properly handle cases where `/dev/stdout` was specified as the destination (it was treated identically to `-`) ([#9362](https://github.com/containers/podman/issues/9362)).
- Fixed a bug where the `podman cp` command would create files with incorrect ownership ([#9526](https://github.com/containers/podman/issues/9626)).
- Fixed a bug where the `podman cp` command did not properly handle cases where the destination directory did not exist.
- Fixed a bug where the `podman cp` command did not properly evaluate symlinks when copying out of containers.
- Fixed a bug where the `podman rm -fa` command would error when attempting to remove containers created with `--rm` ([#9479](https://github.com/containers/podman/issues/9479)).
- Fixed a bug where the ordering of capabilities was nondeterministic in the `CapDrop` field of the output of `podman inspect` on a container ([#9490](https://github.com/containers/podman/issues/9490)).
- Fixed a bug where the `podman network connect` command could be used with containers that were not initially connected to a CNI bridge network (e.g. containers created with `--net=host`) ([#9496](https://github.com/containers/podman/issues/9496)).
- Fixed a bug where DNS search domains required by the `dnsname` CNI plugin were not being added to container's `resolv.conf` under some circumstances.
- Fixed a bug where the `--ignorefile` option to `podman build` was nonfunctional ([#9570](https://github.com/containers/podman/issues/9570)).
- Fixed a bug where the `--timestamp` option to `podman build` was nonfunctional ([#9569](https://github.com/containers/podman/issues/9569)).
- Fixed a bug where the `--iidfile` option to `podman build` could cause Podman to panic if an error occurred during the build.
- Fixed a bug where the `--dns-search` option to `podman build` was nonfunctional ([#9574](https://github.com/containers/podman/issues/9574)).
- Fixed a bug where the `--pull-never` option to `podman build` was nonfunctional ([#9573](https://github.com/containers/podman/issues/9573)).
- Fixed a bug where the `--build-arg` option to `podman build` would, when given a key but not a value, error (instead of attempting to look up the key as an environment variable) ([#9571](https://github.com/containers/podman/issues/9571)).
- Fixed a bug where the `--isolation` option to `podman build` in the remote Podman client was nonfunctional.
- Fixed a bug where the `podman network disconnect` command could cause errors when the container that had a network removed was stopped and its network was cleaned up ([#9602](https://github.com/containers/podman/issues/9602)).
- Fixed a bug where the `podman network rm` command did not properly check what networks a container was present in, resulting in unexpected behavior if `podman network connect` or `podman network disconnect` had been used with the network ([#9632](https://github.com/containers/podman/issues/9632)).
- Fixed a bug where some errors with stopping a container could cause Podman to panic, and the container to be stuck in an unusable `stopping` state ([#9615](https://github.com/containers/podman/issues/9615)).
- Fixed a bug where the `podman load` command could return 0 even in cases where an error occurred ([#9672](https://github.com/containers/podman/issues/9672)).
- Fixed a bug where specifying storage options to Podman using the `--storage-opt` option would override all storage options. Instead, storage options are now overridden only when the `--storage-driver` option is used to override the current graph driver ([#9657](https://github.com/containers/podman/issues/9657)).
- Fixed a bug where containers created with `--privileged` could request more capabilities than were available to Podman.
- Fixed a bug where `podman commit` did not use the `TMPDIR` environment variable to place temporary files created during the commit ([#9825](https://github.com/containers/podman/issues/9825)).
- Fixed a bug where remote Podman could error when attempting to resize short-lived containers ([#9831](https://github.com/containers/podman/issues/9831)).
- Fixed a bug where Podman was unusable on kernels built without `CONFIG_USER_NS`.
- Fixed a bug where the ownership of volumes created by `podman volume create` and then mounted into a container could be incorrect ([#9608](https://github.com/containers/podman/issues/9608)).
- Fixed a bug where Podman volumes using a volume plugin could not pass certain options, and could not be used as non-root users.
- Fixed a bug where the `--tz` option to `podman create` and `podman run` did not properly validate its input.

### API
- Fixed a bug where the `X-Registry-Auth` header did not accept `null` as a valid value.
- A new compat endpoint, `/auth`, has been added. This endpoint validates credentials against a registry ([#9564](https://github.com/containers/podman/issues/9564)).
- Fixed a bug where the compat Build endpoint for Images specified labels using the wrong type (array vs map). Both formats will be accepted now.
- Fixed a bug where the compat Build endpoint for Images did not report that it successfully tagged the built image in its response.
- Fixed a bug where the compat Create endpoint for Images did not provide progress information on pulling the image in its response.
- Fixed a bug where the compat Push endpoint for Images did not properly handle the destination (used a query parameter, instead of a path parameter).
- Fixed a bug where the compat Push endpoint for Images did not send the progress of the push and the digest of the pushed image in the response body.
- Fixed a bug where the compat List endpoint for Networks returned null, instead of an empty array (`[]`), when no networks were present ([#9293](https://github.com/containers/podman/issues/9293)).
- Fixed a bug where the compat List endpoint for Networks returned nulls, instead of empty maps, for networks that do not have Labels and/or Options.
- The Libpod Inspect endpoint for networks (`/libpod/network/$ID/json`) now has an alias at `/libpod/network/$ID` ([#9691](https://github.com/containers/podman/issues/9691)).
- Fixed a bug where the libpod Inspect endpoint for Networks returned a 1-size array of results, instead of a single result ([#9690](https://github.com/containers/podman/issues/9690)).
- The Compat List endpoint for Networks now supports the legacy format for filters in parallel with the current filter format ([#9526](https://github.com/containers/podman/issues/9526)).
- Fixed a bug where the compat Create endpoint for Containers did not properly handle tmpfs filesystems specified with options ([#9511](https://github.com/containers/podman/issues/9511)).
- Fixed a bug where the compat Create endpoint for Containers did not create bind-mount source directories ([#9510](https://github.com/containers/podman/issues/9510)).
- Fixed a bug where the compat Create endpoint for Containers did not properly handle the `NanoCpus` option ([#9523](https://github.com/containers/podman/issues/9523)).
- Fixed a bug where the Libpod create endpoint for Containers has a misnamed field in its JSON.
- Fixed a bug where the compat List endpoint for Containers did not populate information on forwarded ports ([#9553](https://github.com/containers/podman/issues/9553))
- Fixed a bug where the compat List endpoint for Containers did not populate information on container CNI networks ([#9529](https://github.com/containers/podman/issues/9529)).
- Fixed a bug where the compat and libpod Stop endpoints for Containers would ignore a timeout of 0.
- Fixed a bug where the compat and libpod Resize endpoints for Containers did not set the correct terminal sizes (dimensions were reversed) ([#9756](https://github.com/containers/podman/issues/9756)).
- Fixed a bug where the compat Remove endpoint for Containers would not return 404 when attempting to remove a container that does not exist ([#9675](https://github.com/containers/podman/issues/9675)).
- Fixed a bug where the compat Prune endpoint for Volumes would still prune even if an invalid filter was specified.
- Numerous bugs related to filters have been addressed.

### Misc
- Updated Buildah to v1.20.0
- Updated the containers/storage library to v1.28.1
- Updated the containers/image library to v5.10.5
- Updated the containers/common library to v0.35.4

## 3.0.1
### Changes
- Several frequently-occurring `WARN` level log messages have been downgraded to `INFO` or `DEBUG` to not clutter terminal output.

### Bugfixes
- Fixed a bug where the `Created` field of `podman ps --format=json` was formatted as a string instead of an Unix timestamp (integer) ([#9315](https://github.com/containers/podman/issues/9315)).
- Fixed a bug where failing lookups of individual layers during the `podman images` command would cause the whole command to fail without printing output.
- Fixed a bug where `--cgroups=split` did not function properly on cgroups v1 systems.
- Fixed a bug where mounting a volume over an directory in the container that existed, but was empty, could fail ([#9393](https://github.com/containers/podman/issues/9393)).
- Fixed a bug where mounting a volume over a directory in the container that existed could copy the entirety of the container's rootfs, instead of just the directory mounted over, into the volume ([#9415](https://github.com/containers/podman/pull/9415)).
- Fixed a bug where Podman would treat the `--entrypoint=[""]` option to `podman run` and `podman create` as a literal empty string in the entrypoint, when instead it should have been ignored ([#9377](https://github.com/containers/podman/issues/9377)).
- Fixed a bug where Podman would set the `HOME` environment variable to `""` when the container ran as a user without an assigned home directory ([#9378](https://github.com/containers/podman/issues/9378)).
- Fixed a bug where specifying a pod infra image that had no tags (by using its ID) would cause `podman pod create` to panic ([#9374](https://github.com/containers/podman/issues/9374)).
- Fixed a bug where the `--runtime` option was not properly handled by the `podman build` command ([#9365](https://github.com/containers/podman/issues/9365)).
- Fixed a bug where Podman would incorrectly print an error message related to the remote API when the remote API was not in use and starting Podman failed.
- Fixed a bug where Podman would change ownership of a container's working directory, even if it already existed ([#9387](https://github.com/containers/podman/issues/9387)).
- Fixed a bug where the `podman generate systemd --new` command would incorrectly escape `%t` when generating the path for the PID file ([#9373](https://github.com/containers/podman/issues/9373)).
- Fixed a bug where Podman could, when run inside a Podman container with the host's containers/storage directory mounted into the container, erroneously detect a reboot and reset container state if the temporary directory was not also mounted in ([#9191](https://github.com/containers/podman/issues/9191)).
- Fixed a bug where some options of the `podman build` command (including but not limited to `--jobs`) were nonfunctional ([#9247](https://github.com/containers/podman/issues/9247)).

### API
- Fixed a breaking change to the Libpod Wait API for Containers where the Conditions parameter changed type in Podman v3.0 ([#9351](https://github.com/containers/podman/issues/9351)).
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle forwarded ports that did not specify a host port.
- Fixed a bug where the Libpod Wait endpoint for Containers could write duplicate headers after an error occurred.
- Fixed a bug where the Compat Create endpoint for Images would not pull images that already had a matching tag present locally, even if a more recent version was available at the registry ([#9232](https://github.com/containers/podman/issues/9232)).
- The Compat Create endpoint for Images has had its compatibility with Docker improved, allowing its use with the `docker-java` library.

### Misc
- Updated Buildah to v1.19.4
- Updated the containers/storage library to v1.24.6

## 3.0.0
### Features
- Podman now features initial support for Docker Compose.
- Added the `podman rename` command, which allows containers to be renamed after they are created ([#1925](https://github.com/containers/podman/issues/1925)).
- The Podman remote client now supports the `podman copy` command.
- A new command, `podman network reload`, has been added. This command will re-configure the network of all running containers, and can be used to recreate firewall rules lost when the system firewall was reloaded (e.g. via `firewall-cmd --reload`).
- Podman networks now have IDs. They can be seen in `podman network ls` and can be used when removing and inspecting networks. Existing networks receive IDs automatically.
- Podman networks now also support labels. They can be added via the `--label` option to `network create`, and `podman network ls` can filter labels based on them.
- The `podman network create` command now supports setting bridge MTU and VLAN through the `--opt` option ([#8454](https://github.com/containers/podman/issues/8454)).
- The `podman container checkpoint` and `podman container restore` commands can now checkpoint and restore containers that include volumes.
- The `podman container checkpoint` command now supports the `--with-previous` and `--pre-checkpoint` options, and the `podman container restore` command now support the `--import-previous` option. These add support for two-step checkpointing with lowered dump times.
- The `podman push` command can now push manifest lists. Podman will first attempt to push as an image, then fall back to pushing as a manifest list if that fails.
- The `podman generate kube` command can now be run on multiple containers at once, and will generate a single pod containing all of them.
- The `podman generate kube` and `podman play kube` commands now support Kubernetes DNS configuration, and will preserve custom DNS configuration when exporting or importing YAML ([#9132](https://github.com/containers/podman/issues/9132)).
- The `podman generate kube` command now properly supports generating YAML for containers and pods creating using host networking (`--net=host`) ([#9077](https://github.com/containers/podman/issues/9077)).
- The `podman kill` command now supports a `--cidfile` option to kill containers given a file containing the container's ID ([#8443](https://github.com/containers/podman/issues/8443)).
- The `podman pod create` command now supports the `--net=none` option ([#9165](https://github.com/containers/podman/issues/9165)).
- The `podman volume create` command can now specify volume UID and GID as options with the `UID` and `GID` fields passed to the `--opt` option.
- Initial support has been added for Docker Volume Plugins. Podman can now define available plugins in `containers.conf` and use them to create volumes with `podman volume create --driver`.
- The `podman run` and `podman create` commands now support a new option, `--platform`, to specify the platform of the image to be used when creating the container.
- The `--security-opt` option to `podman run` and `podman create` now supports the `systempaths=unconfined` option to unrestrict access to all paths in the container, as well as `mask` and `unmask` options to allow more granular restriction of container paths.
- The `podman stats --format` command now supports a new format specified, `MemUsageBytes`, which prints the raw bytes of memory consumed by a container without human-readable formatting [#8945](https://github.com/containers/podman/issues/8945).
- The `podman ps` command can now filter containers based on what pod they are joined to via the `pod` filter ([#8512](https://github.com/containers/podman/issues/8512)).
- The `podman pod ps` command can now filter pods based on what networks they are joined to via the `network` filter.
- The `podman pod ps` command can now print information on what networks a pod is joined to via the `.Networks` specifier to the `--format` option.
- The `podman system prune` command now supports filtering what containers, pods, images, and volumes will be pruned.
- The `podman volume prune` commands now supports filtering what volumes will be pruned.
- The `podman system prune` command now includes information on space reclaimed ([#8658](https://github.com/containers/podman/issues/8658)).
- The `podman info` command will now properly print information about packages in use on Gentoo and Arch systems.
- The `containers.conf` file now contains an option for disabling creation of a new kernel keyring on container creation ([#8384](https://github.com/containers/podman/issues/8384)).
- The `podman image sign` command can now sign multi-arch images by producing a signature for each image in a given manifest list.
- The `podman image sign` command, when run as rootless, now supports per-user registry configuration files in `$HOME/.config/containers/registries.d`.
- Configuration options for `slirp4netns` can now be set system-wide via the `NetworkCmdOptions` configuration option in `containers.conf`.
- The MTU of `slirp4netns` can now be configured via the `mtu=` network command option (e.g. `podman run --net slirp4netns:mtu=9000`).

### Security
- A fix for CVE-2021-20199 is included. Podman between v1.8.0 and v2.2.1 used `127.0.0.1` as the source address for all traffic forwarded into rootless containers by a forwarded port; this has been changed to address the issue.

### Changes
- Shortname aliasing support has now been turned on by default. All Podman commands that must pull an image will, if a TTY is available, prompt the user about what image to pull.
- The `podman load` command no longer accepts a `NAME[:TAG]` argument. The presence of this argument broke CLI compatibility with Docker by making `docker load` commands unusable with Podman ([#7387](https://github.com/containers/podman/issues/7387)).
- The Go bindings for the HTTP API have been rewritten with a focus on limiting dependency footprint and improving extensibility. Read more [here](https://github.com/containers/podman/blob/v3.0/pkg/bindings/README.md).
- The legacy Varlink API has been completely removed from Podman.
- The default log level for Podman has been changed from Error to Warn.
- The `podman network create` command can now create `macvlan` networks using the `--driver macvlan` option for Docker compatibility. The existing `--macvlan` flag has been deprecated and will be removed in Podman 4.0 some time next year.
- The `podman inspect` command has had the `LogPath` and `LogTag` fields moved into the `LogConfig` structure (from the root of the Inspect structure). The maximum size of the log file is also included.
- The `podman generate systemd` command no longer generates unit files using the deprecated `KillMode=none` option ([#8615](https://github.com/containers/podman/issues/8615)).
- The `podman stop` command now releases the container lock while waiting for it to stop - as such, commands like `podman ps` will no longer block until `podman stop` completes ([#8501](https://github.com/containers/podman/issues/8501)).
- Networks created with `podman network create --internal` no longer use the `dnsname` plugin. This configuration never functioned as expected.
- Error messages for the remote Podman client have been improved when it cannot connect to a Podman service.
- Error messages for `podman run` when an invalid SELinux is specified have been improved.
- Rootless Podman features improved support for containers with a single user mapped into the rootless user namespace.
- Pod infra containers now respect default sysctls specified in `containers.conf` allowing for advanced configuration of the namespaces they will share.
- SSH public key handling for remote Podman has been improved.

### Bugfixes
- Fixed a bug where the `podman history --no-trunc` command would truncate the `Created By` field ([#9120](https://github.com/containers/podman/issues/9120)).
- Fixed a bug where root containers that did not explicitly specify a CNI network to join did not generate an entry for the network in use in the `Networks` field of the output of `podman inspect` ([#6618](https://github.com/containers/podman/issues/6618)).
- Fixed a bug where, under some circumstances, container working directories specified by the image (via the `WORKDIR` instruction) but not present in the image, would not be created ([#9040](https://github.com/containers/podman/issues/9040)).
- Fixed a bug where the `podman generate systemd` command would generate invalid unit files if the container was creating using a command line that included doubled braces (`{{` and `}}`), e.g. `--log-opt-tag={{.Name}}` ([#9034](https://github.com/containers/podman/issues/9034)).
- Fixed a bug where the `podman generate systemd --new` command could generate unit files including invalid Podman commands if the container was created using merged short options (e.g. `podman run -dt`) ([#8847](https://github.com/containers/podman/issues/8847)).
- Fixed a bug where the `podman generate systemd --new` command could generate unit files that did not handle Podman commands including some special characters (e.g. `$`) ([#9176](https://github.com/containers/podman/issues/9176)
- Fixed a bug where rootless containers joining CNI networks could not set a static IP address ([#7842](https://github.com/containers/podman/issues/7842)).
- Fixed a bug where rootless containers joining CNI networks could not set network aliases ([#8567](https://github.com/containers/podman/issues/8567)).
- Fixed a bug where the remote client could, under some circumstances, not include the `Containerfile` when sending build context to the server ([#8374](https://github.com/containers/podman/issues/8374)).
- Fixed a bug where rootless Podman did not mount `/sys` as a new `sysfs` in some circumstances where it was acceptable.
- Fixed a bug where rootless containers that both joined a user namespace and a CNI networks would cause a segfault. These options are incompatible and now return an error.
- Fixed a bug where the `podman play kube` command did not properly handle `CMD` and `ARGS` from images ([#8803](https://github.com/containers/podman/issues/8803)).
- Fixed a bug where the `podman play kube` command did not properly handle environment variables from images ([#8608](https://github.com/containers/podman/issues/8608)).
- Fixed a bug where the `podman play kube` command did not properly print errors that occurred when starting containers.
- Fixed a bug where the `podman play kube` command errored when `hostNetwork` was used ([#8790](https://github.com/containers/podman/issues/8790)).
- Fixed a bug where the `podman play kube` command would always pull images when the `:latest` tag was specified, even if the image was available locally ([#7838](https://github.com/containers/podman/issues/7838)).
- Fixed a bug where the `podman play kube` command did not properly handle SELinux configuration, rending YAML with custom SELinux configuration unusable ([#8710](https://github.com/containers/podman/issues/8710)).
- Fixed a bug where the `podman generate kube` command incorrectly populated the `args` and `command` fields of generated YAML ([#9211](https://github.com/containers/podman/issues/9211)).
- Fixed a bug where containers in a pod would create a duplicate entry in the pod's shared `/etc/hosts` file every time the container restarted ([#8921](https://github.com/containers/podman/issues/8921)).
- Fixed a bug where the `podman search --list-tags` command did not support the `--format` option ([#8740](https://github.com/containers/podman/issues/8740)).
- Fixed a bug where the `http_proxy` option in `containers.conf` was not being respected, and instead was set unconditionally to true ([#8843](https://github.com/containers/podman/issues/8843)).
- Fixed a bug where rootless Podman could, on systems with a recent Conmon and users with a long username, fail to attach to containers ([#8798](https://github.com/containers/podman/issues/8798)).
- Fixed a bug where the `podman images` command would break and fail to display any images if an empty manifest list was present in storage ([#8931](https://github.com/containers/podman/issues/8931)).
- Fixed a bug where locale environment variables were not properly passed on to Conmon.
- Fixed a bug where Podman would not build on the MIPS architecture ([#8782](https://github.com/containers/podman/issues/8782)).
- Fixed a bug where rootless Podman could fail to properly configure user namespaces for rootless containers when the user specified a `--uidmap` option that included a mapping beginning with UID `0`.
- Fixed a bug where the `podman logs` command using the `k8s-file` backend did not properly handle partial log lines with a length of 1 ([#8879](https://github.com/containers/podman/issues/8879)).
- Fixed a bug where the `podman logs` command with the `--follow` option did not properly handle log rotation ([#8733](https://github.com/containers/podman/issues/8733)).
- Fixed a bug where user-specified `HOSTNAME` environment variables were overwritten by Podman ([#8886](https://github.com/containers/podman/issues/8886)).
- Fixed a bug where Podman would applied default sysctls from `containers.conf` in too many situations (e.g. applying network sysctls when the container shared its network with a pod).
- Fixed a bug where Podman did not properly handle cases where a secondary image store was in use and an image was present in both the secondary and primary stores ([#8176](https://github.com/containers/podman/issues/8176)).
- Fixed a bug where systemd-managed rootless Podman containers where the user in the container was not root could fail as the container's PID file was not accessible to systemd on the host ([#8506](https://github.com/containers/podman/issues/8506)).
- Fixed a bug where the `--privileged` option to `podman run` and `podman create` would, under some circumstances, not disable Seccomp ([#8849](https://github.com/containers/podman/issues/8849)).
- Fixed a bug where the `podman exec` command did not properly add capabilities when the container or exec session were run with `--privileged`.
- Fixed a bug where rootless Podman would use the `--enable-sandbox` option to `slirp4netns` unconditionally, even when `pivot_root` was disabled, rendering `slirp4netns` unusable when `pivot_root` was disabled ([#8846](https://github.com/containers/podman/issues/8846)).
- Fixed a bug where `podman build --logfile` did not actually write the build's log to the logfile.
- Fixed a bug where the `podman system service` command did not close STDIN, and could display user-interactive prompts ([#8700](https://github.com/containers/podman/issues/8700)).
- Fixed a bug where the `podman system reset` command could, under some circumstances, remove all the contents of the `XDG_RUNTIME_DIR` directory ([#8680](https://github.com/containers/podman/issues/8680)).
- Fixed a bug where the `podman network create` command created CNI configurations that did not include a default gateway ([#8748](https://github.com/containers/podman/issues/8748)).
- Fixed a bug where the `podman.service` systemd unit provided by default used the wrong service type, and would cause systemd to not correctly register the service as started ([#8751](https://github.com/containers/podman/issues/8751)).
- Fixed a bug where, if the `TMPDIR` environment variable was set for the container engine in `containers.conf`, it was being ignored.
- Fixed a bug where the `podman events` command did not properly handle future times given to the `--until` option ([#8694](https://github.com/containers/podman/issues/8694)).
- Fixed a bug where the `podman logs` command wrote container `STDERR` logs to `STDOUT` instead of `STDERR` ([#8683](https://github.com/containers/podman/issues/8683)).
- Fixed a bug where containers created from an image with multiple tags would report that they were created from the wrong tag ([#8547](https://github.com/containers/podman/issues/8547)).
- Fixed a bug where container capabilities were not set properly when the `--cap-add=all` and `--user` options to `podman create` and `podman run` were combined.
- Fixed a bug where the `--layers` option to `podman build` was nonfunctional ([#8643](https://github.com/containers/podman/issues/8643)).
- Fixed a bug where the `podman system prune` command did not act recursively, and thus would leave images, containers, pods, and volumes present that would be removed by a subsequent call to `podman system prune` ([#7990](https://github.com/containers/podman/issues/7990)).
- Fixed a bug where the `--publish` option to `podman run` and `podman create` did not properly handle ports specified as a range of ports with no host port specified ([#8650](https://github.com/containers/podman/issues/8650)).
- Fixed a bug where `--format` did not support JSON output for individual fields ([#8444](https://github.com/containers/podman/issues/8444)).
- Fixed a bug where the `podman stats` command would fail when run on root containers using the `slirp4netns` network mode ([#7883](https://github.com/containers/podman/issues/7883)).
- Fixed a bug where the Podman remote client would ask for a password even if the server's SSH daemon did not support password authentication ([#8498](https://github.com/containers/podman/issues/8498)).
- Fixed a bug where the `podman stats` command would fail if the system did not support one or more of the cgroup controllers Podman supports ([#8588](https://github.com/containers/podman/issues/8588)).
- Fixed a bug where the `--mount` option to `podman create` and `podman run` did not ignore the `consistency` mount option.
- Fixed a bug where failures during the resizing of a container's TTY would print the wrong error.
- Fixed a bug where the `podman network disconnect` command could cause the `podman inspect` command to fail for a container until it was restarted ([#9234](https://github.com/containers/podman/issues/9234)).
- Fixed a bug where containers created from a read-only rootfs (using the `--rootfs` option to `podman create` and `podman run`) would fail ([#9230](https://github.com/containers/podman/issues/9230)).
- Fixed a bug where specifying Go templates to the `--format` option to multiple Podman commands did not support the `join` function ([#8773](https://github.com/containers/podman/issues/8773)).
- Fixed a bug where the `podman rmi` command could, when run in parallel on multiple images, return `layer not known` errors ([#6510](https://github.com/containers/podman/issues/6510)).
- Fixed a bug where the `podman inspect` command on containers displayed unlimited ulimits incorrectly ([#9303](https://github.com/containers/podman/issues/9303)).
- Fixed a bug where Podman would fail to start when a volume was mounted over a directory in a container that contained symlinks that terminated outside the directory and its subdirectories ([#6003](https://github.com/containers/podman/issues/6003)).

### API
- Libpod API version has been bumped to v3.0.0.
- All Libpod Pod APIs have been modified to properly report errors with individual containers. Cases where the operation as a whole succeeded but individual containers failed now report an HTTP 409 error ([#8865](https://github.com/containers/podman/issues/8865)).
- The Compat API for Containers now supports the Rename and Copy APIs.
- Fixed a bug where the Compat Prune APIs (for volumes, containers, and images) did not return the amount of space reclaimed in their responses.
- Fixed a bug where the Compat and Libpod Exec APIs for Containers would drop errors that occurred prior to the exec session successfully starting (e.g. a "no such file" error if an invalid executable was passed) ([#8281](https://github.com/containers/podman/issues/8281))
- Fixed a bug where the Volumes field in the Compat Create API for Containers was being ignored ([#8649](https://github.com/containers/podman/issues/8649)).
- Fixed a bug where the NetworkMode field in the Compat Create API for Containers was not handling some values, e.g. `container:`, correctly.
- Fixed a bug where the Compat Create API for Containers did not set container name properly.
- Fixed a bug where containers created using the Compat Create API unconditionally used Kubernetes file logging (the default specified in `containers.conf` is now used).
- Fixed a bug where the Compat Inspect API for Containers could include container states not recognized by Docker.
- Fixed a bug where Podman did not properly clean up after calls to the Events API when the `journald` backend was in use, resulting in a leak of file descriptors ([#8864](https://github.com/containers/podman/issues/8864)).
- Fixed a bug where the Libpod Pull endpoint for Images could fail with an `index out of range` error under certain circumstances ([#8870](https://github.com/containers/podman/issues/8870)).
- Fixed a bug where the Libpod Exists endpoint for Images could panic.
- Fixed a bug where the Compat List API for Containers did not support all filters ([#8860](https://github.com/containers/podman/issues/8860)).
- Fixed a bug where the Compat List API for Containers did not properly populate the Status field.
- Fixed a bug where the Compat and Libpod Resize APIs for Containers ignored the height and width parameters ([#7102](https://github.com/containers/podman/issues/7102)).
- Fixed a bug where the Compat Search API for Images returned an incorrectly-formatted JSON response ([#8758](https://github.com/containers/podman/pull/8758)).
- Fixed a bug where the Compat Load API for Images did not properly clean up temporary files.
- Fixed a bug where the Compat Create API for Networks could panic when an empty IPAM configuration was specified.
- Fixed a bug where the Compat Inspect and List APIs for Networks did not include Scope.
- Fixed a bug where the Compat Wait endpoint for Containers did not support the same wait conditions that Docker did.

### Misc
- Updated Buildah to v1.19.2
- Updated the containers/storage library to v1.24.5
- Updated the containers/image library to v5.10.2
- Updated the containers/common library to v0.33.4

## v2.2.1
### Changes
- Due to a conflict with a previously-removed field, we were forced to modify the way image volumes (mounting images into containers using `--mount type=image`) were handled in the database. As a result, containers created in Podman 2.2.0 with image volumes will not have them in v2.2.1, and these containers will need to be re-created.

### Bugfixes
- Fixed a bug where rootless Podman would, on systems without the `XDG_RUNTIME_DIR` environment variable defined, use an incorrect path for the PID file of the Podman pause process, causing Podman to fail to start ([#8539](https://github.com/containers/podman/issues/8539)).
- Fixed a bug where containers created using Podman v1.7 and earlier were unusable in Podman due to JSON decode errors ([#8613](https://github.com/containers/podman/issues/8613)).
- Fixed a bug where Podman could retrieve invalid cgroup paths, instead of erroring, for containers that were not running.
- Fixed a bug where the `podman system reset` command would print a warning about a duplicate shutdown handler being registered.
- Fixed a bug where rootless Podman would attempt to mount `sysfs` in circumstances where it was not allowed; some OCI runtimes (notably `crun`) would fall back to alternatives and not fail, but others (notably `runc`) would fail to run containers.
- Fixed a bug where the `podman run` and `podman create` commands would fail to create containers from untagged images ([#8558](https://github.com/containers/podman/issues/8558)).
- Fixed a bug where remote Podman would prompt for a password even when the server did not support password authentication ([#8498](https://github.com/containers/podman/issues/8498)).
- Fixed a bug where the `podman exec` command did not move the Conmon process for the exec session into the correct cgroup.
- Fixed a bug where shell completion for the `ancestor` option to `podman ps --filter` did not work correctly.
- Fixed a bug where detached containers would not properly clean themselves up (or remove themselves if `--rm` was set) if the Podman command that created them was invoked with `--log-level=debug`.

### API
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the `Binds` and `Mounts` parameters in `HostConfig`.
- Fixed a bug where the Compat Create endpoint for Containers ignored the `Name` query parameter.
- Fixed a bug where the Compat Create endpoint for Containers did not properly handle the "default" value for `NetworkMode` (this value is used extensively by `docker-compose`) ([#8544](https://github.com/containers/podman/issues/8544)).
- Fixed a bug where the Compat Build endpoint for Images would sometimes incorrectly use the `target` query parameter as the image's tag.

### Misc
- Podman v2.2.0 vendored a non-released, custom version of the `github.com/spf13/cobra` package; this has been reverted to the latest upstream release to aid in packaging.
- Updated the containers/image library to v5.9.0

## 2.2.0
### Features
- Experimental support for shortname aliasing has been added. This is not enabled by default, but can be turned on by setting the environment variable `CONTAINERS_SHORT_NAME_ALIASING` to `on`. Documentation is [available here](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md#short-name-aliasing).
- Initial support has been added for the `podman network connect` and `podman network disconnect` commands, which allow existing containers to modify what networks they are connected to. At present, these commands can only be used on running containers that did not specify `--network=none` when they were created.
- The `podman run` command now supports the `--network-alias` option to set network aliases (additional names the container can be accessed at from other containers via DNS if the `dnsname` CNI plugin is in use). Aliases can also be added and removed using the new `podman network connect` and `podman network disconnect` commands. Please note that this requires a new release (v1.1.0) of the `dnsname` plugin, and will only work on newly-created CNI networks.
- The `podman generate kube` command now features support for exporting container's memory and CPU limits ([#7855](https://github.com/containers/podman/issues/7855)).
- The `podman play kube` command now features support for setting CPU and Memory limits for containers ([#7742](https://github.com/containers/podman/issues/7742)).
- The `podman play kube` command now supports persistent volumes claims using Podman named volumes.
- The `podman play kube` command now supports Kubernetes configmaps via the `--configmap` option ([#7567](https://github.com/containers/podman/issues/7567)).
- The `podman play kube` command now supports a `--log-driver` option to set the log driver for created containers.
- The `podman play kube` command now supports a `--start` option, enabled by default, to start the pod after creating it. This allows for `podman play kube` to be more easily used in systemd unitfiles.
- The `podman network create` command now supports the `--ipv6` option to enable dual-stack IPv6 networking for created networks ([#7302](https://github.com/containers/podman/issues/7302)).
- The `podman inspect` command can now inspect pods, networks, and volumes, in addition to containers and images ([#6757](https://github.com/containers/podman/issues/6757)).
- The `--mount` option for `podman run` and `podman create` now supports a new type, `image`, to mount the contents of an image into the container at a given location.
- The Bash and ZSH completions have been completely reworked and have received significant enhancements! Additionally, support for Fish completions and completions for the `podman-remote` executable have been added.
- The `--log-opt` option for `podman create` and `podman run` now supports the `max-size` option to set the maximum size for a container's logs ([#7434](https://github.com/containers/podman/issues/7434)).
- The `--network` option to the `podman pod create` command now allows pods to be configured to use `slirp4netns` networking, even when run as root ([#6097](https://github.com/containers/podman/issues/6097)).
- The `podman pod stop`, `podman pod pause`, `podman pod unpause`, and `podman pod kill` commands now work on multiple containers in parallel and should be significantly faster.
- The `podman search` command now supports a `--list-tags` option to list all available tags for a single image in a single repository.
- The `podman search` command can now output JSON using the `--format=json` option.
- The `podman diff` and `podman mount` commands now work with all containers in the storage library, including those not created by Podman. This allows them to be used with Buildah and CRI-O containers.
- The `podman container exists` command now features a `--external` option to check if a container exists not just in Podman, but also in the storage library. This will allow Podman to identify Buildah and CRI-O containers.
- The `--tls-verify` and `--authfile` options have been enabled for use with remote Podman.
- The `/etc/hosts` file now includes the container's name and hostname (both pointing to localhost) when the container is run with `--net=none` ([#8095](https://github.com/containers/podman/issues/8095)).
- The `podman events` command now supports filtering events based on the labels of the container they occurred on using the `--filter label=key=value` option.
- The `podman volume ls` command now supports filtering volumes based on their labels using the `--filter label=key=value` option.
- The `--volume` and `--mount` options to `podman run` and `podman create` now support two new mount propagation options, `unbindable` and `runbindable`.
- The `name` and `id` filters for `podman pod ps` now match based on a regular expression, instead of requiring an exact match.
- The `podman pod ps` command now supports a new filter `status`, that matches pods in a certain state.

### Changes
- The `podman network rm --force` command will now also remove pods that are using the network ([#7791](https://github.com/containers/podman/issues/7791)).
- The `podman volume rm`, `podman network rm`, and `podman pod rm` commands now return exit code 1 if the object specified for removal does not exist, and exit code 2 if the object is in use and the `--force` option was not given.
- If `/dev/fuse` is passed into Podman containers as a device, Podman will open it before starting the container to ensure that the kernel module is loaded on the host and the device is usable in the container.
- Global Podman options that were not supported with remote operation have been removed from `podman-remote` (e.g. `--cgroup-manager`, `--storage-driver`).
- Many errors have been changed to remove repetition and be more clear as to what has gone wrong.
- The `--storage` option to `podman rm` is now enabled by default, with slightly changed semantics. If the given container does not exist in Podman but does exist in the storage library, it will be removed even without the `--storage` option. If the container exists in Podman it will be removed normally. The `--storage` option for `podman rm` is now deprecated and will be removed in a future release.
- The `--storage` option to `podman ps` has been renamed to `--external`. An alias has been added so the old form of the option will continue to work.
- Podman now delays the SIGTERM and SIGINT signals during container creation to ensure that Podman is not stopped midway through creating a container resulting in potential resource leakage ([#7941](https://github.com/containers/podman/issues/7941)).
- The `podman save` command now strips signatures from images it is exporting, as the formats we export to do not support signatures ([#7659](https://github.com/containers/podman/issues/7659)).
- A new `Degraded` state has been added to pods. Pods that have some, but not all, of their containers running are now considered to be `Degraded` instead of `Running`.
- Podman will now print a warning when conflicting network options related to port forwarding (e.g. `--publish` and `--net=host`) are specified when creating a container.
- The `--restart on-failure` and `--rm` options for containers no longer conflict. When both are specified, the container will be restarted if it exits with a non-zero error code, and removed if it exits cleanly ([#7906](https://github.com/containers/podman/issues/7906)).
- Remote Podman will no longer use settings from the client's `containers.conf`; defaults will instead be provided by the server's `containers.conf` ([#7657](https://github.com/containers/podman/issues/7657)).
- The `podman network rm` command now has a new alias, `podman network remove` ([#8402](https://github.com/containers/podman/issues/8402)).

### Bugfixes
- Fixed a bug where `podman load` on the remote client did not error when attempting to load a directory, which is not yet supported for remote use.
- Fixed a bug where rootless Podman could hang when the `newuidmap` binary was not installed ([#7776](https://github.com/containers/podman/issues/7776)).
- Fixed a bug where the `--pull` option to `podman run`, `podman create`,  and `podman build` did not match Docker's behavior.
- Fixed a bug where sysctl settings from the `containers.conf` configuration file were applied, even if the container did not join the namespace associated with a sysctl.
- Fixed a bug where Podman would not return the text of errors encountered when trying to run a healthcheck for a container.
- Fixed a bug where Podman was accidentally setting the `containers` environment variable in addition to the expected `container` environment variable.
- Fixed a bug where rootless Podman using CNI networking did not properly clean up DNS entries for removed containers ([#7789](https://github.com/containers/podman/issues/7789)).
- Fixed a bug where the `podman untag --all` command was not supported with remote Podman.
- Fixed a bug where the `podman system service` command could time out even if active attach connections were present ([#7826](https://github.com/containers/podman/issues/7826)).
- Fixed a bug where the `podman system service` command would sometimes never time out despite no active connections being present.
- Fixed a bug where Podman's handling of capabilities, specifically inheritable, did not match Docker's.
- Fixed a bug where `podman run` would fail if the image specified was a manifest list and had already been pulled ([#7798](https://github.com/containers/podman/pull/7798)).
- Fixed a bug where Podman did not take search registries into account when looking up images locally ([#6381](https://github.com/containers/podman/issues/6381)).
- Fixed a bug where the `podman manifest inspect` command would fail for images that had already been pulled ([#7726](https://github.com/containers/podman/issues/7726)).
- Fixed a bug where rootless Podman would not add supplemental GIDs to containers when when a user, but not a group, was set via the `--user` option to `podman create` and `podman run` and sufficient GIDs were available to add the groups ([#7782](https://github.com/containers/podman/issues/7782)).
- Fixed a bug where remote Podman commands did not properly handle cases where the user gave a name that could also be a short ID for a pod or container ([#7837](https://github.com/containers/podman/issues/7837)).
- Fixed a bug where `podman image prune` could leave images ready to be pruned after `podman image prune` was run ([#7872](https://github.com/containers/podman/issues/7872)).
- Fixed a bug where the `podman logs` command with the `journald` log driver would not read all available logs ([#7476](https://github.com/containers/podman/issues/7476)).
- Fixed a bug where the `--rm` and `--restart` options to `podman create` and `podman run` did not conflict when a restart policy that is not `on-failure` was chosen ([#7878](https://github.com/containers/podman/issues/7878)).
- Fixed a bug where the `--format "table {{ .Field }}"` option to numerous Podman commands ceased to function on Podman v2.0 and up.
- Fixed a bug where pods did not properly share an SELinux label between their containers, resulting in containers being unable to see the processes of other containers when the pod shared a PID namespace ([#7886](https://github.com/containers/podman/issues/7886)).
- Fixed a bug where the `--namespace` option to `podman ps` did not work with the remote client ([#7903](https://github.com/containers/podman/issues/7903)).
- Fixed a bug where rootless Podman incorrectly calculated the number of UIDs available in the container if multiple different ranges of UIDs were specified.
- Fixed a bug where the `/etc/hosts` file would not be correctly populated for containers in a user namespace ([#7490](https://github.com/containers/podman/issues/7490)).
- Fixed a bug where the `podman network create` and `podman network remove` commands could race when run in parallel, with unpredictable results ([#7807](https://github.com/containers/podman/issues/7807)).
- Fixed a bug where the `-p` option to `podman run`, `podman create`, and `podman pod create` would, when given only a single number (e.g. `-p 80`), assign the same port for both host and container, instead of generating a random host port ([#7947](https://github.com/containers/podman/issues/7947)).
- Fixed a bug where Podman containers did not properly store the cgroup manager they were created with, causing them to stop functioning after the cgroup manager was changed in `containers.conf` or with the `--cgroup-manager` option ([#7830](https://github.com/containers/podman/issues/7830)).
- Fixed a bug where the `podman inspect` command did not include information on the CNI networks a container was connected to if it was not running.
- Fixed a bug where the `podman attach` command would not print a newline after detaching from the container ([#7751](https://github.com/containers/podman/issues/7751)).
- Fixed a bug where the `HOME` environment variable was not set properly in containers when the `--userns=keep-id` option was set ([#8004](https://github.com/containers/podman/issues/8004)).
- Fixed a bug where the `podman container restore` command could panic when the container in question was in a pod ([#8026](https://github.com/containers/podman/issues/8026)).
- Fixed a bug where the output of the `podman image trust show --raw` command was not properly formatted.
- Fixed a bug where the `podman runlabel` command could panic if a label to run was not given ([#8038](https://github.com/containers/podman/issues/8038)).
- Fixed a bug where the `podman run` and `podman start --attach` commands would exit with an error when the user detached manually using the detach keys on remote Podman ([#7979](https://github.com/containers/podman/issues/7979)).
- Fixed a bug where rootless CNI networking did not use the `dnsname` CNI plugin if it was not available on the host, despite it always being available in the container used for rootless networking ([#8040](https://github.com/containers/podman/issues/8040)).
- Fixed a bug where Podman did not properly handle cases where an OCI runtime is specified by its full path, and could revert to using another OCI runtime with the same binary path that existed in the system `$PATH` on subsequent invocations.
- Fixed a bug where the `--net=host` option to `podman create` and `podman run` would cause the `/etc/hosts` file to be incorrectly populated ([#8054](https://github.com/containers/podman/issues/8054)).
- Fixed a bug where the `podman inspect` command did not include container network information when the container shared its network namespace (IE, joined a pod or another container's network namespace via `--net=container:...`) ([#8073](https://github.com/containers/podman/issues/8073)).
- Fixed a bug where the `podman ps` command did not include information on all ports a container was publishing.
- Fixed a bug where the `podman build` command incorrectly forwarded `STDIN` into build containers from `RUN` instructions.
- Fixed a bug where the `podman wait` command's `--interval` option did not work when units were not specified for the duration ([#8088](https://github.com/containers/podman/issues/8088)).
- Fixed a bug where the `--detach-keys` and `--detach` options could be passed to `podman create` despite having no effect (and not making sense in that context).
- Fixed a bug where Podman could not start containers if running on a system without a `/etc/resolv.conf` file (which occurs on some WSL2 images) ([#8089](https://github.com/containers/podman/issues/8089)).
- Fixed a bug where the `--extract` option to `podman cp` was nonfunctional.
- Fixed a bug where the `--cidfile` option to `podman run` would, when the container was not run with `--detach`, only create the file after the container exited ([#8091](https://github.com/containers/podman/issues/8091)).
- Fixed a bug where the `podman images` and `podman images -a` commands could panic and not list any images when certain improperly-formatted images were present in storage ([#8148](https://github.com/containers/podman/issues/8148)).
- Fixed a bug where the `podman events` command could, when the `journald` events backend was in use, become nonfunctional when a badly-formatted event or a log message that container certain string was present in the journal ([#8125](https://github.com/containers/podman/issues/8125)).
- Fixed a bug where remote Podman would, when using SSH transport, not authenticate to the server using hostkeys when connecting on a port other than 22 ([#8139](https://github.com/containers/podman/issues/8139)).
- Fixed a bug where the `podman attach` command would not exit when containers stopped ([#8154](https://github.com/containers/podman/issues/8154)).
- Fixed a bug where Podman did not properly clean paths before verifying them, resulting in Podman refusing to start if the root or temporary directories were specified with extra trailing `/` characters ([#8160](https://github.com/containers/podman/issues/8160)).
- Fixed a bug where remote Podman did not support hashed hostnames in the `known_hosts` file on the host for establishing connections ([#8159](https://github.com/containers/podman/pull/8159)).
- Fixed a bug where the `podman image exists` command would return non-zero (false) when multiple potential matches for the given name existed.
- Fixed a bug where the `podman manifest inspect` command on images that are not manifest lists would error instead of inspecting the image ([#8023](https://github.com/containers/podman/issues/8023)).
- Fixed a bug where the `podman system service` command would fail if the directory the Unix socket was to be created inside did not exist ([#8184](https://github.com/containers/podman/issues/8184)).
- Fixed a bug where pods that shared the IPC namespace (which is done by default) did not share a `/dev/shm` filesystem between all containers in the pod ([#8181](https://github.com/containers/podman/issues/8181)).
- Fixed a bug where filters passed to `podman volume list` were not inclusive ([#6765](https://github.com/containers/podman/issues/6765)).
- Fixed a bug where the `podman volume create` command would fail when the volume's data directory already existed (as might occur when a volume was not completely removed) ([#8253](https://github.com/containers/podman/issues/8253)).
- Fixed a bug where the `podman run` and `podman create` commands would deadlock when trying to create a container that mounted the same named volume at multiple locations (e.g. `podman run -v testvol:/test1 -v testvol:/test2`) ([#8221](https://github.com/containers/podman/issues/8221)).
- Fixed a bug where the parsing of the `--net` option to `podman build` was incorrect ([#8322](https://github.com/containers/podman/issues/8322)).
- Fixed a bug where the `podman build` command would print the ID of the built image twice when using remote Podman ([#8332](https://github.com/containers/podman/issues/8332)).
- Fixed a bug where the `podman stats` command did not show memory limits for containers ([#8265](https://github.com/containers/podman/issues/8265)).
- Fixed a bug where the `podman pod inspect` command printed the static MAC address of the pod in a non-human-readable format ([#8386](https://github.com/containers/podman/pull/8386)).
- Fixed a bug where the `--tls-verify` option of the `podman play kube` command had its logic inverted (`false` would enforce the use of TLS, `true` would disable it).
- Fixed a bug where the `podman network rm` command would error when trying to remove `macvlan` networks and rootless CNI networks ([#8491](https://github.com/containers/podman/issues/8491)).
- Fixed a bug where Podman was not setting sane defaults for missing `XDG_` environment variables.
- Fixed a bug where remote Podman would check if volume paths to be mounted in the container existed on the host, not the server ([#8473](https://github.com/containers/podman/issues/8473)).
- Fixed a bug where the `podman manifest create` and `podman manifest add` commands on local images would drop any images in the manifest not pulled by the host.
- Fixed a bug where networks made by `podman network create` did not include the `tuning` plugin, and as such did not support setting custom MAC addresses ([#8385](https://github.com/containers/podman/issues/8385)).
- Fixed a bug where container healthchecks did not use `$PATH` when searching for the Podman executable to run the healthcheck.
- Fixed a bug where the `--ip-range` option to `podman network create` did not properly handle non-classful subnets when calculating the last usable IP for DHCP assignment ([#8448](https://github.com/containers/podman/issues/8448)).
- Fixed a bug where the `podman container ps` alias for `podman ps` was missing ([#8445](https://github.com/containers/podman/issues/8445)).

### API
- The Compat Create endpoint for Container has received a major refactor to share more code with the Libpod Create endpoint, and should be significantly more stable.
- A Compat endpoint for exporting multiple images at once, `GET /images/get`, has been added ([#7950](https://github.com/containers/podman/issues/7950)).
- The Compat Network Connect and Network Disconnect endpoints have been added.
- Endpoints that deal with image registries now support a `X-Registry-Config` header to specify registry authentication configuration.
- The Compat Create endpoint for images now properly supports specifying images by digest.
- The Libpod Build endpoint for images now supports an `httpproxy` query parameter which, if set to true, will forward the server's HTTP proxy settings into the build container for `RUN` instructions.
- The Libpod Untag endpoint for images will now remove all tags for the given image if no repository and tag are specified for removal.
- Fixed a bug where the Ping endpoint misspelled a header name (`Libpod-Buildha-Version` instead of `Libpod-Buildah-Version`).
- Fixed a bug where the Ping endpoint sent an extra newline at the end of its response where Docker did not.
- Fixed a bug where the Compat Logs endpoint for containers did not send a newline character after each log line.
- Fixed a bug where the Compat Logs endpoint for containers would mangle line endings to change newline characters to add a preceding carriage return ([#7942](https://github.com/containers/podman/issues/7942)).
- Fixed a bug where the Compat Inspect endpoint for Containers did not properly list the container's stop signal ([#7917](https://github.com/containers/podman/issues/7917)).
- Fixed a bug where the Compat Inspect endpoint for Containers formatted the container's create time incorrectly ([#7860](https://github.com/containers/podman/issues/7860)).
- Fixed a bug where the Compat Inspect endpoint for Containers did not include the container's Path, Args, and Restart Count.
- Fixed a bug where the Compat Inspect endpoint for Containers prefixed added and dropped capabilities with `CAP_` (Docker does not do so).
- Fixed a bug where the Compat Info endpoint for the Engine did not include configured registries.
- Fixed a bug where the server could panic if a client closed a connection midway through an image pull ([#7896](https://github.com/containers/podman/issues/7896)).
- Fixed a bug where the Compat Create endpoint for volumes returned an error when a volume with the same name already existed, instead of succeeding with a 201 code ([#7740](https://github.com/containers/podman/issues/7740)).
- Fixed a bug where a client disconnecting from the Libpod or Compat events endpoints could result in the server using 100% CPU ([#7946](https://github.com/containers/podman/issues/7946)).
- Fixed a bug where the "no such image" error message sent by the Compat Inspect endpoint for Images returned a 404 status code with an error that was improperly formatted for Docker compatibility.
- Fixed a bug where the Compat Create endpoint for networks did not properly set a default for the `driver` parameter if it was not provided by the client.
- Fixed a bug where the Compat Inspect endpoint for images did not populate the `RootFS`, `VirtualSize`, `ParentId`, `Architecture`, `Os`, and `OsVersion` fields of the response.
- Fixed a bug where the Compat Inspect endpoint for images would omit the `ParentId` field if the image had no parent, and the `Created` field if the image did not have a creation time.
- Fixed a bug where the Compat Remove endpoint for Networks did not support the `Force` query parameter.

### Misc
- Updated Buildah to v1.18.0
- Updated the containers/storage library to v1.24.1
- Updated the containers/image library to v5.8.1
- Updated the containers/common library to v0.27.0

## 2.1.1
### Changes
- The `podman info` command now includes the cgroup manager Podman is using.

### Bugfixes
- Fixed a bug where Podman would not build with the `varlink` build tag enabled.
- Fixed a bug where the `podman save` command could, when asked to save multiple images, write its progress bar to the archive instead of the terminal, producing a corrupted archive.
- Fixed a bug where the `json-file` log driver did not write logs.
- Fixed a bug where `podman-remote start --attach` did not properly handle detaching using the detach keys.
- Fixed a bug where `podman pod ps --filter label=...` did not work.
- Fixed a bug where the `podman build` command did not respect the `--runtime` flag.

### API
- The REST API now includes a Server header in all responses.
- Fixed a bug where the Libpod and Compat Attach endpoints could terminate early, before sending all output from the container.
- Fixed a bug where the Compat Create endpoint for containers did not properly handle the Interactive parameter.
- Fixed a bug where the Compat Kill endpoint for containers could continue to run after a fatal error.
- Fixed a bug where the Limit parameter of the Compat List endpoint for Containers did not properly handle a limit of 0 (returning nothing, instead of all containers) ([#7722](https://github.com/containers/podman/issues/7722)).
- The Libpod Stats endpoint for containers is being deprecated and will be replaced by a similar endpoint with additional features in a future release.

## 2.1.0
### Features
- A new command, `podman image mount`, has been added. This allows for an image to be mounted, read-only, to inspect its contents without creating a container from it ([#1433](https://github.com/containers/podman/issues/1433)).
- The `podman save` and `podman load` commands can now create and load archives containing multiple images ([#2669](https://github.com/containers/podman/issues/2669)).
- Rootless Podman now supports all `podman network` commands, and rootless containers can now be joined to networks.
- The performance of `podman build` on `ADD` and `COPY` instructions has been greatly improved, especially when a `.dockerignore` is present.
- The `podman run` and `podman create` commands now support a new mode for the `--cgroups` option, `--cgroups=split`. Podman will create two cgroups under the cgroup it was launched in, one for the container and one for Conmon. This mode is useful for running Podman in a systemd unit, as it ensures that all processes are retained in systemd's cgroup hierarchy ([#6400](https://github.com/containers/podman/issues/6400)).
- The `podman run` and `podman create` commands can now specify options to slirp4netns by using the `--network` option as follows:  `--net slirp4netns:opt1,opt2`. This allows for, among other things, switching the port forwarder used by slirp4netns away from rootlessport.
- The `podman ps` command now features a new option, `--storage`, to show containers from Buildah, CRI-O and other applications.
- The `podman run` and `podman create` commands now feature a `--sdnotify` option to control the behavior of systemd's sdnotify with containers, enabling improved support for Podman in `Type=notify` units.
- The `podman run` command now features a `--preserve-fds` option to pass file descriptors from the host into the container ([#6458](https://github.com/containers/podman/issues/6458)).
- The `podman run` and `podman create` commands can now create overlay volume mounts, by adding the `:O` option to a bind mount (e.g. `-v /test:/test:O`). Overlay volume mounts will mount a directory into a container from the host and allow changes to it, but not write those changes back to the directory on the host.
- The `podman play kube` command now supports the Socket HostPath type ([#7112](https://github.com/containers/podman/issues/7112)).
- The `podman play kube` command now supports read-only mounts.
- The `podman play kube` command now supports setting labels on pods from Kubernetes metadata labels.
- The `podman play kube` command now supports setting container restart policy ([#7656](https://github.com/containers/podman/issues/7656)).
- The `podman play kube` command now properly handles `HostAlias` entries.
- The `podman generate kube` command now adds entries to `/etc/hosts` from `--host-add` generated YAML as `HostAlias` entries.
- The `podman play kube` and `podman generate kube` commands now properly support `shareProcessNamespace` to share the PID namespace in pods.
- The `podman volume ls` command now supports the `dangling` filter to identify volumes that are dangling (not attached to any container).
- The `podman run` and `podman create` commands now feature a `--umask` option to set the umask of the created container.
- The `podman create` and `podman run` commands now feature a `--tz` option to set the timezone within the container ([#5128](https://github.com/containers/podman/issues/5128)).
- Environment variables for Podman can now be added in the `containers.conf` configuration file.
- The `--mount` option of `podman run` and `podman create` now supports a new mount type, `type=devpts`, to add a `devpts` mount to the container. This is useful for containers that want to mount `/dev/` from the host into the container, but still create a terminal.
- The `--security-opt` flag to `podman run` and `podman create` now supports a new option, `proc-opts`, to specify options for the container's `/proc` filesystem.
- Podman with the `crun` OCI runtime now supports a new option to `podman run` and `podman create`, `--cgroup-conf`, which allows for advanced configuration of cgroups on cgroups v2 systems.
- The `podman create` and `podman run` commands now support a `--override-variant` option, to override the architecture variant of the image that will be pulled and ran.
- A new global option has been added to Podman, `--runtime-flags`, which allows for setting flags to use when the OCI runtime is called.
- The `podman manifest add` command now supports the `--cert-dir`, `--auth-file`, `--creds`, and `--tls-verify` options.

### Security
- This release resolves CVE-2020-14370, in which environment variables could be leaked between containers created using the Varlink API.

### Changes
- Podman will now retry pulling an image 3 times if a pull fails due to network errors.
- The `podman exec` command would previously print error messages (e.g. `exec session exited with non-zero exit code -1`) when the command run exited with a non-0 exit code. It no longer does this. The `podman exec` command will still exit with the same exit code as the command run in the container did.
- Error messages when creating a container or pod with a name that is already in use have been improved.
- For read-only containers running systemd init, Podman creates a tmpfs filesystem at `/run`. This was previously limited to 65k in size and mounted `noexec`, but is now unlimited size and mounted `exec`.
- The `podman system reset` command no longer removes configuration files for rootless Podman.

### Bugfixes
- Fixed a bug where Podman would not add an entry to `/etc/hosts` for a container if it joined another container's network namespace ([#66782](https://github.com/containers/podman/issues/6678)).
- Fixed a bug where `podman save --format oci-dir` saved the image in an incorrect format ([#6544](https://github.com/containers/podman/issues/6544)).
- Fixed a bug where privileged containers would still configure an AppArmor profile.
- Fixed a bug where the `--format` option of `podman system df` was not properly interpreting format codes that included backslashes ([#7149](https://github.com/containers/podman/issues/7149)).
- Fixed a bug where rootless Podman would ignore errors from `newuidmap` and `newgidmap`, even if `/etc/subuid` and `/etc/subgid` contained valid mappings for the user running Podman.
- Fixed a bug where the `podman commit` command did not properly handle single-character image names ([#7114](https://github.com/containers/podman/issues/7114)).
- Fixed a bug where the output of `podman ps --format=json` did not include a `Status` field ([#6980](https://github.com/containers/podman/issues/6980)).
- Fixed a bug where input to the `--log-level` option was no longer case-insensitive.
- Fixed a bug where `podman images` could segfault when an image pull was aborted while incomplete, leaving an image without a manifest ([#7444](https://github.com/containers/podman/issues/7444)).
- Fixed a bug where rootless Podman would try to create the `~/.config` directory when it did not exist, despite not placing any configuration files inside the directory.
- Fixed a bug where the output of `podman system df` was inconsistent based on whether the `-v` option was specified ([#7405](https://github.com/containers/podman/issues/7405)).
- Fixed a bug where `--security-opt apparmor=unconfined` would error if Apparmor was not enabled on the system ([#7545](https://github.com/containers/podman/issues/7545)).
- Fixed a bug where running `podman stop` on multiple containers starting with `--rm` could sometimes cause `no such container` errors ([#7384](https://github.com/containers/podman/issues/7384)).
- Fixed a bug where `podman-remote` would still try to contact the server when displaying help information about subcommands.
- Fixed a bug where the `podman build --logfile` command would segfault.
- Fixed a bug where the `podman generate systemd` command did not properly handle containers which were created with a name given as `--name=$NAME` instead of `--name $NAME` ([#7157](https://github.com/containers/podman/issues/7157)).
- Fixed a bug where the `podman ps` was ignoring the `--latest` flag.
- Fixed a bug where the `podman-remote kill` command would hang when a signal that did not kill the container was specified ([#7135](https://github.com/containers/podman/issues/7135)).
- Fixed a bug where the `--oom-score-adj` option of `podman run` and `podman create` was nonfunctional.
- Fixed a bug where the `--display` option of `podman runlabel` was nonfunctional.
- Fixed a bug where the `podman runlabel` command would not pull images that did not exist locally on the system.
- Fixed a bug where `podman-remote run` would not exit with the correct code with the container was removed by a `podman-remote rm -f` while `podman-remote run` was still running ([#7117](https://github.com/containers/podman/issues/7117)).
- Fixed a bug where the `podman-remote run --rm` command would error attempting to remove containers that had already been removed (e.g. by `podman-remote rm --force`) ([#7340](https://github.com/containers/podman/issues/7340)).
- Fixed a bug where `podman --user` with a numeric user and `podman run --userns=keepid` could create users in `/etc/passwd` in the container that belong to groups without a corresponding entry in `/etc/group` ([#7389](https://github.com/containers/podman/issues/7389)).
- Fixed a bug where `podman run --userns=keepid` could create entries in `/etc/passwd` with a UID that was already in use by another user ([#7503](https://github.com/containers/podman/issues/7503)).
- Fixed a bug where `podman --user` with a numeric user and `podman run --userns=keepid` could create users that could not be logged into ([#7499](https://github.com/containers/podman/issues/7499)).
- Fixed a bug where trying to join another container's user namespace with `--userns container:$ID` would fail ([#7547](https://github.com/containers/podman/issues/7547)).
- Fixed a bug where the `podman play kube` command would trim underscores from container names ([#7020](https://github.com/containers/podman/issues/7020)).
- Fixed a bug where the `podman attach` command would not show output when attaching to a container with a terminal ([#6523](https://github.com/containers/podman/issues/6253)).
- Fixed a bug where the `podman system df` command could be extremely slow when large quantities of images were present ([#7406](https://github.com/containers/podman/issues/7406)).
- Fixed a bug where `podman images -a` would break if any image pulled by digest was present in the store ([#7651](https://github.com/containers/podman/issues/7651)).
- Fixed a bug where the `--mount` option to `podman run` and `podman create` required the `type=` parameter to be passed first ([#7628](https://github.com/containers/podman/issues/7628)).
- Fixed a bug where the `--infra-command` parameter to `podman pod create` was nonfunctional.
- Fixed a bug where `podman auto-update` would fail for any container started with `--pull=always` ([#7407](https://github.com/containers/podman/issues/7407)).
- Fixed a bug where the `podman wait` command would only accept a single argument.
- Fixed a bug where the parsing of the `--volumes-from` option to `podman run` and `podman create` was broken, making it impossible to use multiple mount options at the same time ([#7701](https://github.com/containers/podman/issues/7701)).
- Fixed a bug where the `podman exec` command would not join executed processes to the container's supplemental groups if the container was started with both the `--user` and `--group-add` options.
- Fixed a bug where the `--iidfile` option to `podman-remote build` was nonfunctional.

### API
- The Libpod API version has been bumped to v2.0.0 due to a breaking change in the Image List API.
- Docker-compatible Volume Endpoints (Create, Inspect, List, Remove, Prune) are now available!
- Added an endpoint for generating systemd unit files for containers.
- The `last` parameter to the Libpod container list endpoint now has an alias, `limit` ([#6413](https://github.com/containers/podman/issues/6413)).
- The Libpod image list API new returns timestamps in Unix format, as integer, as opposed to as strings
- The Compat Inspect endpoint for containers now includes port information in NetworkSettings.
- The Compat List endpoint for images now features limited support for the (deprecated) `filter` query parameter ([#6797](https://github.com/containers/podman/issues/6797)).
- Fixed a bug where the Compat Create endpoint for containers was not correctly handling bind mounts.
- Fixed a bug where the Compat Create endpoint for containers would not return a 404 when the requested image was not present.
- Fixed a bug where the Compat Create endpoint for containers did not properly handle Entrypoint and Command from images.
- Fixed a bug where name history information was not properly added in the Libpod Image List endpoint.
- Fixed a bug where the Libpod image search endpoint improperly populated the Description field of responses.
- Added a `noTrunc` option to the Libpod image search endpoint.
- Fixed a bug where the Pod List API would return null, instead of an empty array, when no pods were present ([#7392](https://github.com/containers/podman/issues/7392)).
- Fixed a bug where endpoints that hijacked would do perform the hijack too early, before being ready to send and receive data ([#7195](https://github.com/containers/podman/issues/7195)).
- Fixed a bug where Pod endpoints that can operate on multiple containers at once (e.g. Kill, Pause, Unpause, Stop) would not forward errors from individual containers that failed.
- The Compat List endpoint for networks now supports filtering results ([#7462](https://github.com/containers/podman/issues/7462)).
- Fixed a bug where the Top endpoint for pods would return both a 500 and 404 when run on a nonexistent pod.
- Fixed a bug where Pull endpoints did not stream progress back to the client.
- The Version endpoints (Libpod and Compat) now provide version in a format compatible with Docker.
- All non-hijacking responses to API requests should not include headers with the version of the server.
- Fixed a bug where Libpod and Compat Events endpoints did not send response headers until the first event occurred ([#7263](https://github.com/containers/podman/issues/7263)).
- Fixed a bug where the Build endpoints (Compat and Libpod) did not stream progress to the client.
- Fixed a bug where the Stats endpoints (Compat and Libpod) did not properly handle clients disconnecting.
- Fixed a bug where the Ignore parameter to the Libpod Stop endpoint was not performing properly.
- Fixed a bug where the Compat Logs endpoint for containers did not stream its output in the correct format ([#7196](https://github.com/containers/podman/issues/7196)).

### Misc
- Updated Buildah to v1.16.1
- Updated the containers/storage library to v1.23.5
- Updated the containers/image library to v5.6.0
- Updated the containers/common library to v0.22.0

## 2.0.6
### Bugfixes
- Fixed a bug where running systemd in a container on a cgroups v1 system would fail.
- Fixed a bug where `/etc/passwd` could be re-created every time a container is restarted if the container's `/etc/passwd` did not contain an entry for the user the container was started as.
- Fixed a bug where containers without an `/etc/passwd` file specifying a non-root user would not start.
- Fixed a bug where the `--remote` flag would sometimes not make remote connections and would instead attempt to run Podman locally.

### Misc
- Updated the containers/common library to v0.14.10

## 2.0.5
### Features
- Rootless Podman will now add an entry to `/etc/passwd` for the user who ran Podman if run with `--userns=keep-id`.
- The `podman system connection` command has been reworked to support multiple connections, and re-enabled for use!
- Podman now has a new global flag, `--connection`, to specify a connection to a remote Podman API instance.

### Changes
- Podman's automatic systemd integration (activated by the `--systemd=true` flag, set by default) will now activate for containers using `/usr/local/sbin/init` as their command, instead of just `/usr/sbin/init` and `/sbin/init` (and any path ending in `systemd`).
- Seccomp profiles specified by the `--security-opt seccomp=...` flag to `podman create` and `podman run` will now be honored even if the container was created using `--privileged`.

### Bugfixes
- Fixed a bug where the `podman play kube` would not honor the `hostIP` field for port forwarding ([#5964](https://github.com/containers/podman/issues/5964)).
- Fixed a bug where the `podman generate systemd` command would panic on an invalid restart policy being specified ([#7271](https://github.com/containers/podman/issues/7271)).
- Fixed a bug where the `podman images` command could take a very long time (several minutes) to complete when a large number of images were present.
- Fixed a bug where the `podman logs` command with the `--tail` flag would not work properly when a large amount of output would be printed ([#7230](https://github.com/containers/podman/issues/7230)).
- Fixed a bug where the `podman exec` command with remote Podman would not return a non-zero exit code when the exec session failed to start (e.g. invoking a nonexistent command) ([#6893](https://github.com/containers/podman/issues/6893)).
- Fixed a bug where the `podman load` command with remote Podman would did not honor user-specified tags ([#7124](https://github.com/containers/podman/issues/7124)).
- Fixed a bug where the `podman system service` command, when run as a non-root user by Systemd, did not properly handle the Podman pause process and would not restart properly as a result ([#7180](https://github.com/containers/podman/issues/7180)).
- Fixed a bug where the `--publish` flag to `podman create`, `podman run`, and `podman pod create` did not properly handle a host IP of 0.0.0.0 (attempting to bind to literal 0.0.0.0, instead of all IPs on the system) ([#7104](https://github.com/containers/podman/issues/7014)).
- Fixed a bug where the `podman start --attach` command would not print the container's exit code when the command exited due to the container exiting.
- Fixed a bug where the `podman rm` command with remote Podman would not remove volumes, even if the `--volumes` flag was specified ([#7128](https://github.com/containers/podman/issues/7128)).
- Fixed a bug where the `podman run` command with remote Podman and the `--rm` flag could exit before the container was fully removed.
- Fixed a bug where the `--pod new:...` flag to `podman run` and `podman create` would create a pod that did not share any namespaces.
- Fixed a bug where the `--preserve-fds` flag to `podman run` and `podman exec` could close the wrong file descriptors while trying to close user-provided descriptors after passing them into the container.
- Fixed a bug where default environment variables (`$PATH` and `$TERM`) were not set in containers when not provided by the image.
- Fixed a bug where pod infra containers were not properly unmounted after exiting.
- Fixed a bug where networks created with `podman network create` with an IPv6 subnet did not properly set an IPv6 default route.
- Fixed a bug where the `podman save` command would not work properly when its output was piped to another command ([#7017](https://github.com/containers/podman/issues/7017)).
- Fixed a bug where containers using a systemd init on a cgroups v1 system could leak mounts under `/sys/fs/cgroup/systemd` to the host.
- Fixed a bug where `podman build` would not generate an event on completion ([#7022](https://github.com/containers/podman/issues/7022)).
- Fixed a bug where the `podman history` command with remote Podman printed incorrect creation times for layers ([#7122](https://github.com/containers/podman/issues/7122)).
- Fixed a bug where Podman would not create working directories specified by the container image if they did not exist.
- Fixed a bug where Podman did not clear `CMD` from the container image if the user overrode `ENTRYPOINT` ([#7115](https://github.com/containers/podman/issues/7115)).
- Fixed a bug where error parsing image names were not fully reported (part of the error message containing the exact issue was dropped).
- Fixed a bug where the `podman images` command with remote Podman did not support printing image tags in Go templates supplied to the `--format` flag ([#7123](https://github.com/containers/podman/issues/7123)).
- Fixed a bug where the `podman rmi --force` command would not attempt to unmount containers it was removing, which could cause a failure to remove the image.
- Fixed a bug where the `podman generate systemd --new` command could incorrectly quote arguments to Podman that contained whitespace, leading to nonfunctional unit files ([#7285](https://github.com/containers/podman/issues/7285)).
- Fixed a bug where the `podman version` command did not properly include build time and Git commit.
- Fixed a bug where running systemd in a Podman container on a system that did not use the `systemd` cgroup manager would fail ([#6734](https://github.com/containers/podman/issues/6734)).
- Fixed a bug where capabilities from `--cap-add` were not properly added when a container was started as a non-root user via `--user`.
- Fixed a bug where Pod infra containers were not properly cleaned up when they stopped, causing networking issues ([#7103](https://github.com/containers/podman/issues/7103)).

### API
- Fixed a bug where the libpod and compat Build endpoints did not accept the `application/tar` content type (instead only accepting `application/x-tar`) ([#7185](https://github.com/containers/podman/issues/7185)).
- Fixed a bug where the libpod Exists endpoint would attempt to write a second header in some error conditions ([#7197](https://github.com/containers/podman/issues/7197)).
- Fixed a bug where compat and libpod Network Inspect and Network Remove endpoints would return a 500 instead of 404 when the requested network was not found.
- Added a versioned `_ping` endpoint (e.g. `http://localhost/v1.40/_ping`).
- Fixed a bug where containers started through a systemd-managed instance of the REST API would be shut down when `podman system service` shut down due to its idle timeout ([#7294](https://github.com/containers/podman/issues/7294)).
- Added stronger parameter verification for the libpod Network Create endpoint to ensure subnet mask is a valid value.
- The `Pod` URL parameter to the Libpod Container List endpoint has been deprecated; the information previously gated by the `Pod` boolean will now be included in the response unconditionally.

### Misc
- Updated Buildah to v1.15.1
- Updated containers/image library to v5.5.2

## 2.0.4
### Bugfixes
- Fixed a bug where the output of `podman image search` did not populate the Description field as it was mistakenly assigned to the ID field.
- Fixed a bug where `podman build -` and `podman build` on an HTTP target would fail.
- Fixed a bug where rootless Podman would improperly chown the copied-up contents of anonymous volumes ([#7130](https://github.com/containers/podman/issues/7130)).
- Fixed a bug where Podman would sometimes HTML-escape special characters in its CLI output.
- Fixed a bug where the `podman start --attach --interactive` command would print the container ID of the container attached to when exiting ([#7068](https://github.com/containers/podman/pull/7068)).
- Fixed a bug where `podman run --ipc=host --pid=host` would only set `--pid=host` and not `--ipc=host` ([#7100](https://github.com/containers/podman/issues/7100)).
- Fixed a bug where the `--publish` argument to `podman run`, `podman create` and `podman pod create` would not allow binding the same container port to more than one host port ([#7062](https://github.com/containers/podman/issues/7062)).
- Fixed a bug where incorrect arguments to `podman images --format` could cause Podman to segfault.
- Fixed a bug where `podman rmi --force` on an image ID with more than one name and at least one container using the image would not completely remove containers using the image ([#7153](https://github.com/containers/podman/issues/7153)).
- Fixed a bug where memory usage in bytes and memory use percentage were swapped in the output of `podman stats --format=json`.

### API
- Fixed a bug where the libpod and compat events endpoints would fail if no filters were specified ([#7078](https://github.com/containers/podman/issues/7078)).
- Fixed a bug where the `CgroupVersion` field in responses from the compat Info endpoint was prefixed by "v" (instead of just being "1" or "2", as is documented).

## 2.0.3
### Features
- The `podman search` command now allows wildcards in search terms.
- The `podman play kube` command now supports the `IfNotPresent` pull type.

### Changes
- The `--disable-content-trust` flag has been added to Podman for Docker compatibility. This is a Docker-specific option and has no effect in Podman; it is provided only to ensure command line compatibility for scripts ([#7034](https://github.com/containers/podman/issues/7034)).
- Setting a static IP address or MAC address for rootless containers and pods now causes an error; previously, they were silently ignored.
- The `/sys/dev` folder is now masked in containers to prevent a potential information leak from the host.

### Bugfixes
- Fixed a bug where rootless Podman would select the wrong cgroup manager on cgroups v1 systems where the user in question had an active systemd user session ([#6982](https://github.com/containers/podman/issues/6982)).
- Fixed a bug where systems with Apparmor could not run privileged containers ([#6933](https://github.com/containers/podman/issues/6933)).
- Fixed a bug where ENTRYPOINT and CMD from images were improperly handled by `podman play kube` ([#6995](https://github.com/containers/podman/issues/6995)).
- Fixed a bug where the `--pids-limit` flag to `podman create` and `podman run` was parsed incorrectly and was unusable ([#6908](https://github.com/containers/podman/issues/6908)).
- Fixed a bug where the `podman system df` command would error if untagged images were present ([#7015](https://github.com/containers/podman/issues/7015)).
- Fixed a bug where the `podman images` command would display incorrect tags if a port number was included in the repository.
- Fixed a bug where Podman did not set a default umask and default rlimits ([#6989](https://github.com/containers/podman/issues/6989)).
- Fixed a bug where protocols in port mappings were not recognized unless they were lower-case ([#6948](https://github.com/containers/podman/issues/6948)).
- Fixed a bug where information on pod infra containers was not included in the output of `podman pod inspect`.
- Fixed a bug where Podman's systemd detection (activated by the enabled-by-default `--systemd=true` flag) would not flag a container for systemd mode if systemd was part of the entrypoint, not the command ([#6920](https://github.com/containers/podman/issues/6920)).
- Fixed a bug where `podman start --attach` was not defaulting `--sig-proxy` to true ([#6928](https://github.com/containers/podman/issues/6928)).
- Fixed a bug where `podman inspect` would show an incorrect command (`podman system service`, the command used to start the server) for containers created by a remote Podman client.
- Fixed a bug where the `podman exec` command with the remote client would not print output if the `-t` or `-i` flags where not provided.
- Fixed a bug where some variations of the `--format {{ json . }}` to `podman info` (involving added or removed whitespace) would not be accepted ([#6927](https://github.com/containers/podman/issues/6927)).
- Fixed a bug where Entrypoint could not be cleared at the command line (if unset via `--entrypoint=""`, it would be reset to the image's entrypoint) ([#6935](https://github.com/containers/podman/issues/6935)).

### API
- Fixed a bug where the events endpoints (both libpod and compat) could potentially panic on parsing filters.
- Fixed a bug where the compat Create endpoint for containers did not properly handle Entrypoint and Command.
- Fixed a bug where the Logs endpoint for containers (both libpod and compat) would not properly handle client disconnect, resulting in high CPU usage.
- The type of filters on the compat events endpoint has been adjusted to match Docker's implementation ([#6899](https://github.com/containers/podman/issues/6899)).
- The idle connection counter now properly handles hijacked connections.
- All endpoints that hijack will now properly print headers per RFC 7230 standards.

### Misc
- Updated containers/common to v0.14.6

## 2.0.2
### Changes
- The `podman system connection` command has been temporarily disabled, as it was not functioning as expected.

### Bugfixes
- Fixed a bug where the `podman ps` command would not truncate long container commands, resulting in display issues as the column could become extremely wide (the `--no-trunc` flag can be used to print the full command).
- Fixed a bug where `podman pod` commands operating on multiple containers (e.g. `podman pod stop` and `podman pod kill`) would not print errors from individual containers, but only a warning that some containers had failed.
- Fixed a bug where the `podman system service` command would panic if a connection to the Events endpoint hung up early ([#6805](https://github.com/containers/libpod/issues/6805)).
- Fixed a bug where rootless Podman would create anonymous and named volumes with the wrong owner for containers run with the `--user` directive.
- Fixed a bug where the `TMPDIR` environment variable (used for storing temporary files while pulling images) was not being defaulted (if unset) to `/var/tmp`.
- Fixed a bug where the `--publish` flag to `podman create` and `podman run` required that a host port be specified if an IP address was given ([#6806](https://github.com/containers/libpod/issues/6806)).
- Fixed a bug where in `podman-remote` commands performing an attach (`podman run`, `podman attach`, `podman start --attach`, `podman exec`) did not properly configure the terminal on Windows.
- Fixed a bug where the `--remote` flag to Podman required an argument, despite being a boolean ([#6704](https://github.com/containers/libpod/issues/6704)).
- Fixed a bug where the `podman generate systemd --new` command could generate incorrect unit files for a pod if a container in the pod was created using the `--pod=...` flag (with an =, instead of a space, before the pod ID) ([#6766](https://github.com/containers/libpod/issues/6766)).
- Fixed a bug where `NPROC` and `NOFILE` rlimits could be improperly set for rootless Podman containers, causing them to fail to start.
- Fixed a bug where `podman mount` as rootless did not error (the `podman mount` command cannot be run rootless unless it is run inside a `podman unshare` shell).
- Fixed a bug where in some cases a race in events handling code could cause error messages related to retrieving events to be lost.

### API
- Fixed a bug where the timestamp format for Libpod image list endpoint was incorrect - the format has been switched to Unix time.
- Fixed a bug where the compatibility Create endpoint did not handle empty entrypoints properly.
- Fixed a bug where the compatibility network remove endpoint would improperly handle errors where the network was not found.
- Fixed a bug where containers would be created with improper permissions because of a umask issue ([#6787](https://github.com/containers/libpod/issues/6787)).

## 2.0.1
### Changes
- The `podman system connection` command was mistakenly omitted from the 2.0 release, and has been included here.
- The `podman ps --format=json` command once again includes container's creation time in a human-readable format in the `CreatedAt` key.
- The `podman inspect` commands on containers now displays forwarded ports in a format compatible with `docker inspect`.
- The `--log-level=debug` flag to `podman run` and `podman exec` will enable syslog for exit commands, ensuring that debug logs are collected for these otherwise-unlogged commands.

### Bugfixes
- Fixed a bug where `podman build` did not properly handle the `--http-proxy` and `--cgroup-manager` flags.
- Fixed a bug where error messages related to a missing or inaccessible `/etc/subuid` or `/etc/subgid` file were very unclear ([#6572](https://github.com/containers/libpod/issues/6572)).
- Fixed a bug where the `podman logs --follow` command would not stop when the container being followed exited.
- Fixed a bug where the `--privileged` flag had mistakenly been marked as conflicting with `--group-add` and `--security-opt`.
- Fixed a bug where the `PODMAN_USERNS` environment variable was not being honored ([#6705](https://github.com/containers/libpod/issues/6705)).
- Fixed a bug where the `podman image load` command would require one argument be passed, when no arguments is also valid ([#6718](https://github.com/containers/libpod/issues/6718)).
- Fixed a bug where the bash completions did not include the `podman network` command and its subcommands.
- Fixed a bug where the mount command would not work inside of rootless containers ([#6735](https://github.com/containers/libpod/issues/6735)).
- Fixed a bug where SSH agent authentication support was not properly working in the `podman-remote` and `podman --remote` commands.
- Fixed a bug where the `podman untag` command was not erroring when no matching image was found.
- Fixed a bug where stop signal for containers was not being set properly if not explicitly provided.
- Fixed a bug where the `podman ps` command was not showing port mappings for containers which share a network namespace with another container (e.g. are part of a pod).
- Fixed a bug where the `--remote` flag could unintentionally be forwarded into containers when using `podman-remote`.
- Fixed a bug where unit files generated for pods by `podman generate systemd` would not allow individual containers to be restarted ([#6770](https://github.com/containers/libpod/issues/6770)).
- Fixed a bug where the `podman run` and `podman create` commands did not support all transports that `podman pull` does ([#6744](https://github.com/containers/libpod/issues/6744)).
- Fixed a bug where the `label` option to `--security-opt` would only be shown once in `podman inspect`, even if provided multiple times.

### API
- Fixed a bug where network endpoint URLs in the compatibility API were mistakenly suffixed with `/json`.
- Fixed a bug where the Libpod volume creation endpoint returned 200 instead of 201 on success.

### Misc
- Updated containers/common to v0.14.3

## 2.0.0
### Features
- The REST API and `podman system service` are no longer experimental, and ready for use!
- The Podman command now supports remotely connections via the REST API using the `--remote` flag.
- The Podman remote client has been entirely rewritten to use the HTTP API instead of Varlink.
- The `podman system connection` command has been added to allow configuring the endpoint that `podman-remote` and `podman --remote` will connect to.
- The `podman generate systemd` command now supports the `--new` flag when used with pods, allowing portable services for pods to be created.
- The `podman play kube` command now supports running Kubernetes Deployment YAML.
- The `podman exec` command now supports the `--detach` flag to run commands in the container in the background.
- The `-p` flag to `podman run` and `podman create` now supports forwarding ports to IPv6 addresses.
- The `podman run`, `podman create` and `podman pod create` command now support a `--replace` flag to remove and replace any existing container (or, for `pod create`, pod) with the same name
- The `--restart-policy` flag to `podman run` and `podman create` now supports the `unless-stopped` restart policy.
- The `--log-driver` flag to `podman run` and `podman create` now supports the `none` driver, which does not log the container's output.
- The `--mount` flag to `podman run` and `podman create` now accepts `readonly` option as an alias to `ro`.
- The `podman generate systemd` command now supports the `--container-prefix`, `--pod-prefix`, and `--separator` arguments to control the name of generated unit files.
- The `podman network ls` command now supports the `--filter` flag to filter results.
- The `podman auto-update` command now supports specifying an authfile to use when pulling new images on a per-container basis using the `io.containers.autoupdate.authfile` label.

### Changes
- Varlink support, including the `podman varlink` command, is deprecated and will be removed in the next release.
- As part of the implementation of the REST API, JSON output for some commands (`podman ps`, `podman images` most notably) has changed.
- Named and anonymous volumes and `tmpfs` filesystems added to containers are no longer mounted `noexec` by default.

### Bugfixes
- Fixed a bug where the `podman exec` command would log to journald when run in containers logged to journald ([#6555](https://github.com/containers/podman/issues/6555)).
- Fixed a bug where the `podman auto-update` command would not preserve the OS and architecture of the original image when pulling a replacement ([#6613](https://github.com/containers/podman/issues/6613)).
- Fixed a bug where the `podman cp` command could create an extra `merged` directory when copying into an existing directory ([#6596](https://github.com/containers/podman/issues/6596)).
- Fixed a bug where the `podman pod stats` command would crash on pods run with `--network=host` ([#5652](https://github.com/containers/podman/issues/5652)).
- Fixed a bug where containers logs written to journald did not include the name of the container.
- Fixed a bug where the `podman network inspect` and `podman network rm` commands did not properly handle non-default CNI configuration paths ([#6212](https://github.com/containers/podman/issues/6212)).
- Fixed a bug where Podman did not properly remove containers when using the Kata containers OCI runtime.
- Fixed a bug where `podman inspect` would sometimes incorrectly report the network mode of containers started with `--net=none`.
- Podman is now better able to deal with cases where `conmon` is killed before the container it is monitoring.

### Misc
- The default Podman CNI configuration now sets `HairpinMode` to allow communication between containers by connecting to a forwarded port on the host.
- Updated Buildah to v1.15.0
- Updated containers/storage to v1.20.2
- Updated containers/image to v5.5.1
- Updated containers/common to v0.14.0

## 1.9.3
### Bugfixes
- Fixed a bug where, on FIPS enabled hosts, FIPS mode secrets were not properly mounted into containers
- Fixed a bug where builds run over Varlink would hang ([#6237](https://github.com/containers/podman/issues/6237))

### Misc
- Named volumes and tmpfs filesystems will no longer default to mounting `noexec` for improved compatibility with Docker
- Updated Buildah to v1.14.9

## 1.9.2
### Bugfixes
- Fixed a bug where `podman save` would fail when the target image was specified by digest ([#5234](https://github.com/containers/podman/issues/5234))
- Fixed a bug where rootless containers with ports forwarded to them could panic and dump core due to a concurrency issue ([#6018](https://github.com/containers/podman/issues/6018))
- Fixed a bug where rootless Podman could race when opening the rootless user namespace, resulting in commands failing to run
- Fixed a bug where HTTP proxy environment variables forwarded into the container by the `--http-proxy` flag could not be overridden by `--env` or `--env-file` ([#6017](https://github.com/containers/podman/issues/6017))
- Fixed a bug where rootless Podman was setting resource limits on cgroups v2 systems that were not using systemd-managed cgroups (and thus did not support resource limits), resulting in containers failing to start

### Misc
- Rootless containers will now automatically set their ulimits to the maximum allowed for the user running the container, to match the behavior of containers run as root
- Packages managed by the core Podman team will no longer include a default `libpod.conf`, instead defaulting to `containers.conf`. The default libpod.conf will remain available in the GitHub repository until the release of Podman 2.0
- The default Podman CNI network configuration now sets HairpinMode to allow containers to access other containers via ports published on the host
- Updated containers/common to v0.8.4

## 1.9.1
### Bugfixes
- Fixed a bug where healthchecks could become nonfunctional if container log paths were manually set with `--log-path` and multiple container logs were placed in the same directory ([#5915](https://github.com/containers/podman/issues/5915))
- Fixed a bug where rootless Podman could, when using an older `libpod.conf`, print numerous warning messages about an invalid CGroup manager config
- Fixed a bug where rootless Podman would sometimes fail to close the rootless user namespace when joining it ([#5873](https://github.com/containers/podman/issues/5873))

### Misc
- Updated containers/common to v0.8.2

## 1.9.0
### Features
- Experimental support has been added for `podman run --userns=auto`, which automatically allocates a unique UID and GID range for the new container's user namespace
- The `podman play kube` command now has a `--network` flag to place the created pod in one or more CNI networks
- The `podman commit` command now supports an `--iidfile` flag to write the ID of the committed image to a file
- Initial support for the new `containers.conf` configuration file has been added. `containers.conf` allows for much more detailed configuration of some Podman functionality

### Changes
- There has been a major cleanup of the `podman info` command resulting in breaking changes. Many fields have been renamed to better suit usage with APIv2
- All uses of the `--timeout` flag have been switched to prefer the alternative `--time`. The `--timeout` flag will continue to work, but man pages and `--help` will use the `--time` flag instead

### Bugfixes
- Fixed a bug where some volume mounts from the host would sometimes not properly determine the flags they should use when mounting
- Fixed a bug where Podman was not propagating `$PATH` to Conmon and the OCI runtime, causing issues for some OCI runtimes that required it
- Fixed a bug where rootless Podman would print error messages about missing support for systemd cgroups when run in a container with no cgroup support ([#5488](https://github.com/containers/podman/issues/5488))
- Fixed a bug where `podman play kube` would not properly handle container-only port mappings ([#5610](https://github.com/containers/podman/issues/5610))
- Fixed a bug where the `podman container prune` command was not pruning containers in the `created` and `configured` states
- Fixed a bug where Podman was not properly removing CNI IP address allocations after a reboot ([#5433](https://github.com/containers/podman/issues/5433))
- Fixed a bug where Podman was not properly applying the default Seccomp profile when `--security-opt` was not given at the command line

### HTTP API
- Many Libpod API endpoints have been added, including `Changes`, `Checkpoint`, `Init`, and `Restore`
- Resolved issues where the `podman system service` command would time out and exit while there were still active connections
- Stability overall has greatly improved as we prepare the API for a beta release soon with Podman 2.0

### Misc
- The default infra image for pods has been upgraded to `k8s.gcr.io/pause:3.2` (from 3.1) to address a bug in the architecture metadata for non-AMD64 images
- The `slirp4netns` networking utility in rootless Podman now uses Seccomp filtering where available for improved security
- Updated Buildah to v1.14.8
- Updated containers/storage to v1.18.2
- Updated containers/image to v5.4.3
- Updated containers/common to v0.8.1

## 1.8.2
### Features
- Initial support for automatically updating containers managed via Systemd unit files has been merged. This allows containers to automatically upgrade if a newer version of their image becomes available

### Bugfixes
- Fixed a bug where unit files generated by `podman generate systemd --new` would not force containers to detach, causing the unit to time out when trying to start
- Fixed a bug where `podman system reset` could delete important system directories if run as rootless on installations created by older Podman ([#4831](https://github.com/containers/podman/issues/4831))
- Fixed a bug where image built by `podman build` would not properly set the OS and Architecture they were built with ([#5503](https://github.com/containers/podman/issues/5503))
- Fixed a bug where attached `podman run` with `--sig-proxy` enabled (the default), when built with Go 1.14, would repeatedly send signal 23 to the process in the container and could generate errors when the container stopped ([#5483](https://github.com/containers/podman/issues/5483))
- Fixed a bug where rootless `podman run` commands could hang when forwarding ports
- Fixed a bug where rootless Podman would not work when `/proc` was mounted with the `hidepid` option set
- Fixed a bug where the `podman system service` command would use large amounts of CPU when `--timeout` was set to 0 ([#5531](https://github.com/containers/podman/issues/5531))

### HTTP API
- Initial support for Libpod endpoints related to creating and operating on image manifest lists has been added
- The Libpod Healthcheck and Events API endpoints are now supported
- The Swagger endpoint can now handle cases where no Swagger documentation has been generated

### Misc
- Updated Buildah to v1.14.3
- Updated containers/storage to v1.16.5
- Several performance improvements have been made to creating containers, which should somewhat improve the performance of `podman create` and `podman run`

## 1.8.1
### Features
- Many networking-related flags have been added to `podman pod create` to enable customization of pod networks, including `--add-host`, `--dns`, `--dns-opt`, `--dns-search`, `--ip`, `--mac-address`, `--network`, and `--no-hosts`
- The `podman ps --format=json` command now includes the ID of the image containers were created with
- The `podman run` and `podman create` commands now feature an `--rmi` flag to remove the image the container was using after it exits (if no other containers are using said image) ([#4628](https://github.com/containers/podman/issues/4628))
- The `podman create` and `podman run` commands now support the `--device-cgroup-rule` flag ([#4876](https://github.com/containers/podman/issues/4876))
- While the HTTP API remains in alpha, many fixes and additions have landed. These are documented in a separate subsection below
- The `podman create` and `podman run` commands now feature a `--no-healthcheck` flag to disable healthchecks for a container ([#5299](https://github.com/containers/podman/issues/5299))
- Containers now recognize the `io.containers.capabilities` label, which specifies a list of capabilities required by the image to run. These capabilities will be used as long as they are more restrictive than the default capabilities used
- YAML produced by the `podman generate kube` command now includes SELinux configuration passed into the container via `--security-opt label=...` ([#4950](https://github.com/containers/podman/issues/4950))

### Bugfixes
- Fixed CVE-2020-1726, a security issue where volumes manually populated before first being mounted into a container could have those contents overwritten on first being mounted into a container
- Fixed a bug where Podman containers with user namespaces in CNI networks with the DNS plugin enabled would not have the DNS plugin's nameserver added to their `resolv.conf` ([#5256](https://github.com/containers/podman/issues/5256))
- Fixed a bug where trailing `/` characters in image volume definitions could cause them to not be overridden by a user-specified mount at the same location ([#5219](https://github.com/containers/podman/issues/5219))
- Fixed a bug where the `label` option in `libpod.conf`, used to disable SELinux by default, was not being respected ([#5087](https://github.com/containers/podman/issues/5087))
- Fixed a bug where the `podman login` and `podman logout` commands required the registry to log into be specified ([#5146](https://github.com/containers/podman/issues/5146))
- Fixed a bug where detached rootless Podman containers could not forward ports ([#5167](https://github.com/containers/podman/issues/5167))
- Fixed a bug where rootless Podman could fail to run if the pause process had died
- Fixed a bug where Podman ignored labels that were specified with only a key and no value ([#3854](https://github.com/containers/podman/issues/3854))
- Fixed a bug where Podman would fail to create named volumes when the backing filesystem did not support SELinux labelling ([#5200](https://github.com/containers/podman/issues/5200))
- Fixed a bug where `--detach-keys=""` would not disable detaching from a container ([#5166](https://github.com/containers/podman/issues/5166))
- Fixed a bug where the `podman ps` command was too aggressive when filtering containers and would force `--all` on in too many situations
- Fixed a bug where the `podman play kube` command was ignoring image configuration, including volumes, working directory, labels, and stop signal ([#5174](https://github.com/containers/podman/issues/5174))
- Fixed a bug where the `Created` and `CreatedTime` fields in `podman images --format=json` were misnamed, which also broke Go template output for those fields ([#5110](https://github.com/containers/podman/issues/5110))
- Fixed a bug where rootless Podman containers with ports forwarded could hang when started ([#5182](https://github.com/containers/podman/issues/5182))
- Fixed a bug where `podman pull` could fail to parse registry names including port numbers
- Fixed a bug where Podman would incorrectly attempt to validate image OS and architecture when starting containers
- Fixed a bug where Bash completion for `podman build -f` would not list available files that could be built ([#3878](https://github.com/containers/podman/issues/3878))
- Fixed a bug where `podman commit --change` would perform incorrect validation, resulting in valid changes being rejected ([#5148](https://github.com/containers/podman/issues/5148))
- Fixed a bug where `podman logs --tail` could take large amounts of memory when the log file for a container was large ([#5131](https://github.com/containers/podman/issues/5131))
- Fixed a bug where Podman would sometimes incorrectly generate firewall rules on systems using `firewalld`
- Fixed a bug where the `podman inspect` command would not display network information for containers properly if a container joined multiple CNI networks ([#4907](https://github.com/containers/podman/issues/4907))
- Fixed a bug where the `--uts` flag to `podman create` and `podman run` would only allow specifying containers by full ID ([#5289](https://github.com/containers/podman/issues/5289))
- Fixed a bug where rootless Podman could segfault when passed a large number of file descriptors
- Fixed a bug where the `podman port` command was incorrectly interpreting additional arguments as container names, instead of port numbers
- Fixed a bug where units created by `podman generate systemd` did not depend on network targets, and so could start before the system network was ready ([#4130](https://github.com/containers/podman/issues/4130))
- Fixed a bug where exec sessions in containers which did not specify a user would not inherit supplemental groups added to the container via `--group-add`
- Fixed a bug where Podman would not respect the `$TMPDIR` environment variable for placing large temporary files during some operations (e.g. `podman pull`) ([#5411](https://github.com/containers/podman/issues/5411))

### HTTP API
- Initial support for secure connections to servers via SSH tunneling has been added
- Initial support for the libpod `create` and `logs` endpoints for containers has been added
- Added a `/swagger/` endpoint to serve API documentation
- The `json` endpoint for containers has received many fixes
- Filtering images and containers has been greatly improved, with many bugs fixed and documentation improved
- Image creation endpoints (commit, pull, etc) have seen many fixes
- Server timeout has been fixed so that long operations will no longer trigger the timeout and shut the server down
- The `stats` endpoint for containers has seen major fixes and now provides accurate output
- Handling the HTTP 304 status code has been fixed for all endpoints
- Many fixes have been made to API documentation to ensure it matches the code

### Misc
- Updated vendored Buildah to v1.14.2
- Updated vendored containers/storage to v1.16.2
- The `Created` field to `podman images --format=json` has been renamed to `CreatedSince` as part of the fix for ([#5110](https://github.com/containers/podman/issues/5110)). Go templates using the old name should still work
- The `CreatedTime` field to `podman images --format=json` has been renamed to `CreatedAt` as part of the fix for ([#5110](https://github.com/containers/podman/issues/5110)). Go templates using the old name should still work
- The `before` filter to `podman images` has been renamed to `since` for Docker compatibility. Using `before` will still work, but documentation has been changed to use the new `since` filter
- Using the `--password` flag to `podman login` now warns that passwords are being passed in plaintext
- Some common cases where Podman would deadlock have been fixed to warn the user that `podman system renumber` must be run to resolve the deadlock

## 1.8.0
### Features
- The `podman system service` command has been added, providing a preview of Podman's new Docker-compatible API. This API is still very new, and not yet ready for production use, but is available for early testing
- Rootless Podman now uses Rootlesskit for port forwarding, which should greatly improve performance and capabilities
- The `podman untag` command has been added to remove tags from images without deleting them
- The `podman inspect` command on images now displays previous names they used
- The `podman generate systemd` command now supports a `--new` option to generate service files that create and run new containers instead of managing existing containers
- Support for `--log-opt tag=` to set logging tags has been added to the `journald` log driver
- Added support for using Seccomp profiles embedded in images for `podman run` and `podman create` via the new `--seccomp-policy` CLI flag ([#4806](https://github.com/containers/podman/pull/4806))
- The `podman play kube` command now honors pull policy ([#4880](https://github.com/containers/podman/issues/4880))

### Bugfixes
- Fixed a bug where the `podman cp` command would not copy the contents of directories when paths ending in `/.` were given ([#4717](https://github.com/containers/podman/issues/4717))
- Fixed a bug where the `podman play kube` command did not properly locate Seccomp profiles specified relative to localhost ([#4555](https://github.com/containers/podman/issues/4555))
- Fixed a bug where the `podman info` command for remote Podman did not show registry information ([#4793](https://github.com/containers/podman/issues/4793))
- Fixed a bug where the `podman exec` command did not support having input piped into it ([#3302](https://github.com/containers/podman/issues/3302))
- Fixed a bug where the `podman cp` command with rootless Podman on CGroups v2 systems did not properly determine if the container could be paused while copying ([#4813](https://github.com/containers/podman/issues/4813))
- Fixed a bug where the `podman container prune --force` command could possible remove running containers if they were started while the command was running ([#4844](https://github.com/containers/podman/issues/4844))
- Fixed a bug where Podman, when run as root, would not properly configure `slirp4netns` networking when requested ([#4853](https://github.com/containers/podman/pull/4853))
- Fixed a bug where `podman run --userns=keep-id` did not work when the user had a UID over 65535 ([#4838](https://github.com/containers/podman/issues/4838))
- Fixed a bug where rootless `podman run` and `podman create` with the `--userns=keep-id` option could change permissions on `/run/user/$UID` and break KDE ([#4846](https://github.com/containers/podman/issues/4846))
- Fixed a bug where rootless Podman could not be run in a systemd service on systems using CGroups v2 ([#4833](https://github.com/containers/podman/issues/4833))
- Fixed a bug where `podman inspect` would show CPUShares as 0, instead of the default (1024), when it was not explicitly set ([#4822](https://github.com/containers/podman/issues/4822))
- Fixed a bug where `podman-remote push` would segfault ([#4706](https://github.com/containers/podman/issues/4706))
- Fixed a bug where image healthchecks were not shown in the output of `podman inspect` ([#4799](https://github.com/containers/podman/issues/4799))
- Fixed a bug where named volumes created with containers from pre-1.6.3 releases of Podman would be autoremoved with their containers if the `--rm` flag was given, even if they were given names ([#5009](https://github.com/containers/podman/issues/5009))
- Fixed a bug where `podman history` was not computing image sizes correctly ([#4916](https://github.com/containers/podman/issues/4916))
- Fixed a bug where Podman would not error on invalid values to the `--sort` flag to `podman images`
- Fixed a bug where providing a name for the image made by `podman commit` was mandatory, not optional as it should be ([#5027](https://github.com/containers/podman/issues/5027))
- Fixed a bug where the remote Podman client would append an extra `"` to `%PATH` ([#4335](https://github.com/containers/podman/issues/4335))
- Fixed a bug where the `podman build` command would sometimes ignore the `-f` option and build the wrong Containerfile
- Fixed a bug where the `podman ps --filter` command would only filter running containers, instead of all containers, if `--all` was not passed ([#5050](https://github.com/containers/podman/issues/5050))
- Fixed a bug where the `podman load` command on compressed images would leave an extra copy on disk
- Fixed a bug where the `podman restart` command would not properly clean up the network, causing it to function differently from `podman stop; podman start` ([#5051](https://github.com/containers/podman/issues/5051))
- Fixed a bug where setting the `--memory-swap` flag to `podman create` and `podman run` to `-1` (to indicate unlimited) was not supported ([#5091](https://github.com/containers/podman/issues/5091))

### Misc
- Initial work on version 2 of the Podman remote API has been merged, but is still in an alpha state and not ready for use. Read more [here](https://podman.io/releases/2020/01/17/podman-new-api.html)
- Many formatting corrections have been made to the manpages
- The changes to address ([#5009](https://github.com/containers/podman/issues/5009)) may cause anonymous volumes created by Podman versions 1.6.3 to 1.7.0 to not be removed when their container is removed
- Updated vendored Buildah to v1.13.1
- Updated vendored containers/storage to v1.15.8
- Updated vendored containers/image to v5.2.0

## 1.7.0
### Features
- Added support for setting a static MAC address for containers
- Added support for creating `macvlan` networks with `podman network create`, allowing Podman containers to be attached directly to networks the host is connected to
- The `podman image prune` and `podman container prune` commands now support the `--filter` flag to filter what will be pruned, and now prompts for confirmation when run without `--force` ([#4410](https://github.com/containers/podman/issues/4410) and [#4411](https://github.com/containers/podman/issues/4411))
- Podman now creates CGroup namespaces by default on systems using CGroups v2 ([#4363](https://github.com/containers/podman/issues/4363))
- Added the `podman system reset` command to remove all Podman files and perform a factory reset of the Podman installation
- Added the `--history` flag to `podman images` to display previous names used by images ([#4566](https://github.com/containers/podman/issues/4566))
- Added the `--ignore` flag to `podman rm` and `podman stop` to not error when requested containers no longer exist
- Added the `--cidfile` flag to `podman rm` and `podman stop` to read the IDs of containers to be removed or stopped from a file
- The `podman play kube` command now honors Seccomp annotations ([#3111](https://github.com/containers/podman/issues/3111))
- The `podman play kube` command now honors `RunAsUser`, `RunAsGroup`, and `selinuxOptions`
- The output format of the `podman version` command has been changed to better match `docker version` when using the `--format` flag
- Rootless Podman will no longer initialize containers/storage twice, removing a potential deadlock preventing Podman commands from running while an image was being pulled ([#4591](https://github.com/containers/podman/issues/4591))
- Added `tmpcopyup` and `notmpcopyup` options to the `--tmpfs` and `--mount type=tmpfs` flags to `podman create` and `podman run` to control whether the content of directories are copied into tmpfs filesystems mounted over them
- Added support for disabling detaching from containers by setting empty detach keys via `--detach-keys=""`
- The `podman build` command now supports the `--pull` and `--pull-never` flags to control when images are pulled during a build
- The `podman ps -p` command now shows the name of the pod as well as its ID ([#4703](https://github.com/containers/podman/issues/4703))
- The `podman inspect` command on containers will now display the command used to create the container
- The `podman info` command now displays information on registry mirrors ([#4553](https://github.com/containers/podman/issues/4553))

### Bugfixes
- Fixed a bug where Podman would use an incorrect runtime directory as root, causing state to be deleted after root logged out and making Podman in systemd services not function properly
- Fixed a bug where the `--change` flag to `podman import` and `podman commit` was not being parsed properly in many cases
- Fixed a bug where detach keys specified in `libpod.conf` were not used by the `podman attach` and `podman exec` commands, which always used the global default `ctrl-p,ctrl-q` key combination ([#4556](https://github.com/containers/podman/issues/4556))
- Fixed a bug where rootless Podman was not able to run `podman pod stats` even on CGroups v2 enabled systems ([#4634](https://github.com/containers/podman/issues/4634))
- Fixed a bug where rootless Podman would fail on kernels without the `renameat2` syscall ([#4570](https://github.com/containers/podman/issues/4570))
- Fixed a bug where containers with chained network namespace dependencies (IE, container A using `--net container=B` and container B using `--net container=C`) would not properly mount `/etc/hosts` and `/etc/resolv.conf` into the container ([#4626](https://github.com/containers/podman/issues/4626))
- Fixed a bug where `podman run` with the `--rm` flag and without `-d` could, when run in the background, throw a 'container does not exist' error when attempting to remove the container after it exited
- Fixed a bug where named volume locks were not properly reacquired after a reboot, potentially leading to deadlocks when trying to start containers using the volume ([#4605](https://github.com/containers/podman/issues/4605) and [#4621](https://github.com/containers/podman/issues/4621))
- Fixed a bug where Podman could not completely remove containers if sent SIGKILL during removal, leaving the container name unusable without the `podman rm --storage` command to complete removal ([#3906](https://github.com/containers/podman/issues/3906))
- Fixed a bug where checkpointing containers started with `--rm` was allowed when `--export` was not specified (the container, and checkpoint, would be removed after checkpointing was complete by `--rm`) ([#3774](https://github.com/containers/podman/issues/3774))
- Fixed a bug where the `podman pod prune` command would fail if containers were present in the pods and the `--force` flag was not passed ([#4346](https://github.com/containers/podman/issues/4346))
- Fixed a bug where containers could not set a static IP or static MAC address if they joined a non-default CNI network ([#4500](https://github.com/containers/podman/issues/4500))
- Fixed a bug where `podman system renumber` would always throw an error if a container was mounted when it was run
- Fixed a bug where `podman container restore` would fail with containers using a user namespace
- Fixed a bug where rootless Podman would attempt to use the journald events backend even on systems without systemd installed
- Fixed a bug where `podman history` would sometimes not properly identify the IDs of layers in an image ([#3359](https://github.com/containers/podman/issues/3359))
- Fixed a bug where containers could not be restarted when Conmon v2.0.3 or later was used
- Fixed a bug where Podman did not check image OS and Architecture against the host when starting a container
- Fixed a bug where containers in pods did not function properly with the Kata OCI runtime ([#4353](https://github.com/containers/podman/issues/4353))
- Fixed a bug where `podman info --format '{{ json . }}' would not produce JSON output ([#4391](https://github.com/containers/podman/issues/4391))
- Fixed a bug where Podman would not verify if files passed to `--authfile` existed ([#4328](https://github.com/containers/podman/issues/4328))
- Fixed a bug where `podman images --digest` would not always print digests when they were available
- Fixed a bug where rootless `podman run` could hang due to a race with reading and writing events
- Fixed a bug where rootless Podman would print warning-level logs despite not be instructed to do so ([#4456](https://github.com/containers/podman/issues/4456))
- Fixed a bug where `podman pull` would attempt to fetch from remote registries when pulling an unqualified image using the `docker-daemon` transport ([#4434](https://github.com/containers/podman/issues/4434))
- Fixed a bug where `podman cp` would not work if STDIN was a pipe
- Fixed a bug where `podman exec` could stop accepting input if anything was typed between the command being run and the exec session starting ([#4397](https://github.com/containers/podman/issues/4397))
- Fixed a bug where `podman logs --tail 0` would print all lines of a container's logs, instead of no lines ([#4396](https://github.com/containers/podman/issues/4396))
- Fixed a bug where the timeout for `slirp4netns` was incorrectly set, resulting in an extremely long timeout ([#4344](https://github.com/containers/podman/issues/4344))
- Fixed a bug where the `podman stats` command would print CPU utilizations figures incorrectly ([#4409](https://github.com/containers/podman/issues/4409))
- Fixed a bug where the `podman inspect --size` command would not print the size of the container's read/write layer if the size was 0 ([#4744](https://github.com/containers/podman/issues/4744))
- Fixed a bug where the `podman kill` command was not properly validating signals before use ([#4746](https://github.com/containers/podman/issues/4746))
- Fixed a bug where the `--quiet` and `--format` flags to `podman ps` could not be used at the same time
- Fixed a bug where the `podman stop` command was not stopping exec sessions when a container was created without a PID namespace (`--pid=host`)
- Fixed a bug where the `podman pod rm --force` command was not removing anonymous volumes for containers that were removed
- Fixed a bug where the `podman checkpoint` command would not export all changes to the root filesystem of the container if performed more than once on the same container ([#4606](https://github.com/containers/podman/issues/4606))
- Fixed a bug where containers started with `--rm` would not be automatically removed on being stopped if an exec session was running inside the container ([#4666](https://github.com/containers/podman/issues/4666))

### Misc
- The fixes to runtime directory path as root can cause strange behavior if an upgrade is performed while containers are running
- Updated vendored Buildah to v1.12.0
- Updated vendored containers/storage library to v1.15.4
- Updated vendored containers/image library to v5.1.0
- Kata Containers runtimes (`kata-runtime`, `kata-qemu`, and `kata-fc`) are now present in the default libpod.conf, but will not be available unless Kata containers is installed on the system
- Podman previously did not allow the creation of containers with a memory limit lower than 4MB. This restriction has been removed, as the `crun` runtime can create containers with significantly less memory

## 1.6.3
### Features
- Handling of the `libpod.conf` configuration file has seen major changes. Most significantly, rootless users will no longer automatically receive a complete configuration file when they first use Podman, and will instead only receive differences from the global configuration.
- Initial support for the CNI DNS plugin, which allows containers to resolve the IPs of other containers via DNS name, has been added
- Podman now supports anonymous named volumes, created by specifying only a destination to the `-v` flag to the `podman create` and `podman run` commands
- Named volumes now support `uid` and `gid` options in `--opt o=...` to set UID and GID of the created volume

### Bugfixes
- Fixed a bug where the `podman start` command would print container ID, instead of name, when starting containers given their name
- Fixed a bug where named volumes with options did not properly detect issues with mounting the volume, leading to an inconsistent state ([#4303](https://github.com/containers/podman/issues/4303))
- Fixed a bug where incorrect Seccomp profiles were used in containers generated by `podman play kube`
- Fixed a bug where processes started by `podman exec` would have the wrong SELinux label in some circumstances ([#4361](https://github.com/containers/podman/issues/4361))
- Fixed a bug where error messages from `slirp4netns` would be lost
- Fixed a bug where `podman run --network=$NAME` would not throw an error in rootless Podman, where CNI networks are not supported
- Fixed a bug where `podman network create` would throw confusing errors when trying to create a volume with a name that already exists
- Fixed a bug where Podman would not error if the `systemd` CGroup manager was specified, but systemd could not be contacted over DBus
- Fixed a bug where image volumes were mounted `noexec` ([#4318](https://github.com/containers/podman/issues/4318))
- Fixed a bug where the `podman stats` command required the name of a container to be given, instead of showing all containers when no container was specified ([#4274](https://github.com/containers/podman/issues/4274))
- Fixed a bug where the `podman volume inspect` command would not show the options that named volumes were created with
- Fixed a bug where custom storage configuration was not written to `storage.conf` at time of first creation for rootless Podman ([#2659](https://github.com/containers/podman/issues/2659))
- Fixed a bug where remote Podman did not support shell redirection of container output

### Misc
- Updated vendored containers/image library to v5.0
- Initial support for images using manifest lists has been added, though commands for directly interacting with manifests are still missing
- Support for pushing to and pulling from OSTree has been removed due to deprecation in the containers/image library
- Rootless Podman no longer enables linger on systems with systemd as init by default. As such, containers will now be killed when the user who ran them logs out, unless linger is explicitly enabled using [loginctl](https://www.freedesktop.org/software/systemd/man/loginctl.html)
- Podman will now check the version of `conmon` that is in use to ensure it is sufficient

## 1.6.2
### Features
- Added a `--runtime` flag to `podman system migrate` to allow the OCI runtime for all containers to be reset, to ease transition to the `crun` runtime on CGroups V2 systems until `runc` gains full support
- The `podman rm` command can now remove containers in broken states which previously could not be removed
- The `podman info` command, when run without root, now shows information on UID and GID mappings in the rootless user namespace
- Added `podman build --squash-all` flag, which squashes all layers (including those of the base image) into one layer
- The `--systemd` flag to `podman run` and `podman create` now accepts a string argument and allows a new value, `always`, which forces systemd support without checking if the container entrypoint is systemd

### Bugfixes
- Fixed a bug where the `podman top` command did not work on systems using CGroups V2 ([#4192](https://github.com/containers/podman/issues/4192))
- Fixed a bug where rootless Podman could double-close a file, leading to a panic
- Fixed a bug where rootless Podman could fail to retrieve some containers while refreshing the state
- Fixed a bug where `podman start --attach --sig-proxy=false` would still proxy signals into the container
- Fixed a bug where Podman would unconditionally use a non-default path for authentication credentials (`auth.json`), breaking `podman login` integration with `skopeo` and other tools using the containers/image library
- Fixed a bug where `podman ps --format=json` and `podman images --format=json` would display `null` when no results were returned, instead of valid JSON
- Fixed a bug where `podman build --squash` was incorrectly squashing all layers into one, instead of only new layers
- Fixed a bug where rootless Podman would allow volumes with options to be mounted (mounting volumes requires root), creating an inconsistent state where volumes reported as mounted but were not ([#4248](https://github.com/containers/podman/issues/4248))
- Fixed a bug where volumes which failed to unmount could not be removed ([#4247](https://github.com/containers/podman/issues/4247))
- Fixed a bug where Podman incorrectly handled some errors relating to unmounted or missing containers in containers/storage
- Fixed a bug where `podman stats` was broken on systems running CGroups V2 when run rootless ([#4268](https://github.com/containers/podman/issues/4268))
- Fixed a bug where the `podman start` command would print the short container ID, instead of the full ID
- Fixed a bug where containers created with an OCI runtime that is no longer available (uninstalled or removed from the config file) would not appear in `podman ps` and could not be removed via `podman rm`
- Fixed a bug where containers restored via `podman container restore --import` would retain the CGroup path of the original container, even if their container ID changed; thus, multiple containers created from the same checkpoint would all share the same CGroup

### Misc
- The default PID limit for containers is now set to 4096. It can be adjusted back to the old default (unlimited) by passing `--pids-limit 0` to `podman create` and `podman run`
- The `podman start --attach` command now automatically attaches `STDIN` if the container was created with `-i`
- The `podman network create` command now validates network names using the same regular expression as container and pod names
- The `--systemd` flag to `podman run` and `podman create` will now only enable systemd mode when the binary being run inside the container is `/sbin/init`, `/usr/sbin/init`, or ends in `systemd` (previously detected any path ending in `init` or `systemd`)
- Updated vendored Buildah to 1.11.3
- Updated vendored containers/storage to 1.13.5
- Updated vendored containers/image to 4.0.1

## 1.6.1
### Bugfixes
- Fixed a bug where rootless Podman on systems using CGroups V2 would not function with the `cgroupfs` CGroups manager
- Fixed a bug where rootless Podman could not correctly identify the DBus session address, causing containers to fail to start ([#4162](https://github.com/containers/podman/issues/4162))
- Fixed a bug where rootless Podman with `slirp4netns` networking would fail to start containers due to mount leaks

## 1.6.0
### Features
- The `podman network create`, `podman network rm`, `podman network inspect`, and `podman network ls` commands have been added to manage CNI networks used by Podman
- The `podman volume create` command can now create and mount volumes with options, allowing volumes backed by NFS, tmpfs, and many other filesystems
- Podman can now run containers without CGroups for better integration with systemd by using the `--cgroups=disabled` flag with `podman create` and `podman run`. This is presently only supported with the `crun` OCI runtime
- The `podman volume rm` and `podman volume inspect` commands can now refer to volumes by an unambiguous partial name, in addition to full name (e.g. `podman volume rm myvol` to remove a volume named `myvolume`) ([#3891](https://github.com/containers/podman/issues/3891))
- The `podman run` and `podman create` commands now support the `--pull` flag to allow forced re-pulling of images ([#3734](https://github.com/containers/podman/issues/3734))
- Mounting volumes into a container using `--volume`, `--mount`, and `--tmpfs` now allows the `suid`, `dev`, and `exec` mount options (the inverse of `nosuid`, `nodev`, `noexec`) ([#3819](https://github.com/containers/podman/issues/3819))
- Mounting volumes into a container using `--mount` now allows the `relabel=Z` and `relabel=z` options to relabel mounts.
- The `podman push` command now supports the `--digestfile` option to save a file containing the pushed digest
- Pods can now have their hostname set via `podman pod create --hostname` or providing Pod YAML with a hostname set to `podman play kube` ([#3732](https://github.com/containers/podman/issues/3732))
- The `podman image sign` command now supports the `--cert-dir` flag
- The `podman run` and `podman create` commands now support the `--security-opt label=filetype:$LABEL` flag to set the SELinux label for container files
- The remote Podman client now supports healthchecks

### Bugfixes
- Fixed a bug where remote `podman pull` would panic if a Varlink connection was not available ([#4013](https://github.com/containers/podman/issues/4013))
- Fixed a bug where `podman exec` would not properly set terminal size when creating a new exec session ([#3903](https://github.com/containers/podman/issues/3903))
- Fixed a bug where `podman exec` would not clean up socket symlinks on the host ([#3962](https://github.com/containers/podman/issues/3962))
- Fixed a bug where Podman could not run systemd in containers that created a CGroup namespace
- Fixed a bug where `podman prune -a` would attempt to prune images used by Buildah and CRI-O, causing errors ([#3983](https://github.com/containers/podman/issues/3983))
- Fixed a bug where improper permissions on the `~/.config` directory could cause rootless Podman to use an incorrect directory for storing some files
- Fixed a bug where the bash completions for `podman import` threw errors
- Fixed a bug where Podman volumes created with `podman volume create` would not copy the contents of their mountpoint the first time they were mounted into a container ([#3945](https://github.com/containers/podman/issues/3945))
- Fixed a bug where rootless Podman could not run `podman exec` when the container was not run inside a CGroup owned by the user ([#3937](https://github.com/containers/podman/issues/3937))
- Fixed a bug where `podman play kube` would panic when given Pod YAML without a `securityContext` ([#3956](https://github.com/containers/podman/issues/3956))
- Fixed a bug where Podman would place files incorrectly when `storage.conf` configuration items were set to the empty string ([#3952](https://github.com/containers/podman/issues/3952))
- Fixed a bug where `podman build` did not correctly inherit Podman's CGroup configuration, causing crashed on CGroups V2 systems ([#3938](https://github.com/containers/podman/issues/3938))
- Fixed a bug where `podman cp` would improperly copy files on the host when copying a symlink in the container that included a glob operator ([#3829](https://github.com/containers/podman/issues/3829))
- Fixed a bug where remote `podman run --rm` would exit before the container was completely removed, allowing race conditions when removing container resources ([#3870](https://github.com/containers/podman/issues/3870))
- Fixed a bug where rootless Podman would not properly handle changes to `/etc/subuid` and `/etc/subgid` after a container was launched
- Fixed a bug where rootless Podman could not include some devices in a container using the `--device` flag ([#3905](https://github.com/containers/podman/issues/3905))
- Fixed a bug where the `commit` Varlink API would segfault if provided incorrect arguments ([#3897](https://github.com/containers/podman/issues/3897))
- Fixed a bug where temporary files were not properly cleaned up after a build using remote Podman ([#3869](https://github.com/containers/podman/issues/3869))
- Fixed a bug where `podman remote cp` crashed instead of reporting it was not yet supported ([#3861](https://github.com/containers/podman/issues/3861))
- Fixed a bug where `podman exec` would run as the wrong user when execing into a container was started from an image with Dockerfile `USER` (or a user specified via `podman run --user`) ([#3838](https://github.com/containers/podman/issues/3838))
- Fixed a bug where images pulled using the `oci:` transport would be improperly named
- Fixed a bug where `podman varlink` would hang when managed by systemd due to SD_NOTIFY support conflicting with Varlink ([#3572](https://github.com/containers/podman/issues/3572))
- Fixed a bug where mounts to the same destination would sometimes not trigger a conflict, causing a race as to which was actually mounted
- Fixed a bug where `podman exec --preserve-fds` caused Podman to hang ([#4020](https://github.com/containers/podman/issues/4020))
- Fixed a bug where removing an unmounted container that was unmounted might sometimes not properly clean up the container ([#4033](https://github.com/containers/podman/issues/4033))
- Fixed a bug where the Varlink server would freeze when run in a systemd unit file ([#4005](https://github.com/containers/podman/issues/4005))
- Fixed a bug where Podman would not properly set the `$HOME` environment variable when the OCI runtime did not set it
- Fixed a bug where rootless Podman would incorrectly print warning messages when an OCI runtime was not found ([#4012](https://github.com/containers/podman/issues/4012))
- Fixed a bug where named volumes would conflict with, instead of overriding, `tmpfs` filesystems added by the `--read-only-tmpfs` flag to `podman create` and `podman run`
- Fixed a bug where `podman cp` would incorrectly make the target directory when copying to a symlink which pointed to a nonexistent directory ([#3894](https://github.com/containers/podman/issues/3894))
- Fixed a bug where remote Podman would incorrectly read `STDIN` when the `-i` flag was not set ([#4095](https://github.com/containers/podman/issues/4095))
- Fixed a bug where `podman play kube` would create an empty pod when given an unsupported YAML type ([#4093](https://github.com/containers/podman/issues/4093))
- Fixed a bug where `podman import --change` improperly parsed `CMD` ([#4000](https://github.com/containers/podman/issues/4000))

### Misc
- Significant changes were made to Podman volumes in this release. If you have pre-existing volumes, it is strongly recommended to run `podman system renumber` after upgrading.
- Version 0.8.1 or greater of the CNI Plugins is now required for Podman
- Version 2.0.1 or greater of Conmon is strongly recommended
- Updated vendored Buildah to v1.11.2
- Updated vendored containers/storage library to v1.13.4
- Improved error messages when trying to create a pod with no name via `podman play kube`
- Improved error messages when trying to run `podman pause` or `podman stats` on a rootless container on a system without CGroups V2 enabled
- `TMPDIR` has been set to `/var/tmp` by default to better handle large temporary files
- `podman wait` has been optimized to detect stopped containers more rapidly
- Podman containers now include a `ContainerManager` annotation indicating they were created by `libpod`
- The `podman info` command now includes information about `slirp4netns` and `fuse-overlayfs` if they are available
- Podman no longer sets a default size of 65kb for tmpfs filesystems
- The default Podman CNI network has been renamed in an attempt to prevent conflicts with CRI-O when both are run on the same system. This should only take effect on system restart
- The output of `podman volume inspect` has been more closely matched to `docker volume inspect`

## 1.5.1
### Features
- The hostname of pods is now set to the pod's name

### Bugfixes
- Fixed a bug where `podman run` and `podman create` did not honor the `--authfile` option ([#3730](https://github.com/containers/podman/issues/3730))
- Fixed a bug where containers restored with `podman container restore --import` would incorrectly duplicate the Conmon PID file of the original container
- Fixed a bug where `podman build` ignored the default OCI runtime configured in `libpod.conf`
- Fixed a bug where `podman run --rm` (or force-removing any running container with `podman rm --force`) were not retrieving the correct exit code ([#3795](https://github.com/containers/podman/issues/3795))
- Fixed a bug where Podman would exit with an error if any configured hooks directory was not present
- Fixed a bug where `podman inspect` and `podman commit` would not use the correct `CMD` for containers run with `podman play kube`
- Fixed a bug created pods when using rootless Podman and CGroups V2 ([#3801](https://github.com/containers/podman/issues/3801))
- Fixed a bug where the `podman events` command with the `--since` or `--until` options could take a very long time to complete

### Misc
- Rootless Podman will now inherit OCI runtime configuration from the root configuration ([#3781](https://github.com/containers/podman/issues/3781))
- Podman now properly sets a user agent while contacting registries ([#3788](https://github.com/containers/podman/issues/3788))

## 1.5.0
### Features
- Podman containers can now join the user namespaces of other containers with `--userns=container:$ID`, or a user namespace at an arbitrary path with `--userns=ns:$PATH`
- Rootless Podman can experimentally squash all UIDs and GIDs in an image to a single UID and GID (which does not require use of the `newuidmap` and `newgidmap` executables) by passing `--storage-opt ignore_chown_errors`
- The `podman generate kube` command now produces YAML for any bind mounts the container has created ([#2303](https://github.com/containers/podman/issues/2303))
- The `podman container restore` command now features a new flag, `--ignore-static-ip`, that can be used with `--import` to import a single container with a static IP multiple times on the same host
- Added the ability for `podman events` to output JSON by specifying `--format=json`
- If the OCI runtime or `conmon` binary cannot be found at the paths specified in `libpod.conf`, Podman will now also search for them in the calling user's path
- Added the ability to use `podman import` with URLs ([#3609](https://github.com/containers/podman/issues/3609))
- The `podman ps` command now supports filtering names using regular expressions ([#3394](https://github.com/containers/podman/issues/3394))
- Rootless Podman containers with `--privileged` set will now mount in all host devices that the user can access
- The `podman create` and `podman run` commands now support the `--env-host` flag to forward all environment variables from the host into the container
- Rootless Podman now supports healthchecks ([#3523](https://github.com/containers/podman/issues/3523))
- The format of the `HostConfig` portion of the output of `podman inspect` on containers has been improved and synced with Docker
- Podman containers now support CGroup namespaces, and can create them by passing `--cgroupns=private` to `podman run` or `podman create`
- The `podman create` and `podman run` commands now support the `--ulimit=host` flag, which uses any ulimits currently set on the host for the container
- The `podman rm` and `podman rmi` commands now use different exit codes to indicate 'no such container' and 'container is running' errors
- Support for CGroups V2 through the `crun` OCI runtime has been greatly improved, allowing resource limits to be set for rootless containers when the CGroups V2 hierarchy is in use

### Bugfixes
- Fixed a bug where a race condition could cause `podman restart` to fail to start containers with ports
- Fixed a bug where containers restored from a checkpoint would not properly report the time they were started at
- Fixed a bug where `podman search` would return at most 25 results, even when the maximum number of results was set higher
- Fixed a bug where `podman play kube` would not honor capabilities set in imported YAML ([#3689](https://github.com/containers/podman/issues/3689))
- Fixed a bug where `podman run --env`, when passed a single key (to use the value from the host), would set the environment variable in the container even if it was not set on the host ([#3648](https://github.com/containers/podman/issues/3648))
- Fixed a bug where `podman commit --changes` would not properly set environment variables
- Fixed a bug where Podman could segfault while working with images with no history
- Fixed a bug where `podman volume rm` could remove arbitrary volumes if given an ambiguous name ([#3635](https://github.com/containers/podman/issues/3635))
- Fixed a bug where `podman exec` invocations leaked memory by not cleaning up files in tmpfs
- Fixed a bug where the `--dns` and `--net=container` flags to `podman run` and `podman create` were not mutually exclusive ([#3553](https://github.com/containers/podman/issues/3553))
- Fixed a bug where rootless Podman would be unable to run containers when less than 5 UIDs were available
- Fixed a bug where containers in pods could not be removed without removing the entire pod ([#3556](https://github.com/containers/podman/issues/3556))
- Fixed a bug where Podman would not properly clean up all CGroup controllers for created cgroups when using the `cgroupfs` CGroup driver
- Fixed a bug where Podman containers did not properly clean up files in tmpfs, resulting in a memory leak as containers stopped
- Fixed a bug where healthchecks from images would not use default settings for interval, retries, timeout, and start period when they were not provided by the image ([#3525](https://github.com/containers/podman/issues/3525))
- Fixed a bug where healthchecks using the `HEALTHCHECK CMD` format where not properly supported ([#3507](https://github.com/containers/podman/issues/3507))
- Fixed a bug where volume mounts using relative source paths would not be properly resolved ([#3504](https://github.com/containers/podman/issues/3504))
- Fixed a bug where `podman run` did not use authorization credentials when a custom path was specified ([#3524](https://github.com/containers/podman/issues/3524))
- Fixed a bug where containers checkpointed with `podman container checkpoint` did not properly set their finished time
- Fixed a bug where running `podman inspect` on any container not created with `podman run` or `podman create` (for example, pod infra containers) would result in a segfault ([#3500](https://github.com/containers/podman/issues/3500))
- Fixed a bug where healthcheck flags for `podman create` and `podman run` were incorrectly named ([#3455](https://github.com/containers/podman/pull/3455))
- Fixed a bug where Podman commands would fail to find targets if a partial ID was specified that was ambiguous between a container and pod ([#3487](https://github.com/containers/podman/issues/3487))
- Fixed a bug where restored containers would not have the correct SELinux label
- Fixed a bug where Varlink endpoints were not working properly if `more` was not correctly specified
- Fixed a bug where the Varlink PullImage endpoint would crash if an error occurred ([#3715](https://github.com/containers/podman/issues/3715))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not allow boolean arguments for its `ro` and `rw` options ([#2980](https://github.com/containers/podman/issues/2980))
- Fixed a bug where pods did not properly share the UTS namespace, resulting in incorrect behavior from some utilities which rely on hostname ([#3547](https://github.com/containers/podman/issues/3547))
- Fixed a bug where Podman would unconditionally append `ENTRYPOINT` to `CMD` during `podman commit` (and when reporting `CMD` in `podman inspect`) ([#3708](https://github.com/containers/podman/issues/3708))
- Fixed a bug where `podman events` with the `journald` events backend would incorrectly print 6 previous events when only new events were requested ([#3616](https://github.com/containers/podman/issues/3616))
- Fixed a bug where `podman port` would exit prematurely when a port number was specified ([#3747](https://github.com/containers/podman/issues/3747))
- Fixed a bug where passing `.` as an argument to the `--dns-search` flag to `podman create` and `podman run` was not properly clearing DNS search domains in the container

### Misc
- Updated vendored Buildah to v1.10.1
- Updated vendored containers/image to v3.0.2
- Updated vendored containers/storage to v1.13.1
- Podman now requires conmon v2.0.0 or higher
- The `podman info` command now displays the events logger being in use
- The `podman inspect` command on containers now includes the ID of the pod a container has joined and the PID of the container's conmon process
- The `-v` short flag for `podman --version` has been re-added
- Error messages from `podman pull` should be significantly clearer
- The `podman exec` command is now available in the remote client

## 1.4.4
### Bugfixes
- Fixed a bug where rootless Podman would attempt to use the entire root configuration if no rootless configuration was present for the user, breaking rootless Podman for new installations
- Fixed a bug where rootless Podman's pause process would block SIGTERM, preventing graceful system shutdown and hanging until the system's init send SIGKILL
- Fixed a bug where running Podman as root with `sudo -E` would not work after running rootless Podman at least once
- Fixed a bug where options for `tmpfs` volumes added with the `--tmpfs` flag were being ignored
- Fixed a bug where images with no layers could not properly be displayed and removed by Podman
- Fixed a bug where locks were not properly freed on failure to create a container or pod

### Misc
- Updated containers/storage to v1.12.13

## 1.4.3
### Features
- Podman now has greatly improved support for containers using multiple OCI runtimes. Containers now remember if they were created with a different runtime using `--runtime` and will always use that runtime
- The `cached` and `delegated` options for volume mounts are now allowed for Docker compatibility ([#3340](https://github.com/containers/podman/issues/3340))
- The `podman diff` command now supports the `--latest` flag

### Bugfixes
- Fixed a bug where `podman cp` on a single file would create a directory at the target and place the file in it ([#3384](https://github.com/containers/podman/issues/3384))
- Fixed a bug where `podman inspect --format '{{.Mounts}}'` would print a hexadecimal address instead of a container's mounts
- Fixed a bug where rootless Podman would not add an entry to container's `/etc/hosts` files for their own hostname ([#3405](https://github.com/containers/podman/issues/3405))
- Fixed a bug where `podman ps --sync` would segfault ([#3411](https://github.com/containers/podman/issues/3411))
- Fixed a bug where `podman generate kube` would produce an invalid ports configuration ([#3408](https://github.com/containers/podman/issues/3408))

### Misc
- Podman now performs much better on systems with heavy I/O load
- The `--cgroup-manager` flag to `podman` now shows the correct default setting in help if the default was overridden by `libpod.conf`
- For backwards compatibility, setting `--log-driver=json-file` in `podman run` is now supported as an alias for `--log-driver=k8s-file`. This is considered deprecated, and `json-file` will be moved to a new implementation in the future ([#3363](https://github.com/containers/podman/issues/3363))
- Podman's default `libpod.conf` file now allows the [crun](https://github.com/giuseppe/crun) OCI runtime to be used if it is installed

## 1.4.2
### Bugfixes
- Fixed a bug where Podman could not run containers using an older version of Systemd as init ([#3295](https://github.com/containers/podman/issues/3295))

### Misc
- Updated vendored Buildah to v1.9.0 to resolve a critical bug with Dockerfile `RUN` instructions
- The error message for running `podman kill` on containers that are not running has been improved
- The Podman remote client can now log to a file if syslog is not available

## 1.4.1
### Features
- The `podman exec` command now sets its error code differently based on whether the container does not exist, and the command in the container does not exist
- The `podman inspect` command on containers now outputs Mounts JSON that matches that of `docker inspect`, only including user-specified volumes and differentiating bind mounts and named volumes
- The `podman inspect` command now reports the path to a container's OCI spec with the `OCIConfigPath` key (only included when the container is initialized or running)
- The `podman run --mount` command now supports the `bind-nonrecursive` option for bind mounts ([#3314](https://github.com/containers/podman/issues/3314))

### Bugfixes
- Fixed a bug where `podman play kube` would fail to create containers due to an unspecified log driver
- Fixed a bug where Podman would fail to build with [musl libc](https://www.musl-libc.org/) ([#3284](https://github.com/containers/podman/issues/3284))
- Fixed a bug where rootless Podman using `slirp4netns` networking in an environment with no nameservers on the host other than localhost would result in nonfunctional networking ([#3277](https://github.com/containers/podman/issues/3277))
- Fixed a bug where `podman import` would not properly set environment variables, discarding their values and retaining only keys
- Fixed a bug where Podman would fail to run when built with Apparmor support but run on systems without the Apparmor kernel module loaded ([#3331](https://github.com/containers/podman/issues/3331))

### Misc
- Remote Podman will now default the username it uses to log in to remote systems to the username of the current user
- Podman now uses JSON logging with OCI runtimes that support it, allowing for better error reporting
- Updated vendored Buildah to v1.8.4
- Updated vendored containers/image to v2.0

## 1.4.0
### Features
- The `podman checkpoint` and `podman restore` commands can now be used to migrate containers between Podman installations on different systems ([#1618](https://github.com/containers/podman/issues/1618))
- The `podman cp` command now supports a `pause` flag to pause containers while copying into them
- The remote client now supports a configuration file for pre-configuring connections to remote Podman installations

### Bugfixes
- Fixed CVE-2019-10152 - The `podman cp` command improperly dereferenced symlinks in host context
- Fixed a bug where `podman commit` could improperly set environment variables that contained `=` characters ([#3132](https://github.com/containers/podman/issues/3132))
- Fixed a bug where rootless Podman would sometimes fail to start containers with forwarded ports ([#2942](https://github.com/containers/podman/issues/2942))
- Fixed a bug where `podman version` on the remote client could segfault ([#3145](https://github.com/containers/podman/issues/3145))
- Fixed a bug where `podman container runlabel` would use `/proc/self/exe` instead of the path of the Podman command when printing the command being executed
- Fixed a bug where filtering images by label did not work ([#3163](https://github.com/containers/podman/issues/3163))
- Fixed a bug where specifying a bing mount or tmpfs mount over an image volume would cause a container to be unable to start ([#3174](https://github.com/containers/podman/issues/3174))
- Fixed a bug where `podman generate kube` did not work with containers with named volumes
- Fixed a bug where rootless Podman would receive `permission denied` errors accessing `conmon.pid` ([#3187](https://github.com/containers/podman/issues/3187))
- Fixed a bug where `podman cp` with a folder specified as target would replace the folder, as opposed to copying into it ([#3184](https://github.com/containers/podman/issues/3184))
- Fixed a bug where rootless Podman commands could double-unlock a lock, causing a crash ([#3207](https://github.com/containers/podman/issues/3207))
- Fixed a bug where Podman incorrectly set `tmpcopyup` on `/dev/` mounts, causing errors when using the Kata containers runtime ([#3229](https://github.com/containers/podman/issues/3229))
- Fixed a bug where `podman exec` would fail on older kernels ([#2968](https://github.com/containers/podman/issues/2968))

### Misc
- The `podman inspect` command on containers now uses the `Id` key (instead of `ID`) for the container's ID, for better compatibility with the output of `docker inspect`
- The `podman commit` command is now usable with the Podman remote client
- The `--signature-policy` flag (used with several image-related commands) has been deprecated
- The `podman unshare` command now defines two environment variables in the spawned shell: `CONTAINERS_RUNROOT` and `CONTAINERS_GRAPHROOT`, pointing to temporary and permanent storage for rootless containers
- Updated vendored containers/storage and containers/image libraries with numerous bugfixes
- Updated vendored Buildah to v1.8.3
- Podman now requires [Conmon v0.2.0](https://github.com/containers/conmon/releases/tag/v0.2.0)
- The `podman cp` command is now aliased as `podman container cp`
- Rootless Podman will now default `init_path` using root Podman's configuration files (`/etc/containers/libpod.conf` and `/usr/share/containers/libpod.conf`) if not overridden in the rootless configuration

## 1.3.1
### Features
- The `podman cp` command can now read input redirected to `STDIN`, and output to `STDOUT` instead of a file, using `-` instead of an argument.
- The Podman remote client now displays version information from both the client and server in `podman version`
- The `podman unshare` command has been added, allowing easy entry into the user namespace set up by rootless Podman (allowing the removal of files created by rootless Podman, among other things)

### Bugfixes
- Fixed a bug where Podman containers with the `--rm` flag were removing created volumes when they were automatically removed ([#3071](https://github.com/containers/podman/issues/3071))
- Fixed a bug where container and pod locks were incorrectly marked as released after a system reboot, causing errors on container and pod removal ([#2900](https://github.com/containers/podman/issues/2900))
- Fixed a bug where Podman pods could not be removed if any container in the pod encountered an error during removal ([#3088](https://github.com/containers/podman/issues/3088))
- Fixed a bug where Podman pods run with the `cgroupfs` CGroup driver would encounter a race condition during removal, potentially failing to remove the pod CGroup
- Fixed a bug where the `podman container checkpoint` and `podman container restore` commands were not visible in the remote client
- Fixed a bug where `podman remote ps --ns` would not print the container's namespaces ([#2938](https://github.com/containers/podman/issues/2938))
- Fixed a bug where removing stopped containers with healthchecks could cause an error
- Fixed a bug where the default `libpod.conf` file was causing parsing errors ([#3095](https://github.com/containers/podman/issues/3095))
- Fixed a bug where pod locks were not being freed when pods were removed, potentially leading to lock exhaustion
- Fixed a bug where 'podman run' with SD_NOTIFY set could, on short-running containers, create an inconsistent state rendering the container unusable

### Misc
- The remote Podman client now uses the Varlink bridge to establish remote connections by default

## 1.3.0
### Features
- Podman now supports container restart policies! The `--restart` flag on `podman create` and `podman run` allows containers to be restarted after they exit. Please note that Podman cannot restart containers after a system reboot - for that, see our next feature
- Podman `podman generate systemd` command was added to generate systemd unit files for managing Podman containers
- The `podman runlabel` command now allows a `$GLOBAL_OPTS` variable, which will be populated by global options passed to the `podman runlabel` command, allowing custom storage configurations to be passed into containers run with `runlabel` ([#2399](https://github.com/containers/podman/issues/2399))
- The `podman play kube` command now allows `File` and `FileOrCreate` volumes
- The `podman pod prune` command was added to prune unused pods
- Added the `podman system migrate` command to migrate containers using older configurations to allow their use by newer Libpod versions ([#2935](https://github.com/containers/podman/issues/2935))
- Podman containers now forward proxy-related environment variables from the host into the container with the `--http-proxy` flag (enabled by default)
- Read-only Podman containers can now create tmpfs filesystems on `/tmp`, `/var/tmp`, and `/run` with the `--read-only-tmpfs` flag (enabled by default)
- The `podman init` command was added, performing all container pre-start tasks without starting the container to allow pre-run debugging

### Bugfixes
- Fixed a bug where `podman cp` would not copy folders ([#2836](https://github.com/containers/podman/issues/2836))
- Fixed a bug where Podman would panic when the Varlink API attempted too pull a nonexistent image ([#2860](https://github.com/containers/podman/issues/2860))
- Fixed a bug where `podman rmi` sometimes did not produce an event when images were deleted
- Fixed a bug where Podman would panic when the Varlink API passed improperly-formatted options when attempting to build ([#2869](https://github.com/containers/podman/issues/2869))
- Fixed a bug where `podman images` would not print a header if no images were present ([#2877](https://github.com/containers/podman/pull/2877))
- Fixed a bug where the `podman images` command with `--filter dangling=false` would incorrectly print dangling images instead of images which are not dangling ([#2884](https://github.com/containers/podman/issues/2884))
- Fixed a bug where rootless Podman would panic when any command was run after the system was rebooted ([#2894](https://github.com/containers/podman/issues/2894))
- Fixed a bug where Podman containers in user namespaces would include undesired directories from the host in `/sys/kernel`
- Fixed a bug where `podman create` would panic when trying to create a container whose name already existed
- Fixed a bug where `podman pull` would exit 0 on failing to pull an image ([#2785](https://github.com/containers/podman/issues/2785))
- Fixed a bug where `podman pull` would not properly print the cause of errors that occurred ([#2710](https://github.com/containers/podman/issues/2710))
- Fixed a bug where rootless Podman commands were not properly suspended via `ctrl-z` in a shell ([#2775](https://github.com/containers/podman/issues/2775))
- Fixed a bug where Podman would error when cleaning up containers when some container mountpoints in `/sys/` were cleaned up already by the closing of the mount namespace
- Fixed a bug where `podman play kube` was not including environment variables from the image run ([#2930](https://github.com/containers/podman/issues/2930))
- Fixed a bug where `podman play kube` would not properly clean up partially-created pods when encountering an error
- Fixed a bug where `podman commit` with the `--change` flag improperly set `CMD` when a multipart value was provided ([#2951](https://github.com/containers/podman/issues/2951))
- Fixed a bug where the `--mount` flag to `podman create` and `podman run` did not properly validate its arguments, causing Podman to panic
- Fixed a bug where conflicts between mounts created by the `--mount`, `--volume`, and `--tmpfs` flags were not properly reported
- Fixed a bug where the `--mount` flag could not be used with named volumes
- Fixed a bug where the `--mount` flag did not properly set options for created tmpfs filesystems
- Fixed a bug where rootless Podman could close too many file descriptors, causing Podman to panic ([#2964](https://github.com/containers/podman/issues/2964))
- Fixed a bug where `podman logout` would not print an error when the login was established by `docker login` ([#2735](https://github.com/containers/podman/issues/2735))
- Fixed a bug where `podman stop` would error when not all containers were running ([#2993](https://github.com/containers/podman/issues/2993))
- Fixed a bug where `podman pull` would fail to pull images by shortname if they were not present in the `docker.io` registry
- Fixed a bug where `podman login` would error when credentials were not present if a credential helper was configured ([#1675](https://github.com/containers/podman/issues/1675))
- Fixed a bug where the `podman system renumber` command and Podman post-reboot state refreshes would not create events
- Fixed a bug where the `podman top` command was not compatible with `docker top` syntax

### Misc
- Updated vendored Buildah to v1.8.2
- Updated vendored containers/storage to v1.12.6
- Updated vendored containers/psgo to v1.2.1
- Updated to sysregistriesv2, including slight changes to the `registries.conf` config file
- Rootless Podman now places all containers within a single user namespace. This change will not take effect for existing containers until containers are restarted, and containers that are not restarted may not be fully usable
- The `podman run`, `podman create`, `podman start`, `podman restart`, `podman attach`, `podman stop`, `podman port`, `podman rm`, `podman top`, `podman image tree`, `podman generate kube`, `podman umount`, `podman container checkpoint`, and `podman container restore` commands are now available in the remote client
- The Podman remote client now builds on Windows
- A major refactor of volumes created using the `podman volume` command was performed. There should be no major user-facing changes, but downgrading from Podman 1.3 to previous versions may render some volumes unable to be removed.
- The `podman events` command now logs events to journald by default. The old behavior (log to file) can be configured in podman.conf via the `events_logger` option
- The `podman commit` command, in versions 1.2 and earlier, included all volumes mounted into the container as image volumes in the committed image. This behavior was incorrect and has been disabled by default; it can be re-enabled with the `--include-volumes` flag


## 1.2.0
### Features
- Podman now supports image healthchecks! The `podman healthcheck run` command was added to manually run healthchecks, and the status of a running healthcheck can be viewed via `podman inspect`
- The `podman events` command was added to show a stream of significant events
- The `podman ps` command now supports a `--watch` flag that will refresh its output on a given interval
- The `podman image tree` command was added to show a tree representation of an image's layers
- The `podman logs` command can now display logs for multiple containers at the same time ([#2219](https://github.com/containers/podman/issues/2219))
- The `podman exec` command can now pass file descriptors to the process being executed in the container via the `--preserve-fds` option ([#2372](https://github.com/containers/podman/issues/2372))
- The `podman images` command can now filter images by reference ([#2266](https://github.com/containers/podman/issues/2266))
- The `podman system df` command was added to show disk usage by Podman
- The `--add-host` option can now be used by containers sharing a network namespace ([#2504](https://github.com/containers/podman/issues/2504))
- The `podman cp` command now has an `--extract` option to extract the contents of a Tar archive and copy them into the container, instead of copying the archive itself ([#2520](https://github.com/containers/podman/issues/2520))
- Podman now allows manually specifying the path of the `slirp4netns` binary for rootless networking via the `--network-cmd-path` flag ([#2506](https://github.com/containers/podman/issues/2506))
- Rootless Podman can now be used with a single UID and GID, without requiring a full 65536 UIDs/GIDs to be allocated in `/etc/subuid` and `/etc/subgid` ([#1651](https://github.com/containers/podman/issues/1651))
- The `podman runlabel` command now supports the `--replace` option to replace containers using the name requested
- Infrastructure containers for Podman pods will now attempt to use the image's `CMD` and `ENTRYPOINT` instead of a fixed command ([#2182](https://github.com/containers/podman/issues/2182))
- The `podman play kube` command now supports the `HostPath` and `VolumeMounts` YAML fields ([#2536](https://github.com/containers/podman/issues/2536))
- Added support to disable creation of `resolv.conf` or `/etc/hosts` in containers by specifying `--dns=none` and `--no-hosts`, respectively, to `podman run` and `podman create` ([#2744](https://github.com/containers/podman/issues/2744))
- The `podman version` command now supports the `{{ json . }}` template (which outputs JSON)
- Podman can now forward ports using the SCTP protocol

### Bugfixes
- Fixed a bug where directories could not be passed to `podman run --device` ([#2380](https://github.com/containers/podman/issues/2380))
- Fixed a bug where rootless Podman with the `--config` flag specified would not use appropriate defaults ([#2510](https://github.com/containers/podman/issues/2510))
- Fixed a bug where rootless Podman containers using the host network (`--net=host`) would show SELinux as enabled in the container when there were no privileges to use it
- Fixed a bug where importing very large images from `STDIN` could cause Podman to run out of memory
- Fixed a bug where some images would fail to run due to symlinks in paths where Podman would normally mount tmpfs filesystems
- Fixed a bug where `podman play kube` would sometimes segfault ([#2209](https://github.com/containers/podman/issues/2209))
- Fixed a bug where `podman runlabel` did not respect the `$PWD` variable ([#2171](https://github.com/containers/podman/issues/2171))
- Fixed a bug where error messages from refreshing the state in rootless Podman were not properly displayed ([#2584](https://github.com/containers/podman/issues/2584))
- Fixed a bug where rootless `podman build` could not access DNS servers when `slirp4netns` was in use ([#2572](https://github.com/containers/podman/issues/2572))
- Fixed a bug where rootless `podman stop` and `podman rm` would not work on containers which specified a non-root user ([#2577](https://github.com/containers/podman/issues/2577))
- Fixed a bug where container labels whose values contained commas were incorrectly parsed and caused errors creating containers ([#2574](https://github.com/containers/podman/issues/2574))
- Fixed a bug where calling Podman with a nonexistent command would exit 0, instead of with an appropriate error code ([#2530](https://github.com/containers/podman/issues/2530))
- Fixed a bug where rootless `podman exec` would fail when `--user` was specified ([#2566](https://github.com/containers/podman/issues/2566))
- Fixed a bug where, when a container had a name that was a fragment of another container's ID, Podman would refuse to operate on the first container by name
- Fixed a bug where `podman pod create` would fail if a pod shared no namespaces but created an infra container
- Fixed a bug where rootless Podman failed on the S390 and CRIS architectures
- Fixed a bug where `podman rm` would exit 0 if no containers specified were found ([#2539](https://github.com/containers/podman/issues/2539))
- Fixed a bug where `podman run` would fail to enable networking for containers with additional CNI networks specified ([#2795](https://github.com/containers/podman/issues/2795))
- Fixed a bug where the `podman images` command on the remote client was not displaying digests ([#2756](https://github.com/containers/podman/issues/2756))
- Fixed a bug where Podman was unable to clean up mounts in containers using user namespaces
- Fixed a bug where `podman image save` would, when told to save to a path that exists, return an error, but still delete the file at the given path
- Fixed a bug where specifying environment variables containing commas with `--env` would cause parsing errors ([#2712](https://github.com/containers/podman/issues/2712))
- Fixed a bug where `podman umount` would not error if called with no arguments
- Fixed a bug where the user and environment variables specified by the image used in containers created by `podman create kube` was being ignored ([#2665](https://github.com/containers/podman/issues/2665))
- Fixed a bug where the `podman pod inspect` command would segfault if not given an argument ([#2681](https://github.com/containers/podman/issues/2681))
- Fixed a bug where rootless `podman pod top` would fail ([#2682](https://github.com/containers/podman/issues/2682))
- Fixed a bug where the `podman load` command would not error if an input file is not specified and a file was not redirected to `STDIN`
- Fixed a bug where rootless `podman` could fail if global configuration was altered via flag (for example, `--root`, `--runroot`, `--storage-driver`)
- Fixed a bug where forwarded ports that were part of a range (e.g. 20-30) were displayed individually by `podman ps`, as opposed to together as a range ([#1358](https://github.com/containers/podman/issues/1358))
- Fixed a bug where `podman run --rootfs` could panic ([#2654](https://github.com/containers/podman/issues/2654))
- Fixed a bug where `podman build` would fail if options were specified after the directory to build ([#2636](https://github.com/containers/podman/issues/2636))
- Fixed a bug where image volumes made by `podman create` and `podman run` would have incorrect permissions ([#2634](https://github.com/containers/podman/issues/2634))
- Fixed a bug where rootless containers were not using the containers/image blob cache, leading to slower image pulls
- Fixed a bug where the `podman image inspect` command incorrectly allowed the `--latest`, `--type`, and `--size` options

### Misc
- Updated Buildah to v1.7.2
- Updated `psgo` library to v1.2, featuring greatly improved safety during concurrent use
- The `podman events` command may not show all activity regarding images, as only Podman was instrumented; images created, deleted, or pulled by CRI-O or Buildah will not be shown in `podman events`
- The `podman pod top` and `podman pod stats` commands are now usable with the Podman remote client
- The `podman kill` and `podman wait` commands are now usable with the Podman remote client
- Removed the unused `restarting` state and mapped `stopped` (also unused) to `exited` in `podman ps --filter status`
- Podman container, pod, and volume names may now contain the `.` (period) character

## 1.1.2
### Bugfixes
- Fixed a bug where the `podman image list`, `podman image rm`, and `podman container list` had broken global storage options
- Fixed a bug where the `--label` option to `podman create` and `podman run` was missing the `-l` alias
- Fixed a bug where running Podman with the `--config` flag would not set an appropriate default value for `tmp_dir` ([#2408](https://github.com/containers/podman/issues/2408))
- Fixed a bug where the `podman logs` command with the `--timestamps` flag produced unreadable output ([#2500](https://github.com/containers/podman/issues/2500))
- Fixed a bug where the `podman cp` command would automatically extract `.tar` files copied into the container ([#2509](https://github.com/containers/podman/issues/2509))

### Misc
- The `podman container stop` command is now usable with the Podman remote client

## 1.1.1
### Bugfixes
- Fixed a bug where `podman container restore` was erroneously available as `podman restore` ([#2191](https://github.com/containers/podman/issues/2191))
- Fixed a bug where the `volume_path` option in `libpod.conf` was not being respected
- Fixed a bug where Podman failed to build when the `varlink` tag was not present ([#2459](https://github.com/containers/podman/issues/2459))
- Fixed a bug where the `podman image load` command was listed twice in help text
- Fixed a bug where the `podman image sign` command was also listed as `podman sign`
- Fixed a bug where the `podman image list` command incorrectly had an `image` alias
- Fixed a bug where the `podman images` command incorrectly had `ls` and `list` aliases
- Fixed a bug where the `podman image rm` command was being displayed as `podman image rmi`
- Fixed a bug where the `podman create` command would attempt to parse arguments meant for the container
- Fixed a bug where the combination of FIPS mode and user namespaces resulted in permissions errors
- Fixed a bug where the `--time` alias for `--timeout` for the `podman restart` and `podman stop` commands did not function
- Fixed a bug where the default stop timeout for newly-created containers was being set to 0 seconds (resulting in an immediate SIGKILL on running `podman stop`)
- Fixed a bug where the output format of `podman port` was incorrect, printing full container ID instead of truncated ID
- Fixed a bug where the `podman container list` command did not exist
- Fixed a bug where `podman build` could not build a container from images tagged locally that did not exist in a registry ([#2469](https://github.com/containers/podman/issues/2469))
- Fixed a bug where some Podman commands that accept no arguments would not error when provided arguments
- Fixed a bug where `podman play kube` could not handle cases where a pod and a container shared a name

### Misc
- Usage text for many commands was greatly improved
- Major cleanups were made to Podman manpages, ensuring that command lists are accurate
- Greatly improved debugging output when the `newuidmap` and `newgidmap` binaries fail when using rootless Podman
- The `-s` alias for the global `--storage-driver` option has been removed
- The `podman container refresh` command has been deprecated, as its intended use case is no longer relevant. The command has been hidden and manpages deleted. It will be removed in a future release
- The `podman container runlabel` command will now pull images not available locally even without the `--pull` option. The `--pull` option has been deprecated
- The `podman container checkpoint` and `podman container restore` commands are now only available on OCI runtimes where they are supported (e.g. `runc`)

## 1.1.0
### Features
- Added `--latest` and `--all` flags to `podman mount` and `podman umount`
- Rootless Podman can now forward ports into containers (using the same `-p` and `-P` flags as root Podman)
- Rootless Podman will now pull some configuration options (for example, OCI runtime path) from the default root `libpod.conf` if they are not explicitly set in the user's own `libpod.conf` ([#2174](https://github.com/containers/podman/issues/2174))
- Added an alias `-f` for the `--format` flag of the `podman info` and `podman version` commands
- Added an alias `-s` for the `--size` flag of the `podman inspect` command
- Added the `podman system info` and `podman system prune` commands
- Added the `podman cp` command to copy files between containers and the host ([#613](https://github.com/containers/podman/issues/613))
- Added the `--password-stdin` flag to `podman login`
- Added the `--all-tags` flag to `podman pull`
- The `--rm` and `--detach` flags can now be used together with `podman run`
- The `podman start` and `podman run` commands for containers in pods will now start dependency containers if they are stopped
- Added the `podman system renumber` command to handle lock changes
- The `--net=host` and `--dns` flags for `podman run` and `podman create` no longer conflict
- Podman now handles mounting the shared /etc/resolv.conf from network namespaces created by `ip netns add` when they are passed in via `podman run --net=ns:`

### Bugfixes
- Fixed a bug with `podman inspect` where different information would be returned when the container was running versus when it was stopped
- Fixed a bug where errors in Go templates passed to `podman inspect` were silently ignored instead of reported to the user ([#2159](https://github.com/containers/podman/issues/2159))
- Fixed a bug where rootless Podman with `--pid=host` containers was incorrectly masking paths in `/proc`
- Fixed a bug where full errors starting rootless `Podman` were not reported when a refresh was requested
- Fixed a bug where Podman would override the config file-specified storage driver with the driver the backing database was created with without warning users
- Fixed a bug where `podman prune` would prune all images not in use by a container, as opposed to only untagged images, by default ([#2192](https://github.com/containers/podman/issues/2192))
- Fixed a bug where `podman create --quiet` and `podman run --quiet` were not properly suppressing output
- Fixed a bug where the `table` keyword in Go template output of `podman ps` was not working ([#2221](https://github.com/containers/podman/issues/2221))
- Fixed a bug where `podman inspect` on images pulled by digest would double-print `@sha256` in output when printing digests ([#2086](https://github.com/containers/podman/issues/2086))
- Fixed a bug where `podman container runlabel` will return a non-0 exit code if the label does not exist
- Fixed a bug where container state was always reset to Created after a reboot ([#1703](https://github.com/containers/podman/issues/1703))
- Fixed a bug where `/dev/pts` was unconditionally overridden in rootless Podman, which was unnecessary except in very specific cases
- Fixed a bug where Podman run as root was ignoring some options in `/etc/containers/storage.conf` ([#2217](https://github.com/containers/podman/issues/2217))
- Fixed a bug where Podman cleanup processes were not being given the proper OCI runtime path if a custom one was specified
- Fixed a bug where `podman images --filter dangling=true` would crash if no dangling images were present ([#2246](https://github.com/containers/podman/issues/2246))
- Fixed a bug where `podman ps --format "{{.Mounts}}"` would not display a container's mounts ([#2238](https://github.com/containers/podman/issues/2238))
- Fixed a bug where `podman pod stats` was ignoring Go templates specified by `--format` ([#2258](https://github.com/containers/podman/issues/2258))
- Fixed a bug where `podman generate kube` would fail on containers with `--user` specified ([#2304](https://github.com/containers/podman/issues/2304))
- Fixed a bug where `podman images` displayed incorrect output for images pulled by digest ([#2175](https://github.com/containers/podman/issues/2175))
- Fixed a bug where `podman port` and `podman ps` did not properly display ports if the container joined a network namespace from a pod or another container ([#846](https://github.com/containers/podman/issues/846))
- Fixed a bug where detaching from a container using the detach keys would cause Podman to hang until the container exited
- Fixed a bug where `podman create --rm` did not work with `podman start --attach`
- Fixed a bug where invalid named volumes specified in `podman create` and `podman run` could cause segfaults ([#2301](https://github.com/containers/podman/issues/2301))
- Fixed a bug where the `runtime` field in `libpod.conf` was being ignored. `runtime` is legacy and deprecated, but will continue to be respected for the foreseeable future
- Fixed a bug where `podman login` would sometimes report it logged in successfully when it did not
- Fixed a bug where `podman pod create` would not error on receiving unused CLI argument
- Fixed a bug where rootless `podman run` with the `--pod` argument would fail if the pod was stopped
- Fixed a bug where `podman images` did not print a trailing newline when not invoked on a TTY ([#2388](https://github.com/containers/podman/issues/2388))
- Fixed a bug where the `--runtime` option was sometimes not overriding `libpod.conf`
- Fixed a bug where `podman pull` and `podman runlabel` would sometimes exit with 0 when they should have exited with an error ([#2405](https://github.com/containers/podman/issues/2405))
- Fixed a bug where rootless `podman export -o` would fail ([#2381](https://github.com/containers/podman/issues/2381))
- Fixed a bug where read-only volumes would fail in rootless Podman when the volume originated on a filesystem mounted `nosuid`, `nodev`, or `noexec` ([#2312](https://github.com/containers/podman/issues/2312))
- Fixed a bug where some files used by checkpoint and restore received improper SELinux labels ([#2334](https://github.com/containers/podman/issues/2334))
- Fixed a bug where Podman's volume path was not properly changed when containers/storage changed location ([#2395](https://github.com/containers/podman/issues/2395))

### Misc
- Podman migrated to a new, shared memory locking model in this release. As part of this, if you are running Podman with pods or dependency containers (e.g. `--net=container:`), you should run the `podman system renumber` command to migrate your containers to the new model - please reference the `podman-system-renumber(1)` man page for further details
- Podman migrated to a new command-line parsing library, and the output format of help and usage text has somewhat changed as a result
- Updated Buildah to v1.7, picking up a number of bugfixes
- Updated containers/image library to v1.5, picking up a number of bugfixes and performance improvements to pushing images
- Updated containers/storage library to v1.10, picking up a number of bugfixes
- Work on the remote Podman client for interacting with Podman remotely over Varlink is progressing steadily, and many image and pod commands are supported - please see the [Readme](https://github.com/containers/podman/blob/main/remote_client.md) for details
- Added path masking to mounts with the `:z` and `:Z` options, preventing users from accidentally performing an SELinux relabel of their entire home directory
- The `podman container runlabel` command will not pull an image if it does not contain the requested label
- Many commands' usage information now includes examples
- `podman rm` can now delete containers in containers/storage, which can be used to resolve some situations where Podman fails to remove a container
- The `podman search` command now searches multiple registries in parallel for improved performance
- The `podman build` command now defaults `--pull-always` to true
- Containers which share a network namespace (for example, when in a pod) will now share /etc/hosts and /etc/resolv.conf between all containers in the pod, causing changes in one container to propagate to all containers sharing their networks
- The `podman rm` and `podman rmi` commands now return 1 (instead of 127) when all specified container or images are missing

## 1.0.0
### Features
- The `podman exec` command now includes a `--workdir` option to set working directory for the executed command
- The `podman create` and `podman run` commands now support the `--init` flag to use a minimal init process in the container
- Added the `podman image sign` command to GPG sign images
- The `podman run --device` flag now accepts directories, and will added any device nodes in the directory to the container
- Added the `podman play kube` command to create pods and containers from Kubernetes pod YAML

### Bugfixes
- Fixed a bug where passing `podman create` or `podman run` volumes with an empty host or container path could cause a segfault
- Fixed a bug where `storage.conf` was sometimes ignored for rootless containers
- Fixed a bug where Podman run as root would error if CAP_SYS_RESOURCE was not available
- Fixed a bug where Podman would fail to start containers after a system restart due to an out-of-date default Apparmor profile
- Fixed a bug where Podman's bash completions were not working
- Fixed a bug where `podman login` would use existing login credentials even if new credentials were provided
- Fixed a bug where Podman could create some directories with the wrong permissions, breaking containers with user namespaces
- Fixed a bug where `podman runlabel` was not properly setting container names when the `--name` was specified
- Fixed a bug where `podman runlabel` sometimes included extra spaces in command output
- Fixed a bug where `podman commit` was including invalid port numbers in created images when committing containers with published ports
- Fixed a bug where `podman exec` was not honoring the container's environment variables
- Fixed a bug where `podman run --device` would fail when a symlink to a device was specified
- Fixed a bug where `podman build` was not properly picking up OCI runtime paths specified in `libpod.conf`
- Fixed a bug where Podman would mount `/dev/shm` into the container read-only for read-only containers (`/dev/shm` should always be read-write)
- Fixed a bug where Podman would ignore any mount whose container mountpoint was `/dev/shm`
- Fixed a bug where `podman export` did not work with the default `fuse-overlayfs` storage driver
- Fixed a bug where `podman inspect -f '{{ json .Config }}'` on images would not output anything (it now prints the image's config)
- Fixed a bug where `podman rmi -fa` displayed the wrong error message when trying to remove images used by pod infra containers

### Misc
- Rootless containers now unconditionally use postrun cleanup processes, ensuring resources are freed when the container stops
- A new version of Buildah is included for `podman build`, featuring improved build speed and numerous bugfixes
- Pulling images has been parallelized, allowing individual layers to be pulled in parallel
- The `podman start --attach` command now defaults the `sig-proxy` option to `true`, matching `podman create` and `podman run`
- The `podman info` command now prints the path of the configuration file controlling container storage
- Added `podman list` and `podman ls` as aliases for `podman ps`, and `podman container ps` and `podman container list` as aliases for `podman container ls`
- Changed `podman generate kube` to generate Kubernetes service YAML in the same file as pod YAML, generating a single file instead of two
- To improve compatibility with the Docker command line, `podman inspect -f '{{ json .ContainerConfig }}'` on images is no longer valid; please use `podman inspect -f '{{ json .Config }}'` instead

## 0.12.1.2
### Bugfixes
- Fixed a bug where an empty path for named volumes could make it impossible to create containers
- Fixed a bug where containers using another container's network namespace would not also use the other container's /etc/hosts and /etc/resolv.conf
- Fixed a bug where containers with `--rm` which failed to start were not removed
- Fixed a potential race condition attempting to read `/etc/passwd` inside containers

## 0.12.1.1
### Features
- Added the `podman generate kube` command to generate Kubernetes Pod and Service YAML for Podman containers and pods
- The `podman pod stop` flag now accepts a `--timeout` flag to set the timeout for stopping containers in the pod

### Bugfixes
- Fixed a bug where rootless Podman would fail to start if the default OCI hooks directory is not present

## 0.12.1
### Features
- Rootless Podman now creates the storage.conf, libpod.conf, and mounts.conf configuration files automatically in `~/.config/containers/` for ease of reconfiguration
- The `podman pod create` command can expose ports in the pod's network namespace, allowing public services to be created in pods
- The `podman container checkpoint` command can now keep containers running after they are checkpointed with the `--leave-running` flag
- The `podman container checkpoint` and `podman container restore` commands now support the `--tcp-established` flag to checkpoint and restore containers with active TCP connections
- The `podman version` command now has a `--format` flag to produce machine-readable output
- Added the `podman container exists`, `podman pod exists`, and `podman image exists` commands to easily check for a container/pod/image, respectively, by name or ID
- The `podman ps --pod` flag now has a short alias, `-p`
- The `podman rmi` and `podman rm` commands now have a `--prune` flag to prune unused images and containers, respectively
- The `podman ps` command now has a `--sync` flag to force a sync of Podman's state against the OCI runtime, resolving some state desync errors
- Added the `podman volume` set of commands for creating and managing local-only named volumes

### Bugfixes
- Fixed a breaking change in rootless Podman where a change in default paths caused Podman to be unable to function on systems upgraded from 0.10.x or earlier
- Fixed a bug where `podman exec` without `-t` would still use a terminal if the container was created with `-t`
- Fixed a bug where container root propagation was not being properly adjusted if volumes with root propagation set were mounted into the container
- Fixed a bug where `podman exec` could hold the container lock longer than necessary waiting for an exited container
- Fixed a bug where rootless containers using `slirp4netns` for networking were reporting using `bridge` networking in `podman inspect`
- Fixed a bug where `podman container restore -a` was attempting to restore all containers, including created and running ones. It will now only attempt to restore stopped and exited containers
- Fixed a bug where rootless Podman detached containers were not being properly cleaned up
- Fixed a bug where privileged containers were being mounted with incorrect (too restrictive) mount options such as `nodev`
- Fixed a bug where `podman stop` would throw an error attempting to stop a container that had already stopped
- Fixed a bug where `NOTIFY_SOCKET` was not properly being passed into Podman containers
- Fixed a bug where `/dev/shm` was not properly mounted in rootless containers
- Fixed a bug where rootless Podman would set up the CNI plugins for networking (despite not using them in rootless mode), potentially causing `inotify` related errors
- Fixed a bug where Podman would error on numeric GIDs that do not exist in the container's `/etc/group`
- Fixed a bug where containers in pods or created with `--net=container` were not mounting `/etc/resolv.conf` and `/etc/hosts`

### Misc
- `podman build` now defaults the `--force-rm` flag to `true`
- Improved `podman runlabel` support for labels featuring arguments with whitespace
- Containers without a network namespace will now use the host's `resolv.conf`
- The `slirp4netns` network mode can now be used with containers running as root. It may be useful for container-in-container scenarios where the outer container does not have host networking set
- Podman now uses `inotify` to wait for container exit files to be created, instead of polling. If `inotify` cannot be used, Podman will fall back to polling to check if the file has been created
- The `podman logs` command now uses improved short-options handling, allowing its flags to be combined if desired (for example, `podman logs -lf` instead of `podman logs -l -f`)
- Hardcoded OCI hooks directories used by Podman are now deprecated; they should instead be coded into the `libpod.conf` configuration file. They can be specified as an array via `hooks_dir`

## 0.11.1.1
### Bugfixes
- Fixed a bug where Podman was not correctly adding firewall rules for containers, preventing them from accessing the network
- Fixed a bug where full error messages were being lost when creating containers with user namespaces
- Fixed a bug where container state was not properly updated if a failure occurred during network setup, which could cause mounts to be left behind when the container was removed
- Fixed a bug where `podman exec` could time out on slower systems by increasing the relevant timeout

### Misc
- `podman rm -f` now removes paused containers. As such, `podman rm -af` completing successfully guarantees all Podman containers have been removed
- Added a field to `podman info` to show if Podman is being run as rootless
- Made a small output format change to `podman images` - image sizes now feature a space between number and unit (e.g. `123 MB` now instead of `123MB`)
- Vendored an updated version of `containers/storage` to fix several bugs reported upstream

## 0.11.1
### Features
- Added `--all` and `--latest` flags to `podman checkpoint` and `podman restore`
- Added `--max-workers` flag to all Podman commands that support operating in parallel, allowing the maximum number of parallel workers used to be specified
- Added `--all` flag to `podman restart`

### Bugfixes
- Fixed a bug where `podman port -l` would segfault if no containers were present
- Fixed a bug where `podman stats -a` would error if containers were present but not running
- Fixed a bug where container status checks would sometimes leave zombie OCI runtime processes
- Fixed checkpoint and restore code to verify an appropriate version of `criu` is being used
- Fixed a bug where environment variables with no specified value (e.g. `-e FOO`) caused errors (they are now added as empty)
- Fixed a bug where rootless Podman would attempt to configure the system firewall, causing errors on some systems where iptables is not in the user's PATH
- Fixed a bug where rootless Podman was unable to successfully write the container ID to a file when `--cid-file` was specified to `podman run`
- Fixed a bug where `podman unmount` would refuse to unmount a container if it was running (the unmount will now be deferred until the container stops)
- Fixed a bug where rootless `podman attach` would fail to attach due to a too-long path name
- Fixed a bug where `podman info` was not properly reporting the Git commit Podman was built from
- Fixed a bug where `podman run --interactive` was not holding STDIN open when `-a` flag was specified
- Fixed a bug where Podman with the `cgroupfs` CGroup driver was sometimes not successfully removing pod CGroups
- Fixed a bug where rootless Podman was unable to run systemd containers (note that this also requires an update to systemd)
- Fixed a bug where `podman run` with the `--user` flag would fail if the container image did not contain `/etc/passwd` or `/etc/group`

### Misc
- `podman rm`, `podman restart`, `podman kill`, `podman pause`, and `podman unpause` now operate in parallel, greatly improving speed when multiple containers are specified
- `podman create`, `podman run`, and `podman ps` have a number of improvements which should greatly increase their speed
- Greatly improved performance and reduced memory utilization of container status checks, which should improve the speed of most Podman commands
- Improve ability of `podman runlabel` to run commands that are not Podman
- Podman containers with an IP address now add their hostnames to `/etc/hosts`
- Changed default location of temporary libpod files in rootless Podman
- Updated the default Podman seccomp profile

### Compatibility
Several paths related to rootless Podman had their default values changed in this release.
If paths were not hardcoded in libpod.conf, your system may lose track of running containers and believe they are newly-created.

## 0.10.1.3
### Bugfixes
- Fixed a bug where `podman build` would not work while any containers were running

## 0.10.1.2
### Bugfixes
- Fixed cgroup mount for containers using systemd as init to work properly with the systemd cgroup manager

## 0.10.1.1
### Features
- Added handling for running containers as users with numeric UIDs not present in the container's /etc/passwd. This allows getpwuid() to work inside these containers.
- Added support for the REGISTRY_AUTH_FILE environment variable, which specifies the location of credentials for registry login. This is supported by the `push`, `pull`, `login`, `logout`, `runlabel`, and `search` commands

### Bugfixes
- Fixed handling for image volumes which are mounted on symlinks. The links are now resolved within the container, not on the host
- Fixed mounts for containers that use systemd as init to properly include all mounts required by systemd to function

### Misc
- Updated vendored version of Buildah used to power `podman build`

## 0.10.1
### Features
- Added the `podman container checkpoint` and `podman container restore` commands to checkpoint and restore containers
- Added the `podman container runlabel` command to run containers based on commands contained in their images
- Added the `podman create --ip` and `podman run --ip` flags to allow setting static IPs for containers
- Added the `podman kill --all` flag to send a signal to all running containers

### Bugfixes
- Fixed Podman cleanup processes for detached containers to properly print debug information when `--syslog` flag is specified
- Fixed manpages for `podman create` and `podman run` to document existing `--net` flag as an alias for `--network`
- Fixed issues with rootless Podman where specifying a single user mapping container was causing all Podman commands to hang
- Fixed an issue with rootless Podman not properly detecting when user namespaces were not enabled
- Fixed an issue where Podman user namespaces were not preserving file capabilities
- Fixed an issue where `resolv.conf` in container would unconditionally forward nameservers into the container, even localhost
- Fixed containers to release resources in the OCI runtime immediately after exiting, improving compatibility with Kata containers
- Fixed OCI runtime handling to fix several issues when using gVisor as an OCI runtime
- Fixed SELinux relabel errors when starting containers after a system restart
- Fixed a crash when initializing hooks on containers running systemd as init
- Fixed an SELinux labelling issue with privileged containers
- Fixed rootless Podman to raise better errors when using CGroup resource limits, which are not currently compatible with rootless
- Fixed a crash when runc was used as the OCI runtime for containers running systemd as init
- Fixed SELinux labelling for containers run with `--security-opt label=disable` to assign the correct label

### Misc
- Changed flag ordering on all Podman commands to ensure flags are alphabetized
- Changed `podman stop` to work in parallel when multiple containers are specified, greatly speeding up stop for containers that do not stop after SIGINT
- Updated vendored version of Buildah used to power `podman build`
- Added version of vendored Buildah to `podman info` to better debug issues

## 0.9.3.1
### Bugfixes
- Fixed a critical issue where SELinux contexts set on tmpfs volumes were causing runc crashes

## 0.9.3
### Features
- Added a flag to `libpod.conf`, `label`, to globally enable/disable SELinux labelling for libpod
- Added `--mount` flag to `podman create` and `podman run` as a new, more explicit way of specifying volume mounts

### Bugfixes
- Fixed a crash during container creation when an image had no names
- Fixed default rootfs mount propagation to for containers to match Docker
- Fixed permissions of `/proc` in containers
- Fixed permissions of some default bind mounts (for example, `/etc/hosts`) in read-only containers
- Fixed `/dev/shm` in `--ipc=container` and `--ipc=host` containers to use the correct SHM
- Fixed rootless Podman to properly join the namespaces of other containers
- Fixed the output of `podman diff` to not display some default changes that will not be committed
- Fixed rootless to better handle cases where insufficient UIDs/GIDs are mapped into the container

## 0.9.2.1
### Bugfixes
- Updated Buildah dependency to fix several bugs in `podman build`

### Misc
- Small performance improvement in image handling code to not recalculate digests

## 0.9.2
### Features
- Added `--interval` flag to `podman wait` to determine the interval between checks for container status
- Added a switch in `libpod.conf` to disable reserving ports for running containers. This lowers the safety of port allocations, but can significantly reduce memory usage.
- Added ability to search all the contents of a registry if no image name is specified when using `podman search`

### Bugfixes
- Further fixes for sharing of UTS namespaces within pods
- Fixed a deadlock in containers/storage that could be caused by numerous parallel Podman processes.
- Fixed Podman running into open file limits when many ports are forwarded
- Fixed default mount propagation on volume mounts
- Fixed default mounts under /dev remaining if /dev is bind-mounted into the container
- Fixed rootless `podman create` with no command specified throwing an error

### Misc
- Added `podman rm --volumes` flag for compatibility with Docker. As Podman does not presently support named volumes, this does nothing for now, but provides improved compatibility with the Docker command line.
- Improved error messages from `podman pull`

### Compatibility
- Podman is no longer being built by default with support for the Devicemapper storage driver. If you are using this storage driver, you should investigate switching to overlayfs.

## 0.9.1.1
### Bugfixes
- Added support for configuring iptables and firewalld firewalls to allow container traffic. This should resolve numerous issues with network access in containers.

### Note
It is recommended that you restart your system firewall after installing this release to clear any firewall rules created by older Podman versions. If port forwarding to containers does not work, it is recommended that you restart your system.

## 0.9.1
### Features
- Added initial support for the `podman pod` command as non-root

### Bugfixes
- Fixed regression where invalid Podman commands would still cause a clean exit
- Fixed `podman rmi --all` to not error if no images are present on the system
- Fixed parsing of container logs with `podman logs` to properly handle CRI logging, fixing some issues with blank lines in logs
- Fixed a bug creating pod cgroups using the systemd cgroup driver with systemd versions 239 and higher
- Fixed handling of volume mounts that overlapped with default container mounts (for example, `podman run -v /dev/:/dev`)
- Fixed sharing of UTS namespace in pods

### Misc
- Added additional debug information when pulling images if `--log-level=debug` is specified
- `podman build` now defaults to caching intermediate layers while building

## 0.8.5
### Features
- Added the ability to add a multipart entrypoint with `podman run --entrypoint`
- Improved help text when invalid commands are specified
- Greatly improved support for containers which use systemd as init

### Bugfixes
- Fixed several bugs with rootless `podman exec`
- Fixed rootless `podman` with a symlinked storage directory crashing
- Fixed bug with `podman ps` and multiple filters where the interface did not match Docker
- Fixed handling of `resolv.conf` on the host to handle symlinks
- Increased open file descriptor and process limits to match Docker and Buildah
- Fixed `podman run -h` to specify the container's hostname (as it does in Docker) instead of printing help text
- Fixed a bug with image shortname handling where repositories were incorrectly being treated as registries
- Fixed a bug where `podman wait` was busywaiting and consuming large amounts of CPU

## 0.8.4
### Features
- Added the `podman pod top` command
- Added the ability to easily share namespaces within a pod
- Added a pod statistics endpoint to the Varlink API
- Added information on container capabilities to the output of `podman inspect`

### Bugfixes
- Fixed a bug with the --device flag in `podman run` and `podman create`
- Fixed `podman pod stats` to accept partial pod IDs and pod names
- Fixed a bug with OCI hooks handling `ALWAYS` matches
- Fixed a bug with privileged rootless containers with `--net=host` set
- Fixed a bug where `podman exec --user` would not work with usernames, only numeric IDs
- Fixed a bug where Podman was forwarding both TCP and UDP ports to containers when protocol was not specified
- Fixed issues with Apparmor in rootless containers
- Fixed an issue with database encoding causing some containers created by Podman versions 0.8.1 and below to be unusable.

### Compatibility:
We switched JSON encoding/decoding to a new library for this release to address a compatibility issue introduced by v0.8.2.
However, this may cause issues with containers created in 0.8.2 and 0.8.3 with custom DNS servers.
