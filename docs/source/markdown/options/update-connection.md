####> This option file is used in:
####>   podman machine init, machine start
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--update-connection**, **-u**

When used in conjunction with `podman machine init --now` or `podman machine start`, this option sets the
associated machine system connection as the default. When using this option, a `-u` or `-update-connection` will
set the value to true.  To set this value to false, meaning no change and no prompting,
use `--update-connection=false`. If the value is unset and stdin is not a tty, no prompt or update
shall occur.

If the value is set to true, the machine connection will be set as the system default.
If the value is set to false, the system default will be unchanged.
If the option is not set, the user will be prompted and asked if it should be changed.
