####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--build-arg-file**=*path*

Specifies a file containing lines of build arguments of the form `arg=value`.
The suggested file name is `argfile.conf`.

Comment lines beginning with `#` are ignored, along with blank lines.
All others must be of the `arg=value` format passed to `--build-arg`.

If several arguments are provided via the `--build-arg-file`
and `--build-arg` options, the build arguments are merged across all
of the provided files and command line arguments.

Any file provided in a `--build-arg-file` option is read before
the arguments supplied via the `--build-arg` option.

When a given argument name is specified several times, the last instance
is the one that is passed to the resulting builds. This means `--build-arg`
values always override those in a `--build-arg-file`.
