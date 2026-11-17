####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `GIDMap=[flags]container_gid:from_gid[:amount]`
<< else >>
#### **--gidmap**=*[flags]container_gid:from_gid[:amount]*
<< endif >>

Run the container in a new user namespace using the supplied GID mapping. This
option conflicts with the << '**UserNS=**' if is_quadlet else '**--userns**' >> and
<< '**SubGIDMap=**' if is_quadlet else '**--subgidname**' >> options. This
option provides a way to map host GIDs to container GIDs in the same way as
__--uidmap__ maps host UIDs to container UIDs. For details see __--uidmap__.

Note: the << '**GIDMap=**' if is_quadlet else '**--gidmap**' >> option cannot be
called in conjunction with the << '**Pod=**' if is_quadlet else '**--pod**' >> option as
a gidmap cannot be set on the container level when in a pod.
