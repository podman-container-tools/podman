####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthStartInterval=interval`
<< else >>
#### **--health-start-interval**=*interval*
<< endif >>

This is an alias for `--health-startup-interval` option made for **Docker** compatibility.
