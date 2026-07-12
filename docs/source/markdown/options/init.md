####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `RunInit=`
<< else >>
#### **--init**
<< endif >>

Run an init inside the container that forwards signals and reaps processes.
The container-init binary is mounted at `/run/podman-init`.
Mounting over `/run` breaks container execution.
