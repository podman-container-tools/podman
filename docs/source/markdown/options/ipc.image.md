####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--ipc**=*how*

Sets the configuration for IPC namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate
that a new IPC namespace is created, or it can be "host" to indicate
that the IPC namespace in which `podman` itself is being run is reused,
or it can be the path to an IPC namespace which is already in use by
another process.
