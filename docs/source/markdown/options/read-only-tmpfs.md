####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ReadOnlyTmpfs=true|false`
<< else >>
#### **--read-only-tmpfs**
<< endif >>

When running --read-only containers, mount a read-write tmpfs on _/dev_, _/dev/shm_, _/run_, _/tmp_, and _/var/tmp_. The default is **true**.

| --read-only | --read-only-tmpfs |  /   | /run, /tmp, /var/tmp|
| ----------- | ----------------- | ---- | ----------------------------------- |
| true        |  true             | r/o  | r/w                                 |
| true        |  false            | r/o  | r/o                                 |
| false       |  false            | r/w  | r/w                                 |
| false       |  true             | r/w  | r/w                                 |


When << '**ReadOnly=true**' if is_quadlet else '**--read-only=true**' >> and
<< '**ReadOnlyTmpfs=true**' if is_quadlet else '**--read-only-tmpfs=true**' >> additional tmpfs are mounted on
the /tmp, /run, and /var/tmp directories.

When << '**ReadOnly=true**' if is_quadlet else '**--read-only=true**' >> and
<< '**ReadOnlyTmpfs=false**' if is_quadlet else '**--read-only-tmpfs=false**' >> /dev and /dev/shm are marked
Read/Only and no tmpfs are mounted on /tmp, /run and /var/tmp. The directories
are exposed from the underlying image, meaning they are read-only by default.
This makes the container totally read-only. No writable directories exist within
the container. In this mode writable directories need to be added via external
volumes or mounts.

By default, when << '**ReadOnly=false**' if is_quadlet else '**--read-only=false**' >> , the /dev and /dev/shm are read/write, and the /tmp, /run, and /var/tmp are read/write directories from the container image.
