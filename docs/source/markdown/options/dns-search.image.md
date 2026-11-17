####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DNSSearch=domain`
<< else >>
#### **--dns-search**=*domain*
<< endif >>

Set custom DNS search domains to be used during the build.
