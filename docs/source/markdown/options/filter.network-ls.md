####> This option file is used in:
####>   podman network ls
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--filter**, **-f**=*filter=value*

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *name=test* **--filter** *driver=bridge*.

Supported filters:

| **Filter** | **Description**                                                                                  |
| ---------- | ------------------------------------------------------------------------------------------------ |
| driver     | Filter by driver type.                                                                           |
| id         | Filter by full or partial network ID.                                                            |
| label      | Filter by network with (or without, in the case of label!=[...] is used) the specified labels.   |
| name       | Filter by network name (accepts `regex`).                                                        |
| until      | Filter by networks created before given timestamp.                                               |
| dangling   | Filter by networks with no containers attached.                                                  |


The `driver` filter accepts values: `bridge`, `macvlan`, `ipvlan`.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which shows networks with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which shows networks without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

The `dangling` *filter* accepts values `true` or `false`.
