####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `EnvironmentFile=file`
<< else >>
#### **--env-file**=*file*
<< endif >>

Read in a line-delimited file of environment variables.
