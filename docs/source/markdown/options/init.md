####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `RunInit=`
<< else >>
#### **--init**
<< endif >>

Run an init inside the container that forwards signals and reaps processes.
The container-init binary is mounted at `/run/podman-init`.
Mounting over `/run` breaks container execution.

The default is the `init` setting in the `[containers]` section of
**containers.conf**, which defaults to **false**.
<< if is_quadlet >>
Set **RunInit=true** or **RunInit=false** to override this default.
<< else >>
For remote clients, the server's **containers.conf** determines the default.
Use **--init=true** or **--init=false** to override this default.
<< endif >>
