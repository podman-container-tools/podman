####> This option file is used in:
####>   podman artifact pull, artifact push, build, podman-build.unit.5.md.in, podman-container.unit.5.md.in, create, farm build, podman-image.unit.5.md.in, manifest push, pull, push, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `RetryDelay=duration`
<< else >>
#### **--retry-delay**=*duration*
<< endif >>

Duration of delay between retry attempts when pulling or pushing images between
the registry and local storage in case of failure. The default is to start at two seconds and then exponentially back off. The delay is used when this value is set, and no exponential back off occurs.
