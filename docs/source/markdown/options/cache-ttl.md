####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cache-ttl**

Limit the use of cached images to only consider images with created timestamps less than *duration* ago.
For example if `--cache-ttl=1h` is specified, Buildah considers intermediate cache images which are created
under the duration of one hour, and intermediate cache images outside this duration is ignored.

Note: Setting `--cache-ttl=0` manually is equivalent to using `--no-cache` in the
implementation since this means that the user does not want to use cache at all.
