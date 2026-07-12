####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `HostName=name`
<< else >>
#### **--hostname**, **-h**=*name*
<< endif >>

Set the container's hostname inside the container.

This option can only be used with a private UTS namespace `--uts=private`
(default). If << '`Pod=`' if is_quadlet else '`--pod`' >> is given and the pod shares the same UTS namespace
(default), the pod's hostname is used. The given hostname is also added to the
`/etc/hosts` file using the container's primary IP address (also see the
<< '**AddHost=**' if is_quadlet else '**--add-host**' >> option).

When << '**HostName=** is unset' if is_quadlet else '**--hostname** is not used' >> and the container uses a private UTS namespace (default), Podman sets the hostname to the first 12 characters of the container ID. The container name assigned with << '**ContainerName=**' if is_quadlet else '**--name**' >> is not used unless *container_name_as_hostname=true* is set in `containers.conf`.

Podman network DNS registers the container name, the short container ID (first 12 characters), and any explicitly set **--hostname** as DNS names. The default hostname matches the short ID alias. See **[podman-network(1)](podman-network.1.md)**.
