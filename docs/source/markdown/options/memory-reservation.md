####> This option file is used in:
####>   podman container clone, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--memory-reservation**=*number[unit]*

Memory soft limit. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).

After setting memory reservation, when the system detects memory contention
or low memory, containers are forced to restrict their consumption to their
reservation. So always set the value below **--memory**, otherwise the
hard limit takes precedence. By default, memory reservation is the same
as memory limit.

This option is not supported on cgroups V1 rootless systems.
