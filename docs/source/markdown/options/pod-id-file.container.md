####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pod-id-file**=*file*

Run container in an existing pod and read the pod's ID from the specified *file*.
When a container is run within a pod which has an infra-container, the infra-container starts first.
