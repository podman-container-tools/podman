####> This option file is used in:
####>   podman create, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--platform**=*OS/ARCH*

Specify the platform for selecting the image.  (Conflicts with --arch and --os)
The `--platform` option can be used to override the current architecture and operating system.
Unless overridden, subsequent lookups of the same image in the local storage matches this platform, regardless of the host.
