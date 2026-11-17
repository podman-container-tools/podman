####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Volume=[HOST-DIR:CONTAINER-DIR[:OPTIONS]]`
<< else >>
#### **--volume**, **-v**=*[HOST-DIR:CONTAINER-DIR[:OPTIONS]]*
<< endif >>

Mount a host directory into containers when executing RUN instructions during
the build.

<< if is_quadlet >>
Special case:

* If `SOURCE-VOLUME` ends with `.volume`, Quadlet will look for the corresponding `.volume` Quadlet unit. If found, Quadlet will use the name of the Volume set in the Unit, otherwise, `systemd-$name` is used. The generated systemd service contains a dependency on the service unit generated for that `.volume` unit, or on `$name-volume.service` if the `.volume` unit is not found. Note: the corresponding `.volume` file must exist.
<< endif >>

The `OPTIONS` are a comma-separated list and can be one or more of:

   * [rw|ro]
   * [z|Z|O]
   * [U]
   * [`[r]shared`|`[r]slave`|`[r]private`]<sup>[[1]](#Footnote1)</sup>

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The `HOST-DIR`
must be an absolute path as well. Podman bind-mounts the `HOST-DIR` to the
specified path when processing RUN instructions.

You can specify multiple  **-v** options to mount one or more mounts.

You can add the `:ro` or `:rw` suffix to a volume to mount it read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

`Chowning Volume Mounts`

By default, Podman does not change the owner and group of source volume
directories mounted. When running using user namespaces, the UID and GID inside
the namespace may correspond to another UID and GID on the host.

The `:U` suffix tells Podman to use the correct host UID and GID based on the
UID and GID within the namespace, to change recursively the owner and group of
the source volume.

**Warning** use with caution since this modifies the host filesystem.

`Labeling Volume Mounts`

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the container context, add one of these two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The `z` option tells Podman that two containers
share the volume content. As a result, Podman labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Podman to label the content with a private unshared label.
Only the current container can use a private volume.

Note: Do not relabel system files and directories. Relabeling system content
might cause other confined services on the host machine to fail.  For these types
of containers, disabling SELinux separation is recommended.  The option
`--security-opt label=disable` disables SELinux separation for the container.
For example, if a user wanted to volume mount their entire home directory into the build containers, they need to disable SELinux separation.

    $ podman build --security-opt label=disable -v $HOME:/home/user .

`Overlay Volume Mounts`

The `:O` flag tells Podman to mount the directory from the host as a
temporary storage using the Overlay file system. The `RUN` command containers
are allowed to modify contents within the mountpoint and are stored in the
container storage in a separate directory.  In Overlay FS terms the source
directory is the lower, and the container storage directory is the
upper. Modifications to the mount point are destroyed when the `RUN` command
finishes executing, similar to a tmpfs mount point.

Any subsequent execution of `RUN` commands sees the original source directory
content, any changes from previous RUN commands no longer exists.

One use case of the `overlay` mount is sharing the package cache from the
host into the container to allow speeding up builds.

Note:

- Overlay mounts are not currently supported in rootless mode.
- The `O` flag is not allowed to be specified with the `Z` or `z` flags.
  Content mounted into the container is labeled with the private label.
  On SELinux systems, labels in the source directory needs to be readable
  by the container label. If not, SELinux container separation must be disabled
  for the container to work.
- Modification of the directory volume mounted into the container with an
  overlay mount can cause unexpected failures. Do not modify the directory until
  the container finishes running.

By default bind mounted volumes are `private`. That means any mounts done
inside containers are not be visible on the host and vice versa. This behavior
can be changed by specifying a volume mount propagation property.

When the mount propagation policy is set to `shared`, any mounts completed
inside the container on that volume is visible to both the host and
container. When the mount propagation policy is set to `slave`, one way mount
propagation is enabled and any mounts completed on the host for that volume is
visible only inside of the container. To control the mount propagation
property of volume use the `:[r]shared`, `:[r]slave` or `:[r]private`
propagation flag. For mount propagation to work on the source mount point (mount
point where source dir is mounted on) has to have the right propagation properties.
For shared volumes, the source mount point has to be shared. And for slave volumes,
the source mount has to be either shared or slave. <sup>[[1]](#Footnote1)</sup>

Use `df <source-dir>` to determine the source mount and then use
`findmnt -o TARGET,PROPAGATION <source-mount-dir>` to determine propagation
properties of source mount, if `findmnt` utility is not available, the source
mount point can be determined by looking at the mount entry in
`/proc/self/mountinfo`. Look at `optional fields` and see if any propagation
properties are specified.
`shared:X` means the mount is `shared`, `master:X` means the mount is `slave`
and if nothing is there that means the mount is `private`. <sup>[[1]](#Footnote1)</sup>

To change propagation properties of a mount point use the `mount` command. For
example, to bind mount the source directory `/foo` do
`mount --bind /foo /foo` and `mount --make-private --make-shared /foo`. This
converts /foo into a `shared` mount point.  The propagation properties of
the source mount can be changed directly. For instance if `/` is the source
mount for `/foo`, then use `mount --make-shared /` to convert `/` into a
`shared` mount.
