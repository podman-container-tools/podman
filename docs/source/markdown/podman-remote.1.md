% podman-remote 1

## NAME
podman-remote - A remote CLI for Podman: A Simple management tool for pods, containers and images.

## SYNOPSIS
**podman-remote** [*options*] *command*

## DESCRIPTION
Podman (Pod Manager) is a fully featured container engine that is a simple daemonless tool.
Podman provides a Docker-CLI comparable command line that eases the transition from other
container engines and allows the management of pods, containers and images.  Simply put: `alias docker=podman`.
Most Podman commands can be run as a regular user, without requiring additional
privileges.

Podman uses Buildah(1) internally to create container images. Both tools share image
(not container) storage, hence each can use or manipulate images (but not containers)
created by the other.

Podman-remote provides a local client interacting with a Podman backend node through a RESTful API tunneled through a ssh connection. In this context, a Podman node is a Linux system with Podman installed on it and the API service activated. Credentials for this session can be passed in using flags, environment variables, or in `containers.conf`.

The `containers.conf` file is placed under `$HOME/.config/containers/containers.conf` on Linux and Mac and `%APPDATA%\containers\containers.conf` on Windows.

**podman [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

#### **--connection**=*name*, **-c**

Remote connection name

Overrides environment variable `CONTAINER_CONNECTION` if set.

#### **--help**, **-h**

Print usage statement

#### **--identity**=*path*

Path to ssh identity file. If the identity file has been encrypted, Podman prompts the user for the passphrase.
If no identity file is provided and no user is given, Podman defaults to the user running the podman command.
Podman prompts for the login password on the remote server.

Identity value resolution precedence:
 - command line value
 - environment variable `CONTAINER_SSHKEY`, if `CONTAINER_HOST` is found
 - `containers.conf`

#### **--log-level**=*level*

Log messages above specified level: debug, info, warn, error (default), fatal or panic

#### **--url**=*value*

URL to access Podman service (default from `containers.conf`, rootless "unix:///run/user/$UID/podman/podman.sock" or as root "unix:///run/podman/podman.sock).

 - `CONTAINER_HOST` is of the format `<schema>://[<user[:<password>]@]<host>[:<port>][<path>]`
 - `CONTAINER_PROXY` is of the format `<socks5|socks5h>://[<user[:<password>]@]<host>[:<port>]`

Details:
 - `schema` is one of:
   * `ssh` (default): a local unix(7) socket on the named `host` and `port`, reachable via SSH
   * `tcp`: an unencrypted, unauthenticated TCP connection to the named `host` and `port`, can work with proxy if `CONTAINER_PROXY` is set
   * `unix`: a local unix(7) socket at the specified `path`, or the default for the user
 - `user` defaults to either `root` or the current running user (`ssh` only)
 - `password` has no default (`ssh` only)
 - `host` must be provided and is either the IP or name of the machine hosting the Podman service (`ssh` and `tcp`)
 - `port` defaults to 22 (`ssh` and `tcp`)
 - `path` defaults to either `/run/podman/podman.sock`, or `/run/user/$UID/podman/podman.sock` if running rootless (`unix`), or must be explicitly specified (`ssh`)
 - `CONTAINER_PROXY`: use proxy (`socks5` or `socks5h`) to access Podman service (`tcp` only)

URL value resolution precedence:
 - command line value
 - environment variable `CONTAINER_HOST`
 - `engine.service_destinations` table in containers.conf, excluding the /usr/share/containers directory
 - `unix:///run/podman/podman.sock`

Remote connections use local containers.conf for default.

Some example URL values in valid formats:
 - unix:///run/podman/podman.sock
 - unix:///run/user/$UID/podman/podman.sock
 - ssh://notroot@localhost:22/run/user/$UID/podman/podman.sock
 - ssh://root@localhost:22/run/podman/podman.sock
 - tcp://localhost:34451
 - tcp://127.0.0.1:34451

#### **--version**

Print the version

## Environment Variables

Podman can set up environment variables from env of [engine] table in containers.conf. These variables can be overridden by passing  environment variables before the `podman` commands.

#### **CONTAINERS_CONF**

Set default locations of containers.conf file

#### **CONTAINER_CONNECTION**

Set default `--connection` value to access Podman service.

#### **CONTAINER_HOST**

Set default `--url` value to access Podman service.

#### **CONTAINER_SSHKEY**

Set default `--identity` path to ssh key file value used to access Podman service.

## Exit Status

The exit code from `podman` gives information about why the container
failed to run or why it exited.  When `podman` commands exit with a non-zero code,
the exit codes follow the `chroot` standard, see below:

  **125** The error is with podman itself

    $ podman run --foo busybox; echo $?
    Error: unknown flag: --foo
    125

  **126** Executing a _contained command_ and the _command_ cannot be invoked

    $ podman run busybox /etc; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"/etc\": permission denied": OCI runtime error
    126

  **127** Executing a _contained command_ and the _command_ cannot be found
    $ podman run busybox foo; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"foo\": executable file not found in $PATH": OCI runtime error
    127

  **Exit code** _contained command_ exit code

    $ podman run busybox /bin/sh -c 'exit 3'; echo $?
    3


## COMMANDS

| Command                                          | Description                                                                 |
| ------------------------------------------------ | --------------------------------------------------------------------------- |
| [podman-attach(1)](podman-attach.1.md)           | Attach to a running container.                                              |
| [podman-build(1)](podman-build.1.md)             | Build a container image using a Dockerfile.                                 |
| [podman-commit(1)](podman-commit.1.md)           | Create new image based on the changed container.                            |
| [podman-container(1)](podman-container.1.md)     | Manage containers.                                                          |
| [podman-cp(1)](podman-cp.1.md)                   | Copy files/folders between a container and the local filesystem.            |
| [podman-create(1)](podman-create.1.md)           | Create a new container.                                                     |
| [podman-diff(1)](podman-diff.1.md)               | Inspect changes on a container or image's filesystem.                       |
| [podman-events(1)](podman-events.1.md)           | Monitor Podman events                                                       |
| [podman-export(1)](podman-export.1.md)           | Export a container's filesystem contents as a tar archive.                  |
| [podman-generate(1)](podman-generate.1.md)       | Generate structured data based on containers and pods.                   |
| [podman-healthcheck(1)](podman-healthcheck.1.md) | Manage healthchecks for containers                                          |
| [podman-history(1)](podman-history.1.md)         | Show the history of an image.                                               |
| [podman-image(1)](podman-image.1.md)             | Manage images.                                                              |
| [podman-images(1)](podman-images.1.md)           | List images in local storage.                                               |
| [podman-import(1)](podman-import.1.md)           | Import a tarball and save it as a filesystem image.                         |
| [podman-info(1)](podman-info.1.md)               | Display Podman related system information.                                  |
| [podman-init(1)](podman-init.1.md)               | Initialize a container                                                      |
| [podman-inspect(1)](podman-inspect.1.md)         | Display a container or image's configuration.                               |
| [podman-kill(1)](podman-kill.1.md)               | Kill the main process in one or more containers.                            |
| [podman-load(1)](podman-load.1.md)               | Load an image from a container image archive into container storage.        |
| [podman-logs(1)](podman-logs.1.md)               | Display the logs of a container.                                            |
| [podman-pause(1)](podman-pause.1.md)             | Pause one or more containers.                                               |
| [podman-pod(1)](podman-pod.1.md)                 | Management tool for groups of containers, called pods.                      |
| [podman-port(1)](podman-port.1.md)               | List port mappings for a container.                                         |
| [podman-ps(1)](podman-ps.1.md)                   | Print out information about containers.                                     |
| [podman-pull(1)](podman-pull.1.md)               | Pull an image from a registry.                                              |
| [podman-push(1)](podman-push.1.md)               | Push an image from local storage to elsewhere.                              |
| [podman-restart(1)](podman-restart.1.md)         | Restart one or more containers.                                             |
| [podman-rm(1)](podman-rm.1.md)                   | Remove one or more containers.                                              |
| [podman-rmi(1)](podman-rmi.1.md)                 | Remove one or more locally stored images.                                   |
| [podman-run(1)](podman-run.1.md)                 | Run a command in a new container.                                           |
| [podman-save(1)](podman-save.1.md)               | Save an image to a container archive.                                       |
| [podman-start(1)](podman-start.1.md)             | Start one or more containers.                                               |
| [podman-stop(1)](podman-stop.1.md)               | Stop one or more running containers.                                        |
| [podman-system(1)](podman-system.1.md)           | Manage podman.                                                              |
| [podman-tag(1)](podman-tag.1.md)                 | Add an additional name to a local image.                                    |
| [podman-top(1)](podman-top.1.md)                 | Display the running processes of a container.                               |
| [podman-unpause(1)](podman-unpause.1.md)         | Unpause one or more containers.                                             |
| [podman-version(1)](podman-version.1.md)         | Display the Podman version information.                                     |
| [podman-volume(1)](podman-volume.1.md)           | Manage Volumes.                                                             |
## FILES

**containers.conf** (`$HOME/.config/containers/containers.conf`)

Podman has builtin defaults for command line options. These defaults can be overridden using the containers.conf configuration files.

Users can modify defaults by creating the `$HOME/.config/containers/containers.conf` file. Podman merges its builtin defaults with the specified fields from this file, if it exists. Fields specified in the users file override the built-in defaults.

Podman uses builtin defaults if no containers.conf file is found.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system-service(1)](podman-system-service.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**

### Troubleshooting

See [podman-troubleshooting(7)](https://github.com/containers/podman/blob/main/troubleshooting.md)
for solutions to common issues.
