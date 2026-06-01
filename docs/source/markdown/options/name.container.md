####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ContainerName=name`
<< else >>
#### **--name**=*name*
<< endif >>

Assign a name to the container.

The operator can identify a container in three ways:

- UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”);
- UUID short identifier (“f78375b1c487”);
- Name (“jonah”).

Podman generates a UUID for each container, and if no name is assigned to the
container using << '**ContainerName=**' if is_quadlet else '**--name**' >>,
Podman generates a random string name such as `exciting_chebyshev` (`adjective_noun`,
compatible with Docker). Container names are not required to be valid hostnames or
domain names. Underscores and other characters allowed by naming rules are
permitted. On Podman networks with DNS enabled, container-to-container name
resolution still uses the name as given, for example `exciting_chebyshev`. The
name can be useful as a more human-friendly way to identify containers.
This works for both background and foreground containers.
The container's name is also added to the `/etc/hosts` file using the
container's primary IP address (also see the
<< '**AddHost=**' if is_quadlet else '**--add-host**' >> option).

The name is not the hostname inside the container; see
<< '**HostName=**' if is_quadlet else '**--hostname**' >>. See
**[podman-network(1)](podman-network.1.md)** for more on network DNS.
