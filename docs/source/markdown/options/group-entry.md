####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--group-entry**=*ENTRY*

Customize the entry that is written to the `/etc/group` file within the container when `--user` is used.

The variables $GROUPNAME, $GID, and $USERLIST are automatically replaced with their value at runtime if present.
