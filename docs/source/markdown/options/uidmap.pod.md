####> This option file is used in:
####>   podman pod clone, pod create, podman-pod.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `UIDMap=container_uid:from_uid[:amount]`
<< else >>
#### **--uidmap**=*container_uid:from_uid[:amount]*
<< endif >>

Run all containers in the pod in a new user namespace using the supplied mapping. This
option conflicts with the << '**UserNS=**' if is_quadlet else '**--userns**' >> and << '**SubUIDMap=**' if is_quadlet else '**--subuidname**' >> options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.
