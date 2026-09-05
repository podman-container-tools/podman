####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Ulimit=option`
<< else >>
#### **--ulimit**=*option*
<< endif >>

Ulimit options. Sets the ulimits values inside of the container.

--ulimit with a soft and hard limit in the format <type>=<soft limit>[:<hard limit>]. For example:

$ podman run --ulimit nofile=1024:1024 --rm ubi9 ulimit -n
1024

Limits are passed to the container unchanged, in the units used by **setrlimit(2)**.
Size limits are given in bytes, not in the units that the **ulimit** shell builtin
displays. The shell scales several of those values when printing them, so the value
reported inside the container may differ from the value given on the command line:

| Type                                           | Unit of the value given to --ulimit | Unit used by ulimit -a |
|:-----------------------------------------------|:------------------------------------|:-----------------------|
| core, fsize                                    | bytes                                   | 512-byte blocks          |
| data, memlock, rss, stack                      | bytes                                   | kbytes                   |
| msgqueue                                       | bytes                                   | bytes                    |
| rttime                                         | microseconds                            | microseconds             |
| cpu                                            | seconds                                 | seconds                  |
| locks, nice, nofile, nproc, rtprio, sigpending | count                                   | count                    |

For example, a locked memory limit of 4096 kbytes is set in bytes:

$ podman run --ulimit memlock=4194304 --rm ubi9 ulimit -l
4096

Unit suffixes such as *k* or *m* are not accepted.

Set -1 for the soft or hard limit to set the limit to the maximum limit of the current
process. In rootful mode this is often unlimited.


If nofile and nproc are unset, a default value of 1048576 will be used, unless overridden
in containers.conf(5).  However, if the default value exceeds the hard limit for the current
rootless user, the current hard limit will be applied instead.

Use **host** to copy the current configuration from the host.

Don't use nproc with the ulimit flag as Linux uses nproc to set the
maximum number of processes available to a user, not to a container.

Use the --pids-limit option to modify the cgroup control to limit the number
of processes within a container.
