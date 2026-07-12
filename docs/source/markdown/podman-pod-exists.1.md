% podman-pod-exists 1

## NAME
podman-pod-exists - Check if a pod exists in local storage

## SYNOPSIS
**podman pod exists** *pod*

## DESCRIPTION
**podman pod exists** checks if a pod exists in local storage. The **ID** or **Name**
of the pod may be used as input.  Podman returns an exit code
of `0` when the pod is found.  A `1` is returned otherwise. An exit code of `125` indicates there
was an issue accessing the local storage.

## EXAMPLES

Check if specified pod exists in local storage (the pod does actually exist):
```
$ sudo podman pod exists web; echo $?
0
```

Check if specified pod exists in local storage (the pod does not actually exist):
```
$ sudo podman pod exists backend; echo $?
1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
