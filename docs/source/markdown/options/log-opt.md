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

**tag**: specify a custom log tag for the container
    (e.g. << '**LogOpt=tag="{{.ImageName}}"**' if is_quadlet else '**--log-opt tag="{{.ImageName}}"**' >>.
It supports the same keys as **podman inspect --format**.
This option is currently supported only by the **journald** log driver.

**label**: specify a custom log label for the container
    (e.g. **--log-opt label="CONTAINER_IMAGE={{.ImageName}}"**.
It supports the same keys as **podman inspect --format**.
This option can be repeated multiple times.
This option is currently supported only by the **journald** log driver.

**labels**: specify a comma-separated list of container label names whose values
should be included as structured journald fields
    (e.g. << '**LogOpt=labels=app_name,version**' if is_quadlet else '**--log-opt labels=app_name,version**' >>).
The label names are sanitized to valid journald field names by replacing
non-alphanumeric characters with underscores and converting to uppercase.
This option is currently supported only by the **journald** log driver.

**env**: specify a comma-separated list of environment variable names whose values
should be included as structured journald fields
    (e.g. << '**LogOpt=env=APP_ENV,APP_VERSION**' if is_quadlet else '**--log-opt env=APP_ENV,APP_VERSION**' >>).
The variable names are sanitized to valid journald field names by replacing
non-alphanumeric characters with underscores and converting to uppercase.
If a label and an environment variable produce the same field name, the
environment variable takes precedence.
This option is currently supported only by the **journald** log driver.
