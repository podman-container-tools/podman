####> This option file is used in:
####>   podman create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--tty**, **-t**

Allocate a pseudo-TTY. The default is **false**.

When set to **true**, Podman allocates a pseudo-tty and attach to the standard
input of the container. This can be used, for example, to run a throwaway
interactive shell.

**NOTE**: The --tty flag prevents redirection of standard output.  It combines STDOUT and STDERR, it can insert control characters, and it can hang pipes. This option is only used when run interactively in a terminal. When feeding input to Podman, use -i only, not -it.
