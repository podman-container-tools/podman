####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--chrootdirs**=*path*

Path to a directory inside the container that is treated as a `chroot` directory.
Any Podman managed file (e.g., /etc/resolv.conf, /etc/hosts, /etc/hostname) that is mounted into the root directory is mounted into that location as well.
Multiple directories are separated with a comma.
