####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pid**=*mode*

Set the PID namespace mode for the container.
The default is to create a private PID namespace for the container.

- **container:**_id_: join another container's PID namespace;
- **host**: use the host's PID namespace for the container. Note the host mode gives the container full access to local PID and is therefore considered insecure;
- **ns:**_path_: join the specified PID namespace;
- **private**: create a new namespace for the container (default).
