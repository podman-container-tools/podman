####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--layer-label**=*label[=value]*

Add an intermediate image *label* (e.g. label=*value*) to the intermediate
image metadata. It can be used multiple times.

If *label* is named, but neither `=` nor a `value` is provided, then
the *label* is set to an empty value.
