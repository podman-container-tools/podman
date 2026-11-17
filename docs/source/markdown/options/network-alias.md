####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, pod create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `NetworkAlias=alias`
<< else >>
#### **--network-alias**=*alias*
<< endif >>

Add a network-scoped alias for the <<container|pod>>, setting the alias for all networks that the container joins. To set a
name only for a specific network, use the alias option as described under the **--network** option.
If the network has DNS enabled (`podman network inspect -f {{.DNSEnabled}} <name>`),
these aliases can be used for name resolution on the given network. This option can be specified multiple times.
