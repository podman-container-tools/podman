% podman-secret-create 1

## NAME
podman\-secret\-create - Create a new secret

## SYNOPSIS
**podman secret create** [*options*] *name* *file|-*

## DESCRIPTION

Creates a secret using standard input or from a file for the secret content.

Create accepts a path to a file, or `-`, which tells podman to read the secret from stdin

A secret is a blob of sensitive data which a container needs at runtime but
is not stored in the image or in source control, such as usernames and passwords,
TLS certificates and keys, SSH keys or other important generic strings or binary content (up to 512 kB in size).

Secrets are not committed to an image with `podman commit`, and do not get committed in the archive created by a `podman export` command.

Secrets can also be used to store passwords for `podman login` to authenticate against container registries.

## OPTIONS

#### **--driver**, **-d**=*driver*

Specify the secret driver (default **file**).

#### **--driver-opts**=*key1=val1,key2=val2*

Specify driver specific options.

#### **--env**=*false*

Read secret data from environment variable.

#### **--help**

Print usage statement.

#### **--ignore**=*false*

If a secret with the same name already exists, do not return an error and return the existing secret's ID instead of creating a new one.
Cannot be used with `--replace`.
The default is **false**.

#### **--label**, **-l**=*label*

Set metadata for the secret (e.g., `--label mykey=value`). Can be used multiple times.
These labels can be viewed in `podman secret inspect` or `podman secret ls`.

#### **--replace**=*false*

If existing secret with the same name already exists, update the secret.
The `--replace` option does not change secrets within existing containers, only newly created containers.
Cannot be used with `--ignore`.
 The default is **false**.

## SECRET DRIVERS

#### file

Secret resides in a read-protected file.

#### pass

Secret resides in a GPG-encrypted file.

#### shell

Secret is managed by custom scripts. An environment variable **SECRET_ID**
is passed to the scripts (except for **list**), and secrets are communicated
via stdin/stdout (where applicable). Driver options **list**, **lookup**,
**store**, and **delete** serve to install the scripts:

```
[secrets]
driver = "shell"

[secrets.opts]
list =
lookup =
store =
delete =
```

## EXAMPLES

Create the specified secret based on a local file.
```
echo -n mysecret > ./secret.txt
$ podman secret create my_secret ./secret.txt
```

Create the specified secret via stdin.
```
$ printf <secret> | podman secret create my_secret -
```

Create or rotate a cryptographically secure random secret just under the maximum `512000` bytes via stdin.
```
openssl rand -base64 378000 | podman secret create --replace my_secret -
```

Mount a local file-based secret securely in a container.
```
podman run --rm --secret source=my_secret,type=mount,uid=1001,gid=1001,mode=440 docker.io/library/alpine ls -l /run/secrets/my_secret
```

Create gpg encrypted secret based on a local file using the pass driver.
```
$ podman secret create --driver=pass my_secret ./secret.txt.gpg
```

Create a secret from an environment variable called 'MYSECRET'.
```
$ podman secret create --env=true my_secret MYSECRET
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**, **[podman-login(1)](podman-login.1.md)**, **[podman-run(1)](podman-run.1.md)**

## HISTORY
* January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
* February 2024, Added example showing secret creation from an environment variable by Brett Calliss <brett@obligatory.email>
* May 2025, Added example showing secure secret generation / rotation & mounting by [Stuart Cardall](https://github.com/itoffshore)
