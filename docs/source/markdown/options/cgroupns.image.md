####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cgroupns**=*how*

Sets the configuration for cgroup namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "private" to indicate
that a new cgroup namespace is created, or it can be "host" to indicate
that the cgroup namespace in which `buildah` itself is being run is reused.
