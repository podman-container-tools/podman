####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthStartupInterval=interval`
<< else >>
#### **--health-startup-interval**=*interval*
<< endif >>

Set an interval for the startup healthcheck. An _interval_ of **disable** results in no automatic timer setup. The default is **30s**.
