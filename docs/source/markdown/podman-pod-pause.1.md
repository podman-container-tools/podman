% podman-pod-pause 1

## NAME
podman\-pod\-pause - Pause one or more pods

## SYNOPSIS
**podman pod pause** [*options*] *pod* ...

## DESCRIPTION
Pauses all the running processes in the containers of one or more pods.  You may use pod IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Pause all pods.

#### **--latest**, **-l**

Instead of providing the pod name or ID, pause the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLES

Pause a pod with a given name:
```
$ podman pod pause mywebserverpod
```

Pause a pod with a given ID:
```
$ podman pod pause 860a4b23
```

Pause all pods
```
$ podman pod pause --all
817973d45404da08f1fe393a13c8eeb0948f4a259d8835f083370b4a63cb0431
0793d692719c8ef1f983fd29d7568e817c5a8e865e2b3925201a75dce24cfe80
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-pod-unpause(1)](podman-pod-unpause.1.md)**, **[podman-pause(1)](podman-pause.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
