####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--inherit-annotations**=*bool-value*

Inherit the annotations from the base image or base stages. (default true).
Use cases which set this flag to *false* may need to do the same for the
**--created-annotation** flag.
