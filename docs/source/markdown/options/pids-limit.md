####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `PidsLimit=limit`
<< else >>
#### **--pids-limit**=*limit*
<< endif >>

Tune the container's pids limit. Set to **-1** to have unlimited pids for the container. The default is **2048** on systems that support "pids" cgroup controller.
