% podman-volume-prune 1

## NAME
podman\-volume\-prune - Remove unused volumes

## SYNOPSIS
**podman volume prune** [*options*]

## DESCRIPTION

Remove unused volumes. By default only **anonymous** (unnamed) unused volumes are removed.
Anonymous volumes are those created with a container (e.g. **podman run -v /path** without a volume name).
Use **--all** (or **-a**) to remove all unused volumes, including named ones.

The **--filter** flag can be used to restrict which volumes are considered. Users are prompted to confirm
removal unless **--force** is used.

## OPTIONS

#### **--all**, **-a**

Remove all unused volumes (anonymous and named). Without this option, only anonymous unused volumes are removed.

**--all** can be combined with **--filter**: **--all** widens the set of candidate volumes to all unused ones, and the filters then restrict which of those are removed. For example, **--all --filter label!=keep** removes every unused volume that does not have the *keep* label.

#### **--dry-run**

Show which volumes would be pruned without removing them.

#### **--filter**

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *label=test* **--filter** *until=10m*.

Filters with the same key work inclusive, with the only exception being `label`
which is exclusive. Filters with different keys always work exclusive.

Supported filters:

| Filter      | Description                                                                                                |
|:-----------:|------------------------------------------------------------------------------------------------------------|
| all         | [Bool] When true, remove all unused volumes (same as **--all**). When false or unset, only anonymous volumes are considered. |
| anonymous   | [Bool] Only remove volumes that are anonymous (true) or named (false).                                     |
| label       | [String] Only remove volumes, with (or without, in the case of label!=[...] is used) the specified labels. |
| label!      | [String] Only remove volumes without the specified labels.                                                 |
| until       | [DateTime] Only remove volumes created before given timestamp.                                             |
| after/since | [Volume] Filter by volumes created after the given VOLUME (name or tag)                                    |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes volumes with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes volumes without the specified labels.

**NOTE:** `label!` filters are combined with **AND**, so that the behavior is consistent with `label`, while in Docker, they are combined with **OR**.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

#### **--force**, **-f**

Do not prompt for confirmation.

#### **--help**

Print usage statement


## EXAMPLES

Prune only anonymous unused volumes (default).
```
$ podman volume prune
```

Prune only anonymous unused volumes without confirmation.
```
$ podman volume prune --force
```

Prune all unused volumes (anonymous and named).
```
$ podman volume prune --all --force
```

Prune all unused volumes except those with a specific label (combine **--all** with a filter).
```
$ podman volume prune --all --force --filter label!=keep
```

Prune all unused volumes using the filter (equivalent to **--all**).
```
$ podman volume prune --filter all=true --force
```

Prune unused volumes that have the specified label.
```
$ podman volume prune --filter label=mylabel=mylabelvalue
```

Prune unused volumes that do NOT have a specific label key/value:
```
$ podman volume prune --filter label!=mylabel=mylabelvalue
```

Prune unused volumes that have a specific label key (regardless of value):
```
$ podman volume prune --filter label=environment
```

Prune unused volumes that do NOT have a specific label key:
```
$ podman volume prune --filter label!=environment
```

Preview all unused volumes without removing them.

```
$ podman volume prune --all --dry-run
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
