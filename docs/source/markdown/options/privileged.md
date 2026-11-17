####> This option file is used in:
####>   podman create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--privileged**

Give extended privileges to this container. The default is **false**.

By default, Podman containers are unprivileged (**=false**) and cannot, for
example, modify parts of the operating system. This is because by default a
container is only allowed limited access to devices. A "privileged" container
is given the same access to devices as the user launching the container, with
the exception of virtual consoles (_/dev/tty\d+_) when running in systemd
mode (**--systemd=always**).

A privileged container turns off the security features that isolate the
container from the host. Dropped Capabilities, limited devices, read-only mount
points, Apparmor/SELinux separation, and Seccomp filters are all disabled.
Due to the disabled security features, the privileged field should almost never
be set as containers can easily break out of confinement.

Containers running in a user namespace (e.g., rootless containers) cannot have
more privileges than the user that launched them.
