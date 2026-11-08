% podman-config-create 1

## NAME
podman\-config\-create - Create a new config

## SYNOPSIS
**podman config create** [*options*] *name* *file|-*

## DESCRIPTION

Creates a config using standard input or from a file for the config content.

Create accepts a path to a file, or `-`, which tells podman to read the config from stdin

A config is a blob of application configuration data that a container needs at runtime. Configs are stored separately from container images and can contain text or binary content (up to 512 kB in size).

Configs are not committed to an image with `podman commit`, and do not get committed in the archive created by a `podman export` command.

## OPTIONS

#### **--driver**, **-d**=*driver*

Specify the config driver (default **file**).

#### **--driver-opts**=*key1=val1,key2=val2*

Specify driver specific options.

#### **--env**=*false*

Read config data from environment variable.

#### **--help**

Print usage statement.

#### **--ignore**=*false*

If a config with the same name already exists, do not return an error and return the existing config's ID instead of creating a new one.
Cannot be used with `--replace`.
The default is **false**.

#### **--label**, **-l**=*label*

Set metadata for the config (e.g., `--label mykey=value`). Can be used multiple times.
These labels can be viewed in `podman config inspect` or `podman config ls`.

#### **--replace**=*false*

If existing config with the same name already exists, update the config.
The `--replace` option does not change configs within existing containers, only newly created containers.
Cannot be used with `--ignore`.
 The default is **false**.

## CONFIG DRIVERS

#### file

Config resides in a file protected from other users.

#### pass

Config resides in a GPG-encrypted file.

#### shell

Config is managed by custom scripts. An environment variable **CONFIG_ID**
is passed to the scripts (except for **list**), and configs are communicated
via stdin/stdout (where applicable). Driver options **list**, **lookup**,
**store**, and **delete** serve to install the scripts:

```
[configs]
driver = "shell"

[configs.opts]
list =
lookup =
store =
delete =
```

## EXAMPLES

Create the specified config based on a local file.
```
echo -n myconfig > ./config.txt
$ podman config create my_config ./config.txt
```

Create the specified config via stdin.
```
$ printf <config> | podman config create my_config -
```

Create or rotate a cryptographically secure random config just under the maximum `512000` bytes via stdin.
```
openssl rand -base64 378000 | podman config create --replace my_config -
```

Mount a config read-only in a container.
```
$ podman run --rm --config source=my_config,target=/run/configs/my_config docker.io/library/alpine ls -l /run/configs/my_config
```

Create gpg encrypted config based on a local file using the pass driver.
```
$ podman config create --driver=pass my_config ./config.txt.gpg
```

Create a config from an environment variable called 'MYCONFIG'.
```
$ podman config create --env=true my_config MYCONFIG
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-config(1)](podman-config.1.md)**, **[podman-login(1)](podman-login.1.md)**, **[podman-run(1)](podman-run.1.md)**

## HISTORY
* January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
* February 2024, Added example showing config creation from an environment variable by Brett Calliss <brett@obligatory.email>
* May 2025, Added example showing secure config generation / rotation & mounting by [Stuart Cardall](https://github.com/itoffshore)
