####> This option file is used in:
####>   podman podman-container.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
### `Exec=command`

Additional arguments for the container; this has exactly the same effect as passing
more arguments after a `podman run <image> <arguments>` invocation.

The format is the same as for [systemd command lines](https://www.freedesktop.org/software/systemd/man/systemd.service.html#Command%20lines),
However, unlike the similarly named systemd `ExecStart=` directive
which operates on the ambient root filesystem, it is very common for container
images to have their own `ENTRYPOINT` or `CMD` metadata which this interacts with.

The default expectation for many images is that the image will include an `ENTRYPOINT`
with a default binary, and this field will add arguments to that entrypoint.

Another way to describe this is that it works the same way as the [args field in a Kubernetes pod](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell).
