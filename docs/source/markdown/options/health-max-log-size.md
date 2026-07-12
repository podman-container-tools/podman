####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthMaxLogSize=size`
<< else >>
#### **--health-max-log-size**=*size of stored logs*
<< endif >>

Set maximum length in characters of stored HealthCheck log. ("0" value means an infinite log length) (Default: 500 characters)
