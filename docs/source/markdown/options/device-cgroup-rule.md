####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--device-cgroup-rule**=*"type major:minor mode"*

Add a rule to the cgroup allowed devices list. The rule is expected to be
in the format specified in the Linux kernel documentation
[admin-guide/cgroup-v1/devices](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/devices.html):
- *type*: `a` (all), `c` (char), or `b` (block);
- *major* and *minor*: either a number, or `*` for all;
- *mode*: a composition of `r` (read), `w` (write), and `m` (mknod(2)).
