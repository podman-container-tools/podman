####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `WorkingDir=dir`
<< else >>
#### **--workdir**, **-w**=*dir*
<< endif >>

Working directory inside the container.

The default working directory for running binaries within a container is the root directory (**/**).
The image developer can set a different default with the WORKDIR instruction. The operator
can override the working directory by using the << '**WorkingDir=**' if is_quadlet else '**-w**' >> option.
