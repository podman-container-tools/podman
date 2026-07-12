####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--os-version**=*version*

Set the exact required operating system *version* for the image which is built.  By default, if the image is not based on *scratch*, the base image's required OS version is kept, if the base image specified one.  This option is typically only meaningful when the image's OS is Windows, and is typically set in Windows base images, so using this option is usually unnecessary.
