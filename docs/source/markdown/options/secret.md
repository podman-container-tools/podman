####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Secret=secret[,opt=opt ...]`
<< else >>
#### **--secret**=*secret[,opt=opt ...]*
<< endif >>

Give the container access to a secret. Can be specified multiple times.

A secret is a blob of sensitive data which a container needs at runtime but
is not stored in the image or in source control, such as usernames and passwords,
TLS certificates and keys, SSH keys or other important generic strings or binary content (up to 512 kB in size).

When secrets are specified as type `mount`, the secrets are copied and mounted into the container when a container is created.
When secrets are specified as type `env`, the secret is set as an environment variable within the container.
Secrets are written in the container at the time of container creation, and modifying the secret using `podman secret` commands
after the container is created affects the secret inside the container.

Secrets and its storage are managed using the `podman secret` command.

Secret Options

- `type=mount|env`    : How the secret is exposed to the container.
                        `mount` mounts the secret into the container as a file.
                        `env` exposes the secret as an environment variable.
                        Defaults to `mount`.
- `target=target`     : Target of secret.
                        For mounted secrets, this is the path to the secret inside the container.
                        If a fully qualified path is provided, the secret is mounted at that location.
                        Otherwise, the secret is mounted to
                        `/run/secrets/target` for Linux containers or
                        `/var/run/secrets/target` for FreeBSD containers.
                        If the target is not set, the secret is mounted to `/run/secrets/secretname` by default.
                        For env secrets, this is the environment variable key. Defaults to `secretname`.
- `uid=0`             : UID of secret. Defaults to 0. Mount secret type only.
- `gid=0`             : GID of secret. Defaults to 0. Mount secret type only.
- `mode=0`            : Mode of secret. Defaults to 0444. Mount secret type only.


Examples

Mount at `/my/location/mysecret` with UID 1:
```
--secret mysecret,target=/my/location/mysecret,uid=1
```

Mount at `/run/secrets/customtarget` with mode 0777:
```
--secret mysecret,target=customtarget,mode=0777
```

Create a secret environment variable called `ENVSEC`:
```
--secret mysecret,type=env,target=ENVSEC
```
