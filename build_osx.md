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
$ git clone https://github.com/containers/podman go/src/github.com/containers/podman
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
[tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/remote_client.md).
