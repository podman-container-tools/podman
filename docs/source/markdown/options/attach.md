####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--attach**, **-a**=*stdin* | *stdout* | *stderr*

Attach to STDIN, STDOUT or STDERR.

In foreground mode (the default when **-d**
is not specified), **podman run** can start the process in the container
and attach the console to the process's standard input, output, and
error. It can even pretend to be a TTY (this is what most command-line
executables expect) and pass along signals. The **-a** option can be set for
each of **stdin**, **stdout**, and **stderr**.
