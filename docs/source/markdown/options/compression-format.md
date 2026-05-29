####> This option file is used in:
####>   podman build, farm build, manifest push, push
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--compression-format**=**gzip** | *zstd* | *zstd:chunked*

Specifies the compression format to use.  Supported values are: `gzip`, `zstd` and `zstd:chunked`.  The default is `gzip` unless overridden in the containers.conf file.
`zstd:chunked` is incompatible with encrypting images, and will be treated as `zstd` with a warning in that case.
