####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `User=user[:group]`
<< else >>
#### **--user**, **-u**=*user[:group]*
<< endif >>

Sets the username or UID used and, optionally, the groupname or GID for the specified command. Both *user* and *group* may be symbolic or numeric.

Without this argument, the command runs as the user specified in the container image. Unless overridden by a `USER` command in the Containerfile or by a value passed to this option, this user generally defaults to root.

When a user namespace is not in use, the UID and GID used within the container and on the host match. When user namespaces are in use, however, the UID and GID in the container may correspond to another UID and GID on the host. In rootless containers, for example, a user namespace is always used, and root in the container by default corresponds to the UID and GID of the user invoking Podman.
