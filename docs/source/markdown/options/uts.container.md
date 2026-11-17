####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--uts**=*mode*

Set the UTS namespace mode for the container. The following values are supported:

- **host**: use the host's UTS namespace inside the container.
- **private**: create a new namespace for the container (default).
- **ns:[path]**: run the container in the given existing UTS namespace.
- **container:[container]**: join the UTS namespace of the specified container.
