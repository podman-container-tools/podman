####> This option file is used in:
####>   podman attach, container diff, container inspect, diff, exec, init, inspect, kill, logs, mount, network reload, pause, pod inspect, pod kill, pod logs, pod rm, pod start, pod stats, pod stop, pod top, port, restart, rm, start, stats, stop, top, unmount, unpause, update, wait
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--latest**, **-l**

Instead of providing the <<container|pod>> name or ID, use the last created <<container|pod>>.
Note: the last started <<container|pod>> can be from other users of Podman on the host machine.
(This option is not available with the remote Podman client, including Mac and Windows
(excluding WSL2) machines)
