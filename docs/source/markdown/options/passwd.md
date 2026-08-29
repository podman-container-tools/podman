####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--passwd**

Allow Podman to add entries to /etc/passwd and /etc/group when used in conjunction with the --user option.
This is used to override the Podman provided user setup in favor of entrypoint configurations such as libnss-extrausers.
