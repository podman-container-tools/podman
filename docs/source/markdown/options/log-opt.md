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
This option is currently supported by the **journald** and **syslog** log drivers.
For **syslog** it is the message identity (the syslog tag).

**syslog-address**: where the **syslog** driver sends messages.
Omit it to use the local syslog service. Conmon checks the value when the container starts and rejects an empty one.
Supported forms are `tcp://host[:port]`, `udp://host[:port]`, `unix:///path` and `unixgram:///path`.
The default port is 514. IPv6 hosts must be in brackets (`tcp://[::1]:1514`).
TCP and UDP use RFC 5424 (TCP also uses the RFC 6587 octet-count prefix). Unix sockets use the traditional local syslog layout.
    (e.g. << '**LogOpt=syslog-address=tcp://localhost:1514**' if is_quadlet else '**--log-opt syslog-address=tcp://localhost:1514**' >>);

**syslog-facility**: syslog facility name or number for the **syslog** driver.
Conmon checks the value when the container starts.
Names are `kern`, `user`, `mail`, `daemon`, `auth`, `syslog`, `lpr`, `news`, `uucp` and `local0` through `local7`.
Numbers are the matching facility codes (`0` through `8` and `16` through `23`).
The default is `user` for local syslog and `daemon` for a **syslog-address**.
    (e.g. << '**LogOpt=syslog-facility=daemon**' if is_quadlet else '**--log-opt syslog-facility=daemon**' >>);

**label**: specify a custom log label for the container
    (e.g. **--log-opt label="CONTAINER_IMAGE={{.ImageName}}"**.
It supports the same keys as **podman inspect --format**.
This option can be repeated multiple times.
This option is currently supported only by the **journald** log driver.
