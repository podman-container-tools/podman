####> This option file is used in:
####>   podman podman-build.unit.5.md.in, podman-container.unit.5.md.in, create, pod clone, pod create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Volume=[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]`
<< else >>
#### **--volume**, **-v**=*[[SOURCE-VOLUME|HOST-DIR:]CONTAINER-DIR[:OPTIONS]]*
<< endif >>

Create a bind mount. If `-v /HOST-DIR:/CONTAINER-DIR` is specified, Podman
bind mounts `/HOST-DIR` from the host into `/CONTAINER-DIR` in the Podman
container. Similarly, `-v SOURCE-VOLUME:/CONTAINER-DIR` mounts the named
volume from the host into the container. If no such named volume exists,
Podman creates one. The **nocreate** option can be used to disable this
behavior and require the volume to already exist. If no source is given,
the volume is created as an anonymously named volume with a randomly
generated name, and is removed when the <<container|pod>> is removed via
the `--rm` flag or the `podman rm --volumes` command.

<< if is_quadlet >>
Special case:

* If `SOURCE-VOLUME` ends with `.volume`, a Podman named volume called `systemd-$name` is used as the source, and the generated systemd service contains a dependency on the `$name-volume.service`. Note that the corresponding `.volume` file must exist.
<< endif >>

(Note when using the remote client, including Mac and Windows (excluding WSL2) machines, the volumes are mounted from the remote server, not necessarily the client machine.)

The _OPTIONS_ is a comma-separated list and can be one or more of:

