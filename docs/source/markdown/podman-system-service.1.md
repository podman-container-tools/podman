% podman-system-service 1

## NAME
podman\-system\-service - Run an API service

## SYNOPSIS
**podman system service** [*options*]

## DESCRIPTION
The **podman system service** command creates a listening service that answers API calls for Podman.
The command is available on Linux systems and is usually executed in systemd services.
The command is not available when the Podman command is executed directly on a Windows or macOS
host or in other situations where the Podman command is accessing a remote Podman API service.

The REST API provided by **podman system service** is split into two parts: a compatibility layer offering support for the Docker v1.40 API, and a Podman-native Libpod layer.
Documentation for the latter is available at *https://docs.podman.io/en/latest/_static/api.html*.
Both APIs are versioned, but the server does not reject requests with an unsupported version set.

### Run the command in a systemd service

The command **podman system service** supports systemd socket activation.
When the command is run in a systemd service, the API service can therefore be provided on demand.
If the systemd service is not already running, it will be activated as soon as
a client connects to the listening socket. Systemd then executes the
**podman system service** command.
After some time of inactivity, as defined by the __--time__ option, the command terminates.
Systemd sets the _podman.service_ state as inactive. At this point there is no
**podman system service** process running. No unnecessary compute resources are wasted.
As soon as another client connects, systemd activates the systemd service again.

The systemd unit files that declares the Podman API service for users are

* _/usr/lib/systemd/user/podman.service_
* _/usr/lib/systemd/user/podman.socket_

In the file _podman.socket_ the path of the listening Unix socket is defined by

```
ListenStream=%t/podman/podman.sock
```

The path contains the systemd specifier `%t` which systemd expands to the value of the environment variable
`XDG_RUNTIME_DIR` (see systemd specifiers in the **systemd.unit(5)** man page).

In addition to the systemd user services, there is also a systemd system service _podman.service_.
It runs rootful Podman and is accessed from the Unix socket _/run/podman/podman.sock_. See the systemd unit files

* _/usr/lib/systemd/system/podman.service_
* _/usr/lib/systemd/system/podman.socket_

The **podman system service** command does not support more than one listening socket for the API service.

Note: The default systemd unit files (system and user) change the log-level option to *info* from *error*. This change provides additional information on each API call.

### Run the command directly

To support running an API service without using a systemd service, the command also takes an
optional endpoint argument for the API in URI form.  For example, *unix:///tmp/foobar.sock* or *tcp://localhost:8080*.
If no endpoint is provided, defaults is used.  The default endpoint for a rootful
service is *unix:///run/podman/podman.sock* and rootless is *unix://$XDG_RUNTIME_DIR/podman/podman.sock* (for
example *unix:///run/user/1000/podman/podman.sock*)

### Access the Unix socket from inside a container

To access the API service inside a container:
- mount the socket as a volume
- run the container with `--security-opt label=disable`

### Security

Please note that the API grants full access to all Podman functionality, and thus allows arbitrary code execution as the user running the API, with no ability to limit or audit this access.
The API's security model is built upon access via a Unix socket with access restricted via standard file permissions, ensuring that only the user running the service will be able to access it.
TLS can be used to secure this socket by requiring clients to present a certificate signed by a trusted certificate authority ("CA"), as well as to allow the client to verify the identity of the API.
We *strongly* recommend against making the API socket available via the network (IE, bindings the service to a *tcp* URL) without enabling mutual TLS to authenticate the client.
Even access via Localhost carries risks - anyone with access to the system will be able to access the API.
If remote access is required, we instead recommend forwarding the API socket via SSH, and limiting access on the remote machine to the greatest extent possible.
If a *tcp* URL must be used without TLS, using the *--cors* option is recommended to improve security.

## OPTIONS

#### **--cors**

CORS headers to inject to the HTTP response. The default value is empty string which disables CORS headers.

#### **--help**, **-h**

Print usage statement.

#### **--time**, **-t**

The time until the session expires in _seconds_. The default is 5
seconds. A value of `0` means no timeout, therefore the session does not expire.

#### **--tls-cert**=*path*

Path to a PEM file containing the TLS certificate to present to clients. `--tls-key` must also be provided.

#### **--tls-client-ca**=*path*

Path to a PEM file containing the TLS certificate bundle to validate client connections against.
Connections that present no certificate or a certificate not signed by one of these certificates will be rejected.

#### **--tls-key**=*path*

Path to a PEM file containing the private key matching `--tls-cert`. `--tls-cert` must also be provided.

The default timeout can be changed via the `service_timeout=VALUE` field in containers.conf.
See **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** for more information.

## EXAMPLES

Start the user systemd socket for a rootless service.
```
systemctl --user start podman.socket
```

Configure DOCKER_HOST environment variable to point to the Podman socket so that
it can be used via Docker API tools like docker-compose.
```
$ export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
$ docker-compose up
```

Configure the systemd socket to be automatically started after reboots, and run as the specified user.
```
systemctl --user enable podman.socket
loginctl enable-linger <USER>
```

Start the systemd socket for the rootful service.
```
sudo systemctl start podman.socket
```

Configure the socket to be automatically started after reboots.
```
sudo systemctl enable podman.socket
```

It is possible to run the API without using systemd socket activation.
In this case the API will not be available on demand because the command will
stay terminated after the inactivity timeout has passed.
Run an API with an inactivity timeout of 5 seconds without using socket activation.
```
podman system service --time 5
```

The default socket was used as no URI argument was provided.

Run an API service with a custom socket path and no timeout:
```
podman system service --time 0 unix:///var/run/mypodman.sock
```

This starts the API service listening on the custom socket `/var/run/mypodman.sock` with no inactivity timeout (runs indefinitely).

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**

## HISTORY
January 2020, Originally compiled by Brent Baude `<bbaude@redhat.com>`
November 2020, Updated by Jhon Honce (jhonce at redhat dot com)
