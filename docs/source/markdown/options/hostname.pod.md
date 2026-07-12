####> This option file is used in:
####>   podman pod clone, pod create
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--hostname**=*name*

Set the pod's hostname inside all containers.

The given hostname is also added to the `/etc/hosts` file using the container's
primary IP address (also see the **--add-host** option).
