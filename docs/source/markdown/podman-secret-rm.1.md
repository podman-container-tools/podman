% podman-secret-rm 1

## NAME
podman\-secret\-rm - Remove one or more secrets

## SYNOPSIS
**podman secret rm** [*options*] *secret* [...]

## DESCRIPTION

Removes one or more secrets.

`podman secret rm` is safe to use on secrets that are in use by a container.
The created container has access to the secret data because secrets are
copied and mounted into the container when a container is created. If a secret is deleted and
another secret is created with the same name, the secret inside the container does not change;
the old secret value still remains.

## OPTIONS

#### **--all**, **-a**

Remove all existing secrets.

#### **--help**

Print usage statement.

#### **--ignore**, **-i**
Ignore errors when specified secrets are not present.

## EXAMPLES

Remove secrets mysecret1 and mysecret2.
```
$ podman secret rm mysecret1 mysecret2
```

Remove all secrets
```
$ podman secret rm --all
3fa78977c813cca1d5b1a4570
4ee314533b16a47d0d8c6e775
```

Removes the specified secrets. No error is thrown if a non-existent secret is included.
```
$ podman secret rm --ignore mysecret1 mysecret2 non_existent_secret
9bb0cad56c4a610da8ebca0cc
3c497981215f1b5dd9ce19cde
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
