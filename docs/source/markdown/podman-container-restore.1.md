% podman-container-restore 1

## NAME
podman\-container\-restore - Restore one or more containers from a checkpoint

## SYNOPSIS
**podman container restore** [*options*] *name* [...]

## DESCRIPTION
**podman container restore** restores a container from a container checkpoint or
checkpoint image. The *container IDs*, *image IDs* or *names* are used as input.

## OPTIONS
#### **--all**, **-a**

Restore all checkpointed *containers*.\
The default is **false**.\
*IMPORTANT: This OPTION does not need a container name or ID as input argument.*

#### **--file-locks**

Restore a *container* with file locks. This option is required to
restore file locks from a checkpoint image. If the checkpoint image
does not contain file locks, this option is ignored. Defaults to not
restoring file locks.\
The default is **false**.

#### **--ignore-rootfs**

If a *container* is restored from a checkpoint tar.gz file it is possible that it also contains all root file-system changes. With **--ignore-rootfs** it is possible to explicitly disable applying these root file-system changes to the restored *container*.\
The default is **false**.\
*IMPORTANT: This OPTION is only available in combination with __--import, -i__.*

#### **--ignore-static-ip**

If the *container* was started with **--ip** the restored *container* also tries to use that
IP address and restore fails if that IP address is already in use. This can happen, if
a *container* is restored multiple times from an exported checkpoint with **--name, -n**.

Using **--ignore-static-ip** tells Podman to ignore the IP address if it was configured
with **--ip** during *container* creation.

The default is **false**.

#### **--ignore-static-mac**

If the *container* was started with **--mac-address** the restored *container* also
tries to use that MAC address and restore fails if that MAC address is already
in use. This can happen, if a *container* is restored multiple times from an
exported checkpoint with **--name, -n**.

Using **--ignore-static-mac** tells Podman to ignore the MAC address if it was
configured with **--mac-address** during *container* creation.

The default is **false**.

#### **--ignore-volumes**

This option must be used in combination with the **--import, -i** option.
When restoring *containers* from a checkpoint tar.gz file with this option,
the content of associated volumes are not restored.\
The default is **false**.

#### **--import**, **-i**=*file*

Import a checkpoint tar.gz file, which was exported by Podman. This can be used
to import a checkpointed *container* from another host.\
*IMPORTANT: This OPTION does not need a container name or ID as input argument.*

During the import of a checkpoint file Podman selects the same container runtime
which was used during checkpointing. This is especially important if a specific
(non-default) container runtime was specified during container creation. Podman
also aborts the restore if the container runtime specified during restore does
not much the container runtime used for container creation.

#### **--import-previous**=*file*

Import a pre-checkpoint tar.gz file which was exported by Podman. This option
must be used with **-i** or **--import**. It only works on `runc 1.0-rc3` or `higher`.
*IMPORTANT: This OPTION is not supported on the remote client, including Mac and Windows (excluding WSL2) machines.*

#### **--keep**, **-k**

Keep all temporary log and statistics files created by `CRIU` during
checkpointing as well as restoring. These files are not deleted if restoring
fails for further debugging. If restoring succeeds these files are
theoretically not needed, but if these files are needed Podman can keep the
files for further analysis. This includes the checkpoint directory with all
files created during checkpointing. The size required by the checkpoint
directory is roughly the same as the amount of memory required by the
processes in the checkpointed *container*.\
Without the **--keep**, **-k** option, the checkpoint is consumed and cannot be used again.\
The default is **false**.

#### **--latest**, **-l**

Instead of providing the *container ID* or *name*, use the last created *container*. The default is **false**.
*IMPORTANT: This OPTION is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines. This OPTION does not need a container name or ID as input argument.*

#### **--name**, **-n**=*name*

If a *container* is restored from a checkpoint tar.gz file it is possible to rename it with **--name, -n**. This way it is possible to restore a *container* from a checkpoint multiple times with different
names.

If the **--name, -n** option is used, Podman does not attempt to assign the same IP
address to the *container* it was using before checkpointing as each IP address can only
be used once, and the restored *container* has another IP address. This also means
that **--name, -n** cannot be used in combination with **--tcp-established**.\
*IMPORTANT: This OPTION is only available for a checkpoint image or in combination
with __--import, -i__.*

#### **--pod**=*name*

Restore a container into the pod *name*. The destination pod for this restore
has to have the same namespaces shared as the pod this container was checkpointed
from (see **[podman pod create --share](podman-pod-create.1.md#--share)**).\
*IMPORTANT: This OPTION is only available for a checkpoint image or in combination
with __--import, -i__.*

This option requires at least CRIU 3.16.

#### **--print-stats**

Print out statistics about restoring the container(s). The output is
rendered in a JSON array and contains information about how much time different
restore operations required. Many of the restore statistics are created
by CRIU and just passed through to Podman. The following information is provided
in the JSON array:

- **podman_restore_duration**: Overall time (in microseconds) needed to restore
  all checkpoints.

- **runtime_restore_duration**: Time (in microseconds) the container runtime
  needed to restore the checkpoint.

- **forking_time**: Time (in microseconds) CRIU needed to create (fork) all
  processes in the restored container (measured by CRIU).

- **restore_time**: Time (in microseconds) CRIU needed to restore all processes
  in the container (measured by CRIU).

- **pages_restored**: Number of memory pages restored (measured by CRIU).

The default is **false**.

#### **--publish**, **-p**=*port*

Replaces the ports that the *container* publishes, as configured during the
initial *container* start, with a new set of port forwarding rules.

For more details, see **[podman run --publish](podman-run.1.md#--publish)**.\
*IMPORTANT: This OPTION is only available for a checkpoint image or in combination
with __--import, -i__.*

#### **--tcp-close**

Restore a *container* and close all TCP connections. This option is useful
when TCP connections are not needed after restore or when connections
will be reestablished by the application. If the checkpoint image was created with
**--tcp-close**, this option should be used during restore.\
The default is **false**.

#### **--tcp-established**

Restore a *container* with established TCP connections. If the checkpoint image
contains established TCP connections, this option is required during restore.
If the checkpoint image does not contain established TCP connections this
option is ignored. Defaults to not restoring *containers* with established TCP
connections.\
The default is **false**.

## EXAMPLES
Restore the container "mywebserver".
```
# podman container restore mywebserver
```

Import a checkpoint file and a pre-checkpoint file.
```
# podman container restore --import-previous pre-checkpoint.tar.gz --import checkpoint.tar.gz
```

Start the container "mywebserver". Make a checkpoint of the container and export it. Restore the container with other port ranges from the exported file.
```
$ podman run --rm -p 2345:80 -d webserver
# podman container checkpoint -l --export=dump.tar
# podman container restore -p 5432:8080 --import=dump.tar
```

Start a container with the name "foobar-1". Create a checkpoint image "foobar-checkpoint". Restore the container from the checkpoint image with a different name.
```
# podman run --name foobar-1 -d webserver
# podman container checkpoint --create-image foobar-checkpoint foobar-1
# podman inspect foobar-checkpoint
# podman container restore --name foobar-2 foobar-checkpoint
# podman container restore --name foobar-3 foobar-checkpoint
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-container-checkpoint(1)](podman-container-checkpoint.1.md)**, **[podman-run(1)](podman-run.1.md)**, **[podman-pod-create(1)](podman-pod-create.1.md)**, **criu(8)**

## HISTORY
September 2018, Originally compiled by Adrian Reber <areber@redhat.com>
