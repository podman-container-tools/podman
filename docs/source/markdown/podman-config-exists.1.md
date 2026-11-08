% podman-config-exists 1

## NAME
podman\-config\-exists - Check if the given config exists

## SYNOPSIS
**podman config exists** *config*

## DESCRIPTION
**podman config exists** checks if a config exists. Podman returns an exit code
of `0` when the config is found. A `1` is returned otherwise. An exit code of
`125` indicates there was another issue.


## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLE

Check if a config called `myconfig` exists.
```
$ podman config exists myconfig
$ echo $?
0
$
```

Check if a config called `mypassword` exists (the config does not actually exist).
```
$ podman config exists mypassword
$ echo $?
1
$
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-config(1)](podman-config.1.md)**

## HISTORY
April 2023, Originally compiled by Ygal Blum `<ygal.blum@gmail.com>`
