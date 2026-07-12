####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--sdnotify**=**container** | *conmon* | *healthy* | *ignore*

Determines how to use the NOTIFY_SOCKET, as passed with systemd and Type=notify.

Default is **container**, which means allow the OCI runtime to proxy the socket into the
container to receive ready notification. Podman sets the MAINPID to conmon's pid.
The **conmon** option sets MAINPID to conmon's pid, and sends READY when the container
has started. The socket is never passed to the runtime or the container.
The **healthy** option sets MAINPID to conmon's pid, and sends READY when the container
has turned healthy; requires a healthcheck to be set. The socket is never passed to the runtime or the container.
The **ignore** option removes NOTIFY_SOCKET from the environment for itself and child processes,
for the case where some other process above Podman uses NOTIFY_SOCKET and Podman does not use it.