* **rw**|**ro**
* **z**|**Z**
* [**O**]
* [**U**]
* [**no**]**copy**
* [**no**]**dev**
* [**no**]**exec**
* [**no**]**suid**
* [**r**]**bind**
* [**r**]**shared**|[**r**]**slave**|[**r**]**private**[**r**]**unbindable** <sup>[[1]](#Footnote1)</sup>
* **idmap**[=**options**]
* **nocreate**

The `CONTAINER-DIR` must be an absolute path such as `/src/docs`. The volume
is mounted into the container at this directory.

If a volume source is specified, it must be a path on the host or the name of a
named volume. Host paths are allowed to be absolute or relative; relative paths
are resolved relative to the directory Podman is run in. If the source does not
exist, Podman returns an error. Users must pre-create the source files or
directories.

Any source that does not begin with a `.` or `/` is treated as the name of
a named volume. If a volume with that name does not exist, it is created.
Volumes created with names are not anonymous, and they are not removed by the `--rm`
option and the `podman rm --volumes` command.

The **nocreate** option can be specified for named volumes to prevent automatic
volume creation. If **nocreate** is set and the volume does not exist, Podman
returns an error instead of creating the volume. This is useful when you want
to ensure that a volume was explicitly created before use.

    $ podman <<fullsubcommand>> -v myvolume:/data:nocreate alpine

Specify multiple **-v** options to mount one or more volumes into a
<<container|pod>>.

`Write Protected Volume Mounts`

Add **:ro** or **:rw** option to mount a volume in read-only or
read-write mode, respectively. By default, the volumes are mounted read-write.
See examples.

`Chowning Volume Mounts`

When a named volume is first mounted to a container, Podman
automatically adjusts the ownership of the volume's mount point during
container initialization. This chown operation occurs under the
following conditions:

- The volume was not used yet (has `NeedsChown` set to true)
- The volume is empty or has not been copied up yet
- The volume is not managed by an external volume driver
- The volume driver is not "image"

For volumes with idmapped mounts (using the `idmap` option), the
ownership change takes into account the container's user namespace
mappings, but the idmapped volume retains proper UID/GID mapping. For
volumes without idmapping, the mount point is chowned to match the
container's process user and group, mapped to the host user namespace
if user namespace remapping is enabled.

If a <<container|pod>> is created in a new user namespace, the UID and
GID in the container may correspond to another UID and GID on the host.

The `:U` suffix tells Podman to use the correct host UID and GID based on the
UID and GID within the <<container|pod>>, to change recursively the owner and
group of the source volume. Chowning walks the file system under the volume and
changes the UID/GID on each file. If the volume has thousands of inodes, this
process takes a long time, delaying the start of the <<container|pod>>.

**Warning** use with caution since this modifies the host filesystem.

`Labeling Volume Mounts`

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a <<container|pod>>. Without a label, the security system
might prevent the processes running inside the <<container|pod>> from using the
content. By default, Podman does not change the labels set by the OS.

To change a label in the <<container|pod>> context, add either of two suffixes
**:z** or **:Z** to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The **z** option tells Podman that two or more
<<containers|pods>> share the volume content. As a result, Podman labels the
content with a shared content label. Shared volume labels allow all containers
to read/write content. The **Z** option tells Podman to label the content with
a private unshared label. Only the current <<container|pod>> can use a private
volume.

Note: all containers within a `pod` share the same SELinux label. This
means all containers within said pod can read/write volumes shared into the
container created with the `:Z` on any one of the containers. Relabeling walks
the file system under the volume and changes the label on each file; if the
volume has thousands of inodes, this process takes a long time, delaying the
start of the <<container|pod>>. If the volume was previously relabeled with the
`z` option, Podman is optimized to not relabel a second time. If files are
moved into the volume, then the labels can be manually changed with the
`chcon -Rt container_file_t PATH` command.

Note: Do not relabel system files and directories. Relabeling system content
might cause other confined services on the machine to fail.  For these types
of containers we recommend disabling SELinux separation.  The option
**--security-opt label=disable** disables SELinux separation for the <<container|pod>>.
For example if a user wanted to volume mount their entire home directory into a
<<container|pod>>, they need to disable SELinux separation.

    $ podman <<fullsubcommand>> --security-opt label=disable -v $HOME:/home/user fedora touch /home/user/file

`Overlay Volume Mounts`

The `:O` flag tells Podman to mount the directory from the host as a
temporary storage using the `overlay file system`. The <<container|pod>> processes
can modify content within the mountpoint which is stored in the
container storage in a separate directory. In overlay terms, the source
directory is the lower, and the container storage directory is the
upper. Modifications to the mount point are destroyed when the <<container|pod>>
finishes executing, similar to a tmpfs mount point being unmounted.

For advanced users, the **overlay** option also supports custom non-volatile
**upperdir** and **workdir** for the overlay mount. Custom **upperdir** and
**workdir** can be fully managed by the users themselves, and Podman does not
remove it on lifecycle completion.
Example **:O,upperdir=/some/upper,workdir=/some/work**

Subsequent executions of the container sees the original source directory
content, any changes from previous <<container|pod>> executions no longer exist.

One use case of the overlay mount is sharing the package cache from the
host into the container to allow speeding up builds.

Note: The `O` flag conflicts with other options listed above.

Content mounted into the container is labeled with the private label.
On SELinux systems, labels in the source directory must be readable
by the <<|pod infra>> container label. Usually containers can read/execute `container_share_t`
and can read/write `container_file_t`. If unable to change the labels on a
source volume, SELinux container separation must be disabled for the <<|pod or infra>> container
to work.

Do not modify the source directory mounted into the <<container|pod>> with an overlay mount,
it can cause unexpected failures. Only modify the directory after the container finishes running.

`Mounts propagation`

By default, bind-mounted volumes are `private`. That means any mounts done
inside the <<container|pod>> are not visible on the host and vice versa.
One can change this behavior by specifying a volume mount propagation property.
When a volume is `shared`, mounts done under that volume inside the <<container|pod>>
are visible on host and vice versa. Making a volume **slave**<sup>[[1]](#Footnote1)</sup>
enables only one-way mount propagation: mounts done on the host under that volume
are visible inside the container but not the other way around.

To control mount propagation property of a volume one can use the [**r**]**shared**,
[**r**]**slave**, [**r**]**private** or the [**r**]**unbindable** propagation flag.
Propagation property can be specified only for bind mounted volumes and not for
internal volumes or named volumes. For mount propagation to work the source mount
point (the mount point where source dir is mounted on) has to have the right propagation
properties. For shared volumes, the source mount point has to be shared. And for
slave volumes, the source mount point has to be either shared or slave.
<sup>[[1]](#Footnote1)</sup>

To recursively mount a volume and all of its submounts into a
<<container|pod>>, use the **rbind** option. By default the bind option is
used, and submounts of the source directory is not mounted into the
<<container|pod>>.

Mounting the volume with a **copy** option tells podman to copy content from
the underlying destination directory onto newly created internal volumes. The
**copy** only happens on the initial creation of the volume. Content is not
copied up when the volume is subsequently used on different containers. The
**copy** option is ignored on bind mounts and has no effect.

Mounting volumes with the **nosuid** options means that SUID executables on the
volume can not be used by applications to change their privilege. By default
volumes are mounted with **nosuid**.

Mounting the volume with the **noexec** option means that no executables on the
volume can be executed within the <<container|pod>>.

Mounting the volume with the **nodev** option means that no devices on the volume
can be used by processes within the <<container|pod>>. By default volumes
are mounted with **nodev**.

If the _HOST-DIR_ is a mount point, then **dev**, **suid**, and **exec** options are
ignored by the kernel.

Use **df HOST-DIR** to figure out the source mount, then use
**findmnt -o TARGET,PROPAGATION _source-mount-dir_** to figure out propagation
properties of source mount. If **findmnt**(1) utility is not available, then one
can look at the mount entry for the source mount point in _/proc/self/mountinfo_. Look
at the "optional fields" and see if any propagation properties are specified.
In there, **shared:N** means the mount is shared, **master:N** means mount
is slave, and if nothing is there, the mount is private. <sup>[[1]](#Footnote1)</sup>

To change propagation properties of a mount point, use **mount**(8) command. For
example, if one wants to bind mount source directory _/foo_, one can do
**mount --bind /foo /foo** and **mount --make-private --make-shared /foo**. This
converts /foo into a shared mount point. Alternatively, one can directly
change propagation properties of source mount. Say _/_ is source mount for
_/foo_, then use **mount --make-shared /** to convert _/_ into a shared mount.

Note: if the user only has access rights via a group, accessing the volume
from inside a rootless <<container|pod>> fails.

`Idmapped mount`

If `idmap` is specified, create an idmapped mount to the target user
namespace in the container. The idmap option supports a custom mapping
that can be different than the user namespace used by the
container. The mapping can be specified after the idmap option like:
`idmap=uids=0-1-10#10-11-10;gids=0-100-10`.
For each triplet, the first value is the start of the backing file
system IDs that are mapped to the second value on the host.  The
length of this mapping is given in the third value.
Multiple ranges are separated with #.
