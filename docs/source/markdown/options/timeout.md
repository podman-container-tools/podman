####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--timeout**=*seconds*

Maximum time a container is allowed to run before conmon sends it the kill
signal.  By default containers run until they exit or are stopped by
`podman stop`.
