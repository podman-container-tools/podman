% podman-pod-unpause 1

## NAME
podman\-pod\-unpause - Unpause one or more pods

## SYNOPSIS
**podman pod unpause** [*options*] *pod* ...

## DESCRIPTION
Unpauses all the paused processes in the containers of one or more pods.  You may use pod IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Unpause all pods.

#### **--latest**, **-l**

Instead of providing the pod name or ID, unpause the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

Unpause pod with a given name:
```
podman pod unpause mywebserverpod
```

Unpause pod with a given ID:
```
podman pod unpause 860a4b23
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-pod-pause(1)](podman-pod-pause.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
