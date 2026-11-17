####> This option file is used in:
####>   podman update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--env**, **-e**=*env*

Add a value (e.g. env=*value*) to the container. Can be used multiple times.
If the value already exists in the container, it is overridden.
To remove an environment variable from the container, use the `--unsetenv`
option.

Note that the env updates only affect the main container process after
the next start.
