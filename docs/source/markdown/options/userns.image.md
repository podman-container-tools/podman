####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--userns**=*how*

Sets the configuration for user namespaces when handling `RUN` instructions.
The configured value can be "" (the empty string) or "container" to indicate that a new user namespace is created, it can be "host" to indicate that the user namespace in which `podman` itself is being run is reused, or it can be the path to a user namespace which is already in use by another process.
