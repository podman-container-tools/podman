####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `StopTimeout=seconds`
<< else >>
#### **--stop-timeout**=*seconds*
<< endif >>

Timeout to stop a container. Default is **10**.
Remote connections use local containers.conf for defaults.

<< if is_quadlet >>
**Note:** this value should be lower than the actual systemd unit timeout to ensure the `podman rm` command is not killed by systemd.
<< endif >>
