# Building the Podman client on macOS

The following describes the process for building the Podman client on macOS.

## Install brew
Podman requires brew -- a package manager for macOS.  This will allow additional packages to be installed that are
needed by Podman.  See the [brew project page](https://brew.sh/) for installation instructions.

## Install build dependencies
Podman requires some software from brew to be able to build.  This can be done using brew from a macOS terminal:

```
$ brew install go go-md2man
```

## Obtain Podman source code

You can obtain the latest source code for Podman from its github repository.

```
$ git clone https://github.com/podman-container-tools/podman go/src/github.com/containers/podman
```

## Build client
After completing the preparatory steps of obtaining the Podman source code and installing its dependencies, the client
can now be built.

```
$ cd go/src/github.com/containers/podman
$ make podman-remote
$ mv bin/darwin/podman bin/podman
```

The binary will be located in bin/
```
$ ls -l bin/
```

### Using gvproxy from homebrew, with podman from git

Recent podman builds depend on a `gvproxy` binary which comes from [containers/gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock).  A common development scenario may be using the podman desktop app as a baseline, with a development
binary of `podman` you build from git.  To ensure that the podman you build here can find the gvproxy installed from podman desktop, use:

`make podman-remote HELPER_BINARIES_DIR=/opt/podman/bin`

(Also note that because the `Makefile` rules do not correctly invalidate the binary when this variable changes,
 so if you already have a build you'll need to `rm bin/darwin/podman` first if you have an existing build).

Alternatively, you can set `helper_binaries_dir=` in `~/.config/containers/containers.conf`.

### Building docs

If you would like to build the docs associated with Podman on macOS:
```
$ make podman-remote-darwin-docs
$ ls docs/build/remote/darwin
```

To install and view these manpages:

```
$ cp -a docs/build/remote/darwin/* /usr/share/man/man1
$ man podman
```

## Using the client

To learn how to use the Podman client, refer to its
[tutorial](https://github.com/podman-container-tools/podman/blob/main/docs/tutorials/remote_client.md).

## Build and test the Podman server

On macOS, `podman` is only a client. The Podman server (the Linux `podman`
binary running `podman system service`) runs inside the Podman machine. The
server code (`libpod`, `pkg/api`, ...) only builds on Linux, so changes to it
are built in a Linux environment. This section describes how to use a
Fedora machine created with Apple's
[`container`](https://github.com/apple/container) tool for that, and how to run
the resulting binary in the Podman machine to test it together with the macOS
client.

:information_source: `container` requires a Mac with Apple silicon and macOS 26.

### Set up a Fedora machine

Install `container` by downloading the signed installer package from its
[releases page](https://github.com/apple/container/releases), then start its
system service:

```
$ container system start
```

The Linux machine must run the same Fedora release as the Podman machine, since
the `podman` binary built in it is dynamically linked against the system
libraries. Retrieve the release used by the Podman machine:

```
$ podman machine ssh "grep PRETTY_NAME /etc/os-release"
```

A container machine boots the image's `/sbin/init`, but the Fedora container
images don't include systemd. Build an image with the matching Fedora release
(e.g. `fedora:45` for Fedora 45) that adds it:

```
$ printf 'FROM docker.io/library/fedora:45\nRUN dnf -y install systemd && dnf clean all\n' | \
    container build -t fedora-machine:45 -f - .
```

Create the build machine from that image. Give it enough resources to build and
test Podman:

```
$ container machine create --name podman-dev --cpus 4 --memory 8G fedora-machine:45
```

By default the macOS home directory is mounted in the machine, so the Podman
repository cloned in the previous steps is available in it. Update the machine
and install git:

```
$ container machine run --name podman-dev --root -- dnf -y update
$ container machine run --name podman-dev --root -- dnf -y install git
```

Then configure the machine for Podman development following the Podman
documentation, using the shell opened by `container machine run --name
podman-dev` (typically `sudo dnf -y builddep rpm/podman.spec` is enough to
configure the machine for building and test Podman).

- [Building from source](https://podman.io/docs/installation#building-from-source):
  lists the build dependencies to install on Fedora, and the required Go version.
- [CONTRIBUTING.md](CONTRIBUTING.md): development workflow and requirements for
  pull requests.
- [test/README.md](test/README.md): additional tools needed to run the
  integration and system tests (e.g. `bats`, `ginkgo`).

### Build and test Podman in the Linux machine

Build and test following the Podman documentation, from a shell opened in the
machine:

- [Building from source](https://podman.io/docs/installation#building-from-source):
  `make binaries` builds everything, `make podman` only the `podman` binary
  (written to `bin/podman`).
- [test/README.md](test/README.md): how to run unit, integration
  (`make localintegration`) and system (`make localsystem`) tests.

Commands can also be run from the macOS terminal, from the root of the
repository, without opening an interactive shell:

```
$ container machine run --name podman-dev --workdir "$PWD" -- make podman
```

:information_source: Because the repository is shared, `make podman` in the
machine writes the Linux binary to `bin/podman`, the same path the macOS client
is moved to in [Build client](#build-client). Don't run the `mv` of that section
for this workflow: use the macOS client at `bin/darwin/podman`, and keep the
Linux binary as `bin/podman`.

### Replace the Podman binary in the Podman machine

To test the server changes with the macOS client, replace the `podman` binary
used by the service in the Podman machine with the one just built. The steps
below assume a running Podman machine named `podman-machine-default`. List the
existing machines with `bin/darwin/podman machine list` and replace the name in
the commands below if it is different (for a machine that is not the default
one, also pass its name to `machine ssh`, e.g. `machine ssh <name> "..."`).
Run the commands from the root of the repository.

1. Stop the Podman service running in the machine. The first time (or after the
   machine is rebooted), stop the systemd unit so that it doesn't restart the
   service:

   ```
   $ bin/darwin/podman machine ssh sudo systemctl stop podman.service
   ```

2. Copy the Linux binary into the machine and make it executable:

   ```
   $ bin/darwin/podman machine cp bin/podman podman-machine-default:/tmp/podman
   $ bin/darwin/podman machine ssh sudo chmod +x /tmp/podman
   ```

3. Start the service with the new binary:

   ```
   $ bin/darwin/podman machine ssh 'nohup /tmp/podman system service --time=0 > /tmp/podman-service.log 2>&1 &'
   ```

4. Verify that the machine is using the new server, then use the client as usual:

   ```
   $ bin/darwin/podman version
   $ bin/darwin/podman run hello
   ```

:information_source: `/tmp` is cleared when the machine restarts, and the
systemd-managed service is replaced by the manually started one only until then.
After a restart, `machine start` brings back the original binary; repeat the
steps above to test again.
