####> This option file is used in:
####>   podman pod clone, pod create
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pid**=*pid*

Set the PID mode for the pod. The default is to create a private PID namespace for the pod. Requires the PID namespace to be shared via --share.

    host: use the host’s PID namespace for the pod
    ns: join the specified PID namespace
    private: create a new namespace for the pod (default)
