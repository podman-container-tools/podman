####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, kube play, podman-kube.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `UserNS=mode`
<< else >>
#### **--userns**=*mode*
<< endif >>

Set the user namespace mode for the container.

If `--userns` is not set, the default value is determined as follows.
- If `--pod` is set, `--userns` is ignored and the user namespace of the pod is used.
- If the environment variable **PODMAN_USERNS** is set its value is used.
- If `userns` is specified in `containers.conf` this value is used.
- Otherwise, `--userns=host` is assumed.

`--userns=""` (i.e., an empty string) is an alias for `--userns=host`.

This option is incompatible with << '**GIDMap=**' if is_quadlet else '**--gidmap**' >>, << '**UIDMap=**' if is_quadlet else '**--uidmap**' >>, << '**SubUIDMap=**' if is_quadlet else '**--subuidname**' >> and << '**SubGIDMap=**' if is_quadlet else '**--subgidname**' >>.

Rootless user --userns=Key mappings:

Key                     | Host User | Container User
------------------------|-----------|---------------------
auto                    | $UID      | nil (Host User UID is not mapped into container.)
host                    | $UID      | 0 (Default User account mapped to root user in container.)
keep-id                 | $UID      | $UID (Map user account to same UID within container.)
keep-id:uid=200,gid=210 | $UID      | 200:210 (Map user account to specified UID, GID value within container.)
nomap                   | $UID      | nil (Host User UID is not mapped into container.)

Valid _mode_ values are:

**auto**[:_OPTIONS,..._]: automatically create a unique user namespace.

* `rootful mode`: The `--userns=auto` flag requires that the user name __containers__ be specified in the /etc/subuid and /etc/subgid files, with an unused range of subordinate user IDs that Podman containers are allowed to allocate.  Example: `containers:2147483647:2147483648`.

* `rootless mode`: The users range from the /etc/subuid and /etc/subgid files will be used. Note running a single container without using --userns=auto will use the entire range of UIDs and not allow further subdividing. See subuid(5).

Podman allocates unique ranges of UIDs and GIDs from the `containers` subordinate user IDs. The size of the ranges is based on the number of UIDs required in the image. The number of UIDs and GIDs can be overridden with the `size` option.

The option `--userns=keep-id` uses all the subuids and subgids of the user.
The option `--userns=nomap` uses all the subuids and subgids of the user except the user's own ID.
Using `--userns=auto` when starting new containers does not work as long as any containers exist that were started with `--userns=nomap` or `--userns=keep-id` without limiting the user namespace size.

  Valid `auto` options:

  - *gidmapping*=_CONTAINER\_GID:HOST\_GID:SIZE_: to force a GID mapping to be present in the user namespace.
  - *size*=_SIZE_: to specify an explicit size for the automatic user namespace. e.g. `--userns=auto:size=8192`. If `size` is not specified, `auto` estimates a size for the user namespace.
  - *uidmapping*=_CONTAINER\_UID:HOST\_UID:SIZE_: to force a UID mapping to be present in the user namespace.

The host UID and GID in *gidmapping* and *uidmapping* can optionally be prefixed with the `@` symbol.
In this case, podman will look up the intermediate ID corresponding to host ID and it will map the found intermediate ID to the container id.
For details see << '**UIDMap=**' if is_quadlet else '**--uidmap**' >>.

**container:**_id_: join the user namespace of the specified container.

**host** or **""** (empty string): run in the user namespace of the caller. The processes running in the container have the same privileges on the host as any other process launched by the calling user.

**keep-id**: creates a user namespace where the current user's UID:GID are mapped to the same values in the container. For containers created by root, the current mapping is created into a new user namespace.

  In addition, the init process within the container will run under the current user's UID. This behavior overrides the image's `USER` instruction unless you explicitly set `--user`.

  Valid `keep-id` options:

  - *uid*=UID: override the UID inside the container that is used to map the current user to.
  - *gid*=GID: override the GID inside the container that is used to map the current user to.
  - *size*=SIZE: override the size of the configured user namespace.  It is useful to not saturate all the available IDs.  Not supported when running as root.

**nomap**: creates a user namespace where the current rootless user's UID:GID are not mapped into the container. This option is not allowed for containers created by the root user.

**ns:**_namespace_: run the <<container|pod>> in the given existing user namespace.
