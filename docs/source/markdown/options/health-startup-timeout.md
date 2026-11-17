####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthStartupTimeout=timeout`
<< else >>
#### **--health-startup-timeout**=*timeout*
<< endif >>

The maximum time a startup healthcheck command has to complete before it is marked as failed. The value can be expressed in a time
format like **2m3s**. The default value is **30s**.
