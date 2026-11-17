####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DNSOption=option`
<< else >>
#### **--dns-option**=*option*
<< endif >>

Set custom DNS options. Invalid if using << '**DNSOption=**' if is_quadlet else '**--dns-option**' >>
with << '**Network=**' if is_quadlet else '**--network**' >> that is set to **none** or **container:**_id_.
