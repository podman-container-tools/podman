####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--created-annotation**

Add an image *annotation* (see also **--annotation**) to the image metadata
setting "org.opencontainers.image.created" to the current time, or to the
datestamp specified to the **--source-date-epoch** or **--timestamp** flag,
if either was used.  If *false*, no such annotation will be present in the
written image.

Note: this information is not present in Docker image formats, so it is discarded when writing images in Docker formats.
