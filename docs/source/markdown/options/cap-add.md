####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `AddCapability=capability`
<< else >>
#### **--cap-add**=*capability*
<< endif >>

Add Linux capabilities.

Granting additional capabilities increases the privileges of the
processes running inside the container and potentially allow it to
break out of confinement.  Capabilities like `CAP_SYS_ADMIN`,
`CAP_SYS_PTRACE`, `CAP_MKNOD` and `CAP_SYS_MODULE` are particularly
dangerous when they are not used within a user namespace.  Please
refer to **user_namespaces(7)** for a more detailed explanation of the
interaction between user namespaces and capabilities.

Before adding any capability, review its security implications and
ensure it is really necessary for the container’s functionality.  See
**capabilities(7)** for more information.
