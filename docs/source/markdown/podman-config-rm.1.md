% podman-config-rm 1

## NAME
podman\-config\-rm - Remove one or more configs

## SYNOPSIS
**podman config rm** [*options*] *config* [...]

## DESCRIPTION

Removes one or more configs.

`podman config rm` is safe to use on configs that are in use by a container.
The created container has access to the config data because configs are
copied and mounted into the container when a container is created. If a config is deleted and
another config is created with the same name, the config inside the container does not change;
the old config value still remains.

## OPTIONS

#### **--all**, **-a**

Remove all existing configs.

#### **--help**

Print usage statement.

#### **--ignore**, **-i**
Ignore errors when specified configs are not present.

## EXAMPLES

Remove configs myconfig1 and myconfig2.
```
$ podman config rm myconfig1 myconfig2
```

Remove all configs
```
$ podman config rm --all
3fa78977c813cca1d5b1a4570
4ee314533b16a47d0d8c6e775
```

Removes the specified configs. No error is thrown if a non-existent config is included.
```
$ podman config rm --ignore myconfig1 myconfig2 non_existent_config
9bb0cad56c4a610da8ebca0cc
3c497981215f1b5dd9ce19cde
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-config(1)](podman-config.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
