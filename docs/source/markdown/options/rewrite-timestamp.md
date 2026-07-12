####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--rewrite-timestamp**

When generating new layers for the image, ensure that no newly added content
bears a timestamp later than the value used by the **--source-date-epoch**
flag, if one was provided, by replacing any timestamps which are later than
that value, with that value.
