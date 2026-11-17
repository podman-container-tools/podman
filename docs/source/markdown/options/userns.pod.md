####> This option file is used in:
####>   podman pod clone, pod create, podman-pod.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `UserNS=mode`
<< else >>
#### **--userns**=*mode*
<< endif >>

Set the user namespace mode for all the containers in a pod. It defaults to the `PODMAN_USERNS` environment variable. An empty value ("") means user namespaces are disabled.

Rootless user << '**UserNS=Key**' if is_quadlet else '**--userns=Key**' >> mappings:

Key       | Host User |  Container User
----------|---------------|---------------------
""        |$UID           |0 (Default User account mapped to root user in container.)
host      |$UID           |0 (Default User account mapped to root user in container.)
keep-id   |$UID           |$UID (Map user account to same UID within container.)
auto      |$UID           | nil (Host User UID is not mapped into container.)
nomap     |$UID           | nil (Host User UID is not mapped into container.)

Valid _mode_ values are:

  - *auto[:*_OPTIONS,..._*]*: automatically create a namespace. It is possible to specify these options to `auto`:

    - *gidmapping=*_CONTAINER\_GID:HOST\_GID:SIZE_ to force a GID mapping to be present in the user namespace.

    - *size=*_SIZE_: to specify an explicit size for the automatic user namespace. e.g. `<< 'UserNS=' if is_quadlet else '--userns=' >>auto:size=8192`. If `size` is not specified, `auto` estimates the size for the user namespace.

    - *uidmapping=*_CONTAINER\_UID:HOST\_UID:SIZE_ to force a UID mapping to be present in the user namespace.

  - *host*: run in the user namespace of the caller. The processes running in the container have the same privileges on the host as any other process launched by the calling user (default).

  - *keep-id*: creates a user namespace where the current rootless user's UID:GID are mapped to the same values in the container. This option is not allowed for containers created by the root user.

  - *nomap*: creates a user namespace where the current rootless user's UID:GID are not mapped into the container. This option is not allowed for containers created by the root user.
