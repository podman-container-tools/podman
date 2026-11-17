####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ReadOnly=`
<< else >>
#### **--read-only**
<< endif >>

Mount the container's root filesystem as read-only.

By default, container root filesystems are writable, allowing processes
to write files anywhere. By specifying the << '**ReadOnly=**' if is_quadlet else '**--read-only**' >> flag,
the container's root filesystem is mounted read-only prohibiting any writes.
