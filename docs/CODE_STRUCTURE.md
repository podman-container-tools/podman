# Podman Codebase structure

Description about important directories in our repository.

### bin/

- Build binaries are put here, podman, podman-remote, quadlet, etc...

### cmd/

 - Each directory here builds its own binary.

#### cmd/podman/

- Podman CLI code, CLI commands and flags are defined here, we are using the [Cobra CLI library](https://github.com/spf13/cobra) for command line parsing.

#### cmd/quadlet/

- Quadlet CLI code.

### contrib/

- CI scripts, packaging files some container image build files.

### docs/

- Sphinx based documentation for Podman that is build on [Read the Docs](https://readthedocs.com/) and hosted at [docs.podman.io](https://docs.podman.io/).
- More information is found in [README.md](./README.md).

### libpod/ (only works on linux and freebsd)
- Underlying core for most Podman operations, defines container, pod, volume management operations.
- Contains the database to store these information on disk, either Sqlite or Botldb (our old db format).
- Integrates with our other libraries such as:
	- containers/storage to create and mount container storage.
	- containers/buildah for building images.
	- containers/common/libnetwork for network management.

### pkg/

- Various packages to do all sorts of different things.

#### pkg/api/

- The HTTP REST API server code.

#### pkg/bindings/

- The HTTP REST API client code.
- This package must have a stable API as it is for use by external consumers as well.

#### pkg/domain/

- "glue" code between cli and the actual operations performed.

##### pkg/domain/entities/

- The package defines two interfaces (ContainerEngine, ImageEngine) that more or less have a function for each cli command defined.
- It also defines a lot of types (structs) for the various options the functions accept.

##### pkg/domain/infra/tunnel/

- Implements the two interfaces for the remote mode (podman-remote) which just maps each operations to the bindings code from pkg/bindings which then talks to the remote server.

##### pkg/domain/infra/abi/

- Implements the two interfaces for the local mode (podman) that calls then directly into the core parts of libpod/ or our other libraries containers/{common,image,storage}...

#### pkg/libartifact/

- Core code for the new podman artifact commands.

#### pkg/machine/

- Core code for podman machine commands.

##### pkg/machine/e2e/

- e2e tests for podman machine commands.
- Runs on Windows, MacOS and Linux.

### test/

- Various tests suites, see the test [README.md](../test/README.md) for more details.
- These run on linux only.

### vendor/

- Directory created with "go mod vendor".
- This includes all go deps in our repo, DO NOT edit this directory directly, changes in dependencies must be made in their respective upstream repositories and then updated in go.mod.
