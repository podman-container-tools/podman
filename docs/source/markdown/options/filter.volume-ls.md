####> This option file is used in:
####>   podman volume ls
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--filter**, **-f**=*filter*

Filter what volumes are shown in the output.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *name=test* **--filter** *driver=local*.

Filters with the same key work inclusive, with the only exception being `label`
which is exclusive. Filters with different keys always work exclusive.

Volumes can be filtered by the following attributes:

| **Filter**  | **Description**                                                                       |
| ----------  | ------------------------------------------------------------------------------------- |
| anonymous   | [Bool] Matches anonymous volumes (true) or named volumes (false)                      |
| dangling    | [Dangling] Matches all volumes not referenced by any containers                       |
| driver      | [Driver] Matches volumes based on their driver                                        |
| label       | [Key] or [Key=Value] Label assigned to a volume                                       |
| label!      | [Key] or [Key=Value] Volumes without the specified label                              |
| name        | [Name] Volume name (accepts regex)                                                    |
| opt         | Matches a storage driver options                                                      |
| scope       | Filters volume by scope                                                               |
| after/since | Filter by volumes created after the given VOLUME (name or tag)                        |
| until       | Filter by volumes created before given timestamp                                      |
