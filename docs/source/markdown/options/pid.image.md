####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pid**=*pid*

Sets the configuration for PID namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate that a new PID namespace is created, or it can be "host" to indicate that the PID namespace in which `podman` itself is being run is reused, or it can be the path to a PID namespace which is already in use by another
process.
