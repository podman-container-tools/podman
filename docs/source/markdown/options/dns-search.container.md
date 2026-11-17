####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DNSSearch=domain`
<< else >>
#### **--dns-search**=*domain*
<< endif >>

Set custom DNS search domains. Invalid if using << '**DNSSearch=**' if is_quadlet else '**--dns-search**' >>
with << '**Network=**' if is_quadlet else '**--network**' >> that is set to **none** or **container:**_id_.
Use << '**DNSSearch=.**' if is_quadlet else '**--dns-search=.**' >> to remove the search domain.
