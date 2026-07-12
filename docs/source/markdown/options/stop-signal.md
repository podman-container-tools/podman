####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `StopSignal=signal`
<< else >>
#### **--stop-signal**=*signal*
<< endif >>

Signal to stop a container. Default is **SIGTERM**.
