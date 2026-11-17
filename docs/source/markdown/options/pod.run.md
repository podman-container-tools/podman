####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pod**=*name*

Run container in an existing pod. Podman makes the pod automatically if the pod name is prefixed with **new:**.
To make a pod with more granular options, use the **podman pod create** command before creating a container.
When a container is run with a pod with an infra-container, the infra-container is started first.
