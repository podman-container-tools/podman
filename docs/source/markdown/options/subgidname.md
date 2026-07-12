####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, pod clone, pod create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `SubGIDMap=name`
<< else >>
#### **--subgidname**=*name*
<< endif >>

Run the container in a new user namespace using the map with _name_ in the _/etc/subgid_ file.
When running rootless, the user needs the necessary permissions to use the mapping. See **subgid**(5).
This flag conflicts with << '**UserNS=**' if is_quadlet else '**--userns**' >> and << '**GIDMap=**' if is_quadlet else '**--gidmap**' >>.
Together with << '**GIDMap=**' if is_quadlet else '**--gidmap**' >> it acts as if << '**GIDMap=**' if is_quadlet else '**--gidmap**' >> was passed with all mappings of the _name_ group in _/etc/subgid_.
