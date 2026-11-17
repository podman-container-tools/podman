####> This option file is used in:
####>   podman kube play, podman-kube.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ConfigMap=path`
<< else >>
#### **--configmap**=*path*
<< endif >>

Use the Kubernetes ConfigMap YAML as the *path* to provide environment variable values within the containers of the pod.  (This option is not available with the remote Podman client)

<< if is_quadlet >>
The value may contain only one path but it may be absolute or relative to the location of the unit file.
<< else >>
Note: The **--configmap** option can be used multiple times, or a comma-separated list of paths can be used to pass multiple Kubernetes ConfigMap YAMLs.
The YAML file may be in a multi-doc YAML format. But, it must contain only ConfigMaps.
<< endif >>
