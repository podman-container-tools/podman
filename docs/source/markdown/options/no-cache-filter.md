####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-cache-filter**=*stagename*

Do not use existing cached images for the specified stages of a multi-stage
Dockerfile. Build those stages from scratch while still using cache for other
stages. To specify multiple stages, use a comma-separated list or pass the flag
multiple times.

```
$ podman build --no-cache-filter install .
$ podman build --no-cache-filter install,test .
```

This option cannot be combined with **--no-cache**.
