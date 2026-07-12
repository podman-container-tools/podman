####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthRetries=retries`
<< else >>
#### **--health-retries**=*retries*
<< endif >>

The number of retries allowed before a healthcheck is considered to be unhealthy. The default value is **3**.

Note: This parameter can overwrite the healthcheck configuration from the image.
