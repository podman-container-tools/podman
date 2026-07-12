####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Rootfs=`
<< else >>
#### **--rootfs**
<< endif >>

If specified, the first argument refers to an exploded container on the file system.

<< if is_quadlet >>
This option conflicts with the `Image` option.

<< endif >>
This is useful to run a container without requiring any image management, the rootfs
of the container is assumed to be managed externally.

  `Overlay Rootfs Mounts`

   The `:O` flag tells Podman to mount the directory from the rootfs path as
storage using the `overlay file system`. The container processes
can modify content within the mount point which is stored in the
container storage in a separate directory. In overlay terms, the source
directory is the lower, and the container storage directory is the
upper. Modifications to the mount point are destroyed when the container
finishes executing, similar to a tmpfs mount point being unmounted.

Note: On **SELinux** systems, the rootfs needs the correct label, which is by default
**unconfined_u:object_r:container_file_t:s0**.

  `idmap`

If `idmap` is specified, create an idmapped mount to the target user
namespace in the container.
The idmap option supports a custom mapping that can be different than the user
namespace used by the container.  The mapping can be specified after the idmap
option like: `idmap=uids=0-1-10#10-11-10;gids=0-100-10`.  For each triplet, the
first value is the start of the backing file system IDs that are mapped to the
second value on the host.  The length of this mapping is given in the third value.
Multiple ranges are separated with #.
