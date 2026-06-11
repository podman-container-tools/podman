% podman-quadlet-rm 1

## NAME
podman\-quadlet\-rm - Removes an installed quadlet

## SYNOPSIS
**podman quadlet rm** [*options*] *quadlet|application* [*quadlet|application*]...

## DESCRIPTION

Remove one or more installed Quadlets from the current user. Following command also takes application name
as input and removes all the Quadlets which belongs to that specific application.

When the argument is uninstantiated template quadlet, this command removes the template quadlet file (e.g. `templateName@.container`) and the generated systemd template unit (e.g. `templateName@.service`). If there are running instances of that systemd template, the command fails if **--force** option is not set, and tries to stop the instances if **--force** option is set.

Note: If a quadlet is part of an application, removing that specific quadlet will remove the entire application.
When a quadlet is installed from a directory, all files installed from that directory—including both quadlet and non-quadlet files—are considered part
of a single application.

## OPTIONS

#### **--all**, **-a**

Remove all Quadlets for the current user.

#### **--force**, **-f**

Remove running Quadlets (in case of uninstantiated template quadlets, stop its instances).

#### **--ignore**, **-i**

Do not error for Quadlets that do not exist.

#### **--recursive**

Required when removing applications (default false).

#### **--reload-systemd**

Reload systemd after removing Quadlets (default true).
In order to disable it users need to manually set the value
of this flag to `false`.

## EXAMPLES

```
$ podman quadlet rm myquadlet.container
myquadlet.container
$ podman quadlet rm --recursive myapp
web.container
data.container
data.volume
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**, **[podman-systemd.unit(5)](podman-systemd.unit.5.md)**
