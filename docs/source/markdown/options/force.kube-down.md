####> This option file is used in:
####>   podman kube down
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `KubeDownForce=true`
<< else >>
#### **--force**
<< endif >>

Remove all resources, including volumes, when calling `podman kube down`.
