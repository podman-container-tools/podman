####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthStartPeriod=period`
<< else >>
#### **--health-start-period**=*period*
<< endif >>

The initialization time needed for a container to bootstrap. The value can be expressed in time format like
**2m3s**. The default value is **0s**.

Note: The health check command is executed as soon as a container is started, if the health check is successful
the container's health state will be updated to `healthy`. However, if the health check fails, the health state will
stay as `starting` until either the health check is successful or until
the << '`HealthStartPeriod=`' if is_quadlet else '`--health-start-period`' >> time is over. If the
health check command fails after the << '`HealthStartPeriod=`' if is_quadlet else '`--health-start-period`' >>
time is over, the health state will be updated to `unhealthy`.
The health check command is executed periodically based on the value of
<< '`HealthInterval=`' if is_quadlet else '`--health-interval`' >>.

Note: This parameter will overwrite related healthcheck configuration from the image.
