####> This option file is used in:
####>   podman exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--preserve-fd**=*FD1[,FD2,...]*

Pass down to the process the additional file descriptors specified in the comma separated list.  It can be specified multiple times.
This option is only supported with the crun OCI runtime.  It might be a security risk to use this option with other OCI runtimes.

(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
