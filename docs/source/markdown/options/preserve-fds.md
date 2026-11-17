####> This option file is used in:
####>   podman exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--preserve-fds**=*N*

Pass down to the process N additional file descriptors (in addition to 0, 1, 2).
The total FDs are 3+N.
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
