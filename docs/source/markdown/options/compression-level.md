####> This option file is used in:
####>   podman build, farm build, manifest push, push
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--compression-level**=*level*

Specifies the compression level to use.  The value is specific to the compression algorithm used, e.g. for zstd the accepted values are in the range 1-20 (inclusive) with a default of 3, while for gzip it is 1-9 (inclusive) and has a default of 5.
