####> This option file is used in:
####>   podman pod ps
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--filter**, **-f**=*filter*

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *label=test* **--filter** *until=10m*.

Supported filters:

| Filter     | Description                                                                                      |
|------------|--------------------------------------------------------------------------------------------------|
| ctr-ids    | Filter by container ID within the pod. (CID prefix match by default; accepts regex)              |
| ctr-names  | Filter by container name within the pod.                                                         |
| ctr-number | Filter by number of containers in the pod.                                                       |
| ctr-status | Filter by container status within the pod.                                                       |
| id         | Filter by pod ID. (Prefix match by default; accepts regex)                                       |
| label      | Filter by container with (or without, in the case of label!=[...] is used) the specified labels. |
| name       | Filter by pod name.                                                                              |
| network    | Filter by network name or full ID of network.                                                    |
| status     | Filter by pod status.                                                                            |
| until      | Filter by pods created before given timestamp.                                                   |

The `ctr-ids`, `ctr-names`, `id`, `name` filters accept `regex` format.

The `ctr-status` filter accepts values: `created`, `running`, `paused`, `stopped`, `exited`, `unknown`.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes containers with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes containers without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

The `status` filter accepts values: `stopped`, `running`, `paused`, `exited`, `dead`, `created`, `degraded`.
