####> This option file is used in:
####>   podman build, farm build, manifest push, push
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--force-compression**

If set, push uses the specified compression algorithm even if the destination contains a differently-compressed variant already.
Defaults to `true` if `--compression-format` is explicitly specified on the command-line, `false` otherwise.
