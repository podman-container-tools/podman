% podman-system-check 1

## NAME
podman\-system\-check - Perform consistency checks on image and container storage

## SYNOPSIS
**podman system check** [*options*]

## DESCRIPTION
Perform consistency checks on image and container storage, reporting images and
containers which have identified issues.

## OPTIONS

#### **--force**, **-f**

When attempting to remove damaged images, also remove containers which depend
on those images.  By default, damaged images which are being used by containers
are left alone.

Containers which depend on damaged images do so regardless of which engine
created them, but because podman only "knows" how to shut down containers that
it started, the effect on still-running containers which were started by other
engines is difficult to predict.

#### **--max**, **-m**=*duration*

When considering layers which are not used by any images or containers, assume
that any layers which are more than *duration* old are the results of canceled
attempts to pull images, and should be treated as though they are damaged.

#### **--quick**, **-q**

Skip checks which are known to be time-consuming.  This will prevent some types
of errors from being detected.

The exact checks performed by this option are subject to change.

#### **--repair**, **-r**

Remove any images which are determined to have been damaged in some way, unless
they are in use by containers.  Use **--force** to remove containers which
depend on damaged images, and those damaged images, as well.

## EXAMPLE

A reasonably quick check:
```
podman system check --quick --repair --force
```

A more thorough check:
```
podman system check --repair --max=1h --force
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**

## HISTORY
April 2024
