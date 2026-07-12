####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HealthMaxLogCount=number`
<< else >>
#### **--health-max-log-count**=*number of stored logs*
<< endif >>

Set maximum number of attempts in the HealthCheck log file. ('0' value means an infinite number of attempts in the log file) (Default: 5 attempts)
