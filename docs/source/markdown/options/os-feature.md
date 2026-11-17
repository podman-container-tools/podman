####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--os-feature**=*feature*

Set the name of a required operating system *feature* for the image which is built.  By default, if the image is not based on *scratch*, the base image's required OS feature list is kept, if the base image specified any.  This option is typically only meaningful when the image's OS is Windows.

If *feature* has a trailing `-`, then the *feature* is removed from the set of required features which is listed in the image.
