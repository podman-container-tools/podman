####> This option file is used in:
####>   podman pod clone, pod create
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--gidmap**=*container_gid:from_gid[:amount]*

Run all containers in the pod in a new user namespace using the supplied GID
mapping. This option conflicts with the **--userns** option. It provides a way
to map host GIDs to container GIDs. It can be passed several times to map
different ranges.
