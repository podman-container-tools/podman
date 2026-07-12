####> This option file is used in:
####>   podman create, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--hosts-file**=*path* | *none* | *image*

Base file to create the `/etc/hosts` file inside the container. This must either
be an absolute path to a file on the host system, or one of the following
special flags:
  ""      Follow the `base_hosts_file` configuration in _containers.conf_ (the default)
  `none`  Do not use a base file (i.e. start with an empty file)
  `image` Use the container image's `/etc/hosts` file as base file
