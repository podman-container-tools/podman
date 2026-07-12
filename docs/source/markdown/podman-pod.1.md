% podman-pod 1

## NAME
podman\-pod - Management tool for groups of containers, called pods

## SYNOPSIS
**podman pod** *subcommand*

## DESCRIPTION
podman pod is a set of subcommands that manage pods, or groups of containers.

## SUBCOMMANDS

| Command | Man Page                                          | Description                                                                       |
| ------- | ------------------------------------------------- | --------------------------------------------------------------------------------- |
| clone   | [podman-pod-clone(1)](podman-pod-clone.1.md)      | Create a copy of an existing pod.                                                 |
| create  | [podman-pod-create(1)](podman-pod-create.1.md)    | Create a new pod.                                                                 |
| exists  | [podman-pod-exists(1)](podman-pod-exists.1.md)    | Check if a pod exists in local storage.                                           |
| inspect | [podman-pod-inspect(1)](podman-pod-inspect.1.md)  | Display information describing a pod.                                             |
| kill    | [podman-pod-kill(1)](podman-pod-kill.1.md)        | Kill the main process of each container in one or more pods.                      |
| logs    | [podman-pod-logs(1)](podman-pod-logs.1.md)        | Display logs for pod with one or more containers.                                 |
| pause   | [podman-pod-pause(1)](podman-pod-pause.1.md)      | Pause one or more pods.                                                           |
| prune   | [podman-pod-prune(1)](podman-pod-prune.1.md)      | Remove all stopped pods and their containers.                                     |
| ps      | [podman-pod-ps(1)](podman-pod-ps.1.md)            | Print out information about pods.                                                 |
| restart | [podman-pod-restart(1)](podman-pod-restart.1.md)  | Restart one or more pods.                                                         |
| rm      | [podman-pod-rm(1)](podman-pod-rm.1.md)            | Remove one or more stopped pods and containers.                                   |
| start   | [podman-pod-start(1)](podman-pod-start.1.md)      | Start one or more pods.                                                           |
| stats   | [podman-pod-stats(1)](podman-pod-stats.1.md)      | Display a live stream of resource usage stats for containers in one or more pods. |
| stop    | [podman-pod-stop(1)](podman-pod-stop.1.md)        | Stop one or more pods.                                                            |
| top     | [podman-pod-top(1)](podman-pod-top.1.md)          | Display the running processes of containers in a pod.                             |
| unpause | [podman-pod-unpause(1)](podman-pod-unpause.1.md)  | Unpause one or more pods.                                                         |

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
