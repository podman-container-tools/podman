####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pidfile**=*path*

When the pidfile location is specified, the container process' PID is written to the pidfile. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
If the pidfile option is not specified, the container process' PID is written to /run/containers/storage/${storage-driver}-containers/$CID/userdata/pidfile.

After the container is started, the location for the pidfile can be discovered with the following `podman inspect` command:

    $ podman inspect --format '{{ .PidFile }}' $CID
    /run/containers/storage/${storage-driver}-containers/$CID/userdata/pidfile
