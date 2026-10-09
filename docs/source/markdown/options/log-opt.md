####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, kube play, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `LogOpt=name=value`
<< else >>
#### **--log-opt**=*name=value*
<< endif >>

Logging driver specific options.

Set custom logging configuration. The following *name*s are supported:

**path**: specify a path to the log file
    (e.g. << '**LogOpt=path=/var/log/container/mycontainer.json**' if is_quadlet else '**--log-opt path=/var/log/container/mycontainer.json**' >>);

**max-size**: specify a max size of the log file
    (e.g. << '**LogOpt=max-size=10mb**' if is_quadlet else '**--log-opt max-size=10mb**' >>);

**max-file**: specify the maximum number of rotated log files to keep when log
rotation is enabled (requires conmon >= 2.2.0)
    (e.g. << '**LogOpt=max-file=5**' if is_quadlet else '**--log-opt max-file=5**' >>).
Setting this option implicitly enables log rotation. If log rotation is enabled
without setting **max-file**, conmon keeps 1 backup file by default.

**log-rotate**: enable or disable log rotation for the container log file
(requires conmon >= 2.2.0). When enabled, the log file is rotated with a numbered
suffix instead of being truncated when it reaches the maximum size
    (e.g. << '**LogOpt=log-rotate=true**' if is_quadlet else '**--log-opt log-rotate=true**' >>).

**tag**: specify a custom log tag for the container
    (e.g. << '**LogOpt=tag="{{.ImageName}}"**' if is_quadlet else '**--log-opt tag="{{.ImageName}}"**' >>.
It supports the same keys as **podman inspect --format**.
This option is currently supported only by the **journald** log driver.

**label**: specify a custom log label for the container
    (e.g. **--log-opt label="CONTAINER_IMAGE={{.ImageName}}"**.
It supports the same keys as **podman inspect --format**.
This option can be repeated multiple times.
This option is currently supported only by the **journald** log driver.
