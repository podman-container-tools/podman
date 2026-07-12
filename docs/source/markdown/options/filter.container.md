####> This option file is used in:
####>   podman pause, ps, restart, rm, start, stop, unpause
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--filter**, **-f**=*filter*

Filter what containers <<subcommand>>.
Multiple filters can be given with multiple uses of the --filter flag.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter**           | **Description**                                                                                 |
|----------------------|-------------------------------------------------------------------------------------------------|
| id                   | [ID] Container's ID (CID prefix match by default; accepts regex)                                |
| name                 | [Name] Container's name (accepts regex)                                                         |
| annotation           | [Key] or [Key=Value] Annotation assigned to a container                                         |
| annotation!          | [Key] or [Key=Value] Annotation NOT assigned to a container                                     |
| label                | [Key] or [Key=Value] Label assigned to a container                                              |
| label!               | [Key] or [Key=Value] Label NOT assigned to a container                                          |
| exited               | [Int] Container's exit code                                                                     |
| status               | [Status] Container's status: 'created', 'initialized', 'running', 'stopped', 'paused', 'exited', 'removing', 'stopping', 'unknown' |
| ancestor             | [ImageName] Image or descendant used to create container (accepts regex)                        |
| before               | [ID] or [Name] Containers created before this container                                         |
| since                | [ID] or [Name] Containers created since this container                                          |
| volume               | [VolumeName] or [MountpointDestination] Volume mounted in container                             |
| health               | [Status] healthy or unhealthy                                                                   |
| pod                  | [Pod] name or full or partial ID of pod                                                         |
| network              | [Network] name or full ID of network                                                            |
| restart-policy       | [Policy] Container's restart policy (e.g., 'no', 'on-failure', 'always', 'unless-stopped')  |
| until                | [DateTime] Containers created before the given duration or time.                                |
| command              | [Command] the command the container is executing, only argv[0] is taken                         |
| should-start-on-boot | [Bool] Containers that need to be restarted after system reboot. True for containers with restart policy 'always', or 'unless-stopped' that were not explicitly stopped by the user |
