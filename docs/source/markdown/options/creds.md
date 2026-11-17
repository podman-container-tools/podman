####> This option file is used in:
####>   podman artifact pull, artifact push, build, container runlabel, create, farm build, podman-image.unit.5.md.in, kube play, manifest add, manifest push, pull, push, run, search
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Creds=[username[:password]]`
<< else >>
#### **--creds**=*[username[:password]]*
<< endif >>

The [username[:password]] to use to authenticate with the registry, if required.
If one or both values are not supplied, a command line prompt appears and the
value can be entered. The password is entered without echo.

Note that the specified credentials are only used to authenticate against
target registries.  They are not used for mirrors or when the registry gets
rewritten (see `containers-registries.conf(5)`); to authenticate against those
consider using a `containers-auth.json(5)` file.
