# Podman socket activation

Socket activation conceptually works by having systemd create a socket (e.g. TCP, UDP or Unix
socket). As soon as a client connects to the socket, systemd will start the systemd service that is
configured for the socket. The newly started program inherits the file descriptor of the socket
and can then accept the incoming connection (in other words run the system call `accept()`).
This description corresponds to the default systemd socket configuration
[`Accept=no`](https://www.freedesktop.org/software/systemd/man/systemd.socket.html#Accept=)
that lets the service accept the socket.

Podman supports two forms of socket activation:

* Socket activation of the API service
* Socket activation of containers

## Socket activation of the API service

The architecture looks like this

``` mermaid
stateDiagram-v2
    [*] --> systemd: first client connects
    systemd --> podman: socket inherited via fork/exec
```

The file _/usr/lib/systemd/user/podman.socket_ on a Fedora system defines the Podman API socket for
rootless users:

```
$ cat /usr/lib/systemd/user/podman.socket
[Unit]
Description=Podman API Socket
Documentation=man:podman-system-service(1)

[Socket]
ListenStream=%t/podman/podman.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
```

The socket is configured to be a Unix socket and can be started like this

```
$ systemctl --user start podman.socket
$ ls $XDG_RUNTIME_DIR/podman/podman.sock
/run/user/1000/podman/podman.sock
$
```
The socket can later be used by for instance __docker-compose__ that needs a Docker-compatible API

```
$ export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
$ docker-compose up
```

When __docker-compose__ or any other client connects to the UNIX socket `$XDG_RUNTIME_DIR/podman/podman.sock`,
the service _podman.service_ is started. See its definition in the file _/usr/lib/systemd/user/podman.service_.

## Socket activation of containers

Since version 3.4.0 Podman supports socket activation of containers, i.e.,  passing
a socket-activated socket to the container. Thanks to the fork/exec model of Podman, the socket will be first
inherited by conmon and then by the OCI runtime and finally by the container
as can be seen in the following diagram:


``` mermaid
stateDiagram-v2
    [*] --> systemd: first client connects
    systemd --> podman: socket inherited via fork/exec
    state "OCI runtime" as s2
    podman --> conmon: socket inherited via double fork/exec
    conmon --> s2: socket inherited via fork/exec
    s2 --> container: socket inherited via exec
```

This type of socket activation can be used in systemd services that are generated from container unit files (see [podman-systemd.unit(5)](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)) (Note, quadlet requires the use of cgroup v2) or from the command [`podman generate systemd`](https://docs.podman.io/en/latest/markdown/podman-generate-systemd.1.html).
The container must also support socket activation. Not all software daemons support socket activation
but it's getting more popular. For instance Apache HTTP server, MariaDB, DBUS, PipeWire, Gunicorn, CUPS
all have socket activation support.

### Example: socket-activated echo server container in a systemd service

This example shows how to run the socket-activated echo server
[socket-activate-echo](https://github.com/eriksjolund/socket-activate-echo/pkgs/container/socket-activate-echo)
in a systemd user service. Podman version 4.4.0 or higher is required.

Enable lingering for your regular user

```
$ loginctl enable-linger $USER
```

The command has these effects on your enabled systemd user units:

* the units are automatically started after a reboot
* the units are not automatically stopped after you log out

Create directories

```
$ mkdir -p ~/.config/systemd/user
$ mkdir -p ~/.config/containers/systemd
```

Create the file _~/.config/containers/systemd/echo.container_ with the file contents:

```
[Unit]
Description=Example echo service
Requires=echo.socket
After=echo.socket

[Container]
Image=ghcr.io/eriksjolund/socket-activate-echo
Network=none

[Install]
WantedBy=default.target
```

The file follows the syntax described in [__podman-systemd.unit__(5)](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

The `[Install]` section is optional. If you remove the two last lines, the _echo.service_ will not
be automatically started after a reboot. Instead, the _echo.service_ is started when the first
client connects to the socket.

The line `Network=none` is optional. It improves security by removing network connectivity for the container.
The container can still be serving the internet because `Network=none` has no effect on activated sockets.

A socket-activated service also requires a systemd socket unit.
Create the file _~/.config/systemd/user/echo.socket_ that defines the
sockets that the container should use

```
[Unit]
Description=Example echo socket

[Socket]
ListenStream=127.0.0.1:3000
ListenDatagram=127.0.0.1:3000
ListenStream=[::1]:3000
ListenDatagram=[::1]:3000
ListenStream=%h/echo_stream_sock

# VMADDR_CID_ANY (-1U) = 2^32 -1 = 4294967295
# See "man vsock"
ListenStream=vsock:4294967295:3000

[Install]
WantedBy=sockets.target
```

`%h` is a systemd specifier that expands to the user's home directory.

After editing the unit files, systemd needs to reload its configuration.

```
$ systemctl --user daemon-reload
```

While reloading its configuration systemd generates the unit _echo.service_
from the file _~/.config/containers/systemd/echo.container_
by executing the unit generator `/usr/lib/systemd/system-generators/podman-system-generator`.

Optional: View the generated _echo.service_ to see the `podman run` command that
will be run.

```
$ systemctl --user cat echo.service
```

Configure systemd to automatically start _echo.socket_ after reboots.

```
$ systemctl --user enable echo.socket
```

Pull the container image beforehand

```
$ podman pull ghcr.io/eriksjolund/socket-activate-echo
```

Start the socket unit

```
$ systemctl --user start echo.socket
```

Test the echo server with the program __socat__

```
$ echo hello | socat -t 30 - tcp4:127.0.0.1:3000
hello
$ echo hello | socat -t 30 - tcp6:[::1]:3000
hello
$ echo hello | socat -t 30 - udp4:127.0.0.1:3000
hello
$ echo hello | socat -t 30 - udp6:[::1]:3000
hello
$ echo hello | socat -t 30 - unix:$HOME/echo_stream_sock
hello
$ echo hello | socat -t 30 - VSOCK-CONNECT:1:3000
hello
```

The option `-t 30` configures socat to use a timeout of 30 seconds when socat reads from the socket awaiting to get an EOF (End-Of-File).
As the container image has already been pulled, such a long timeout is not really needed.

The echo server works as expected. It replies _"hello"_ after receiving the text _"hello"_.

### Example: socket activate an Apache HTTP server with systemd-socket-activate

Instead of setting up a systemd service to test out socket activation, an alternative is to use the command-line
tool [__systemd-socket-activate__](https://www.freedesktop.org/software/systemd/man/systemd-socket-activate.html#).

Let's build a container image for the Apache HTTP server that is configured to support socket activation on port 8080.

Create a new directory _ctr_ and a file _ctr/Containerfile_ with this contents

```
FROM docker.io/library/fedora
RUN dnf -y update && dnf install -y httpd && dnf clean all
RUN sed -i "s/Listen 80/Listen 127.0.0.1:8080/g" /etc/httpd/conf/httpd.conf
CMD ["/usr/sbin/httpd", "-DFOREGROUND"]
```

Build the container image

```
$ podman build -t socket-activate-httpd ctr
```

In one shell, start __systemd-socket-activate__.

```
$ systemd-socket-activate -l 8080 podman run --rm --network=none localhost/socket-activate-httpd
```

The TCP port number 8080 is given as an option to __systemd-socket-activate__. The  __--publish__ (__-p__)
option for `podman run` is not used.

In another shell, fetch a web page from _localhost:8080_

```
$ curl -s localhost:8080 | head -6
<!doctype html>
<html>
  <head>
<meta charset='utf-8'>
<meta name='viewport' content='width=device-width, initial-scale=1'>
<title>Test Page for the HTTP Server on Fedora</title>
$
```

### Disabling the network with _--network=none_

If the container only needs to communicate over the socket-activated socket, it's possible to disable
the network by passing __--network=none__ to `podman run`. This improves security because the
container then runs with less privileges.

### Native network performance over the socket-activated socket

When using rootless Podman, network traffic is normally passed through pasta. This comes with
a performance penalty. Fortunately, communication over the socket-activated socket does not pass through
pasta so it has the same performance characteristics as the normal network on the host.

### Starting a socket-activated service

There is a delay when the first connection is made because the container needs to
start up. To minimize this delay, consider passing __--pull=never__ to `podman run` and instead
pull the container image beforehand. Instead of waiting for the start of the service to be triggered by the
first client connecting to it, the service can also be explicitly started (`systemctl --user start echo.service`).

### Stopping a socket-activated service

Some services run a command (configured by the systemd directive __ExecStart__) that exits after some time of inactivity.
Depending on the restart configuration for the service
(systemd directive [__Restart__](https://www.freedesktop.org/software/systemd/man/systemd.service.html#Restart=)),
it may then be stopped. An example of this is _podman.service_ that stops after some time of inactivity.
The service will be started again when the next client connects to the socket.
