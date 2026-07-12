####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--ipc**=*ipc*

Set the IPC namespace mode for a container. The default is to create
a private IPC namespace.

- "": Use Podman's default, defined in containers.conf.
- **container:**_id_: reuses another container's shared memory, semaphores, and message queues
- **host**: use the host's shared memory, semaphores, and message queues inside the container. Note: the host mode gives the container full access to local shared memory and is therefore considered insecure.
- **none**:  private IPC namespace, with /dev/shm not mounted.
- **ns:**_path_: path to an IPC namespace to join.
- **private**: private IPC namespace.
- **shareable**: private IPC namespace with a possibility to share it with other containers.
