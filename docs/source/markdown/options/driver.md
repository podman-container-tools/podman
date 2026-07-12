####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Driver=driver`
<< else >>
#### **--driver**, **-d**=*driver*
<< endif >>

Driver to manage the network. Currently `bridge`, `macvlan` and `ipvlan` are supported. Defaults to `bridge`.
When running rootless, the `macvlan` and `ipvlan` drivers have no access to the host network interfaces because rootless networking requires a separate network namespace.

The netavark backend allows the use of so called *netavark plugins*, see the
[plugin-API.md](https://github.com/containers/netavark/blob/main/plugin-API.md)
documentation in netavark. The binary must be placed in a specified directory
so Podman can discover it, this list is set in `netavark_plugin_dirs` in
**[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**
under the `[network]` section.

The name of the plugin can then be used as driver to create a network for your plugin.
The list of all supported drivers and plugins can be seen with `podman info --format {{.Plugins.Network}}`.

Note that the `macvlan` and `ipvlan` drivers do not support port forwarding. Support for port forwarding
with a plugin depends on the implementation of the plugin.
