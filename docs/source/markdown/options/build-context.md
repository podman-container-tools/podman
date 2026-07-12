####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--build-context**=*name=value*

Specify an additional build context using its short name and its location.
Additional build contexts can be referenced in the same manner as we access
different stages in COPY instruction.

Valid values are:

* Local directory – e.g. --build-context project2=../path/to/project2/src
* HTTP URL to a tarball – e.g. --build-context src=https://example.org/releases/src.tar
* Container image – specified with a container-image:// prefix, e.g. --build-context alpine=container-image://alpine:3.15, (also accepts docker://, docker-image://)

On the Containerfile side, reference the build context on all
commands that accept the “from” parameter. Here’s how that might look:

```dockerfile
FROM [name]
COPY --from=[name] ...
RUN --mount=from=[name] …
```

The value of [name] is matched with the following priority order:

* Named build context defined with --build-context [name]=..
* Stage defined with AS [name] inside Containerfile
* Image [name], either local or in a remote registry
