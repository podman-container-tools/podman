####> This option file is used in:
####>   podman build, create, farm build, kube play, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-hosts**

Do not modify the `/etc/hosts` file in the <<container|pod>>.

Podman assumes control over the <<container|pod>>'s `/etc/hosts` file by
default and adds entries for the container's name (see **--name** option) and
hostname (see **--hostname** option), the internal `host.containers.internal`
and `host.docker.internal` hosts, as well as any hostname added using the
**--add-host** option. Refer to the **--add-host** option for details. Passing
**--no-hosts** disables this, so that the image's `/etc/hosts` file is kept
unmodified. The same can be achieved globally by setting *no_hosts=true* in
`containers.conf`.
