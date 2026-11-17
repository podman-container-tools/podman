####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, pod clone, pod create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `SubUIDMap=name`
<< else >>
#### **--subuidname**=*name*
<< endif >>

Run the container in a new user namespace using the map with _name_ in the _/etc/subuid_ file.
When running rootless, the user needs the necessary permissions to use the mapping. See **subuid**(5).
This flag conflicts with << '**UserNS=**' if is_quadlet else '**--userns**' >> and << '**UIDMap=**' if is_quadlet else '**--uidmap**' >>.
Together with << '**UIDMap=**' if is_quadlet else '**--uidmap**' >> it acts as if << '**UIDMap=**' if is_quadlet else '**--uidmap**' >> was passed with all mappings of the _name_ user in _/etc/subuid_.
