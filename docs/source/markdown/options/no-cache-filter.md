####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-cache-filter**=*stage*

Do not use existing cached images for the named or numbered build *stage*, without requiring **--no-cache** to also be set.
Stages not named by **--no-cache-filter** may still use the cache. Only has an effect when **--layers** is used, since
caching is not performed on a per-stage basis otherwise. May be specified multiple times.

Notes: **--no-cache** disables the cache for all stages, so **--no-cache-filter** has no additional effect when both are set.
If a stage's name is identical to another stage's numeric position (for example, a stage literally named "1"),
a value passed to **--no-cache-filter** may match both.
