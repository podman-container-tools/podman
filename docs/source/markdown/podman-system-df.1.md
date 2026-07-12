% podman-system-df 1

## NAME
podman\-system\-df - Show podman disk usage

## SYNOPSIS
**podman system df** [*options*]

## DESCRIPTION
Show podman disk usage for images, containers and volumes.

Note: The RECLAIMABLE size that is reported for images can be incorrect. It might
report that it can reclaim more than a prune would actually free. This will happen
if you are using different images that share some layers.

## OPTIONS
#### **--format**=*format*

Pretty-print images using a Go template or JSON. This flag is not allowed in combination with **--verbose**

Valid placeholders for the Go template are listed below:

| **Placeholder**           | **Description**                                  |
| ------------------------- | ------------------------------------------------ |
| .Active                   | Indicates whether volume is in use               |
| .RawReclaimable           | Raw reclaimable size of each Type                |
| .RawSize                  | Raw size of each type                            |
| .Reclaimable              | Reclaimable size or each type (human-readable)   |
| .Size                     | Size of each type (human-readable)               |
| .Total                    | Total items for each type                        |
| .Type                     | Type of data                                     |


#### **--verbose**, **-v**
Show detailed information on space usage

## EXAMPLE

Show disk usage:
```
$ podman system df
TYPE            TOTAL   ACTIVE   SIZE    RECLAIMABLE
Images          6       2        281MB   168MB (59%)
Containers      3       1        0B      0B (0%)
Local Volumes   1       1        22B     0B (0%)
```

Show disk usage in verbose mode:
```
$ podman system df -v
Images space usage:

REPOSITORY                 TAG      IMAGE ID       CREATED       SIZE     SHARED SIZE   UNIQUE SIZE   CONTAINERS
docker.io/library/alpine   latest   5cb3aa00f899   2 weeks ago   5.79MB   0B            5.79MB       5

Containers space usage:

CONTAINER ID    IMAGE   COMMAND       LOCAL VOLUMES   SIZE     CREATED        STATUS       NAMES
073f7e62812d    5cb3    sleep 100     1               0B       20 hours ago   exited       zen_joliot
3f19f5bba242    5cb3    sleep 100     0               5.52kB   22 hours ago   exited       pedantic_archimedes
8cd89bf645cc    5cb3    ls foodir     0               58B      21 hours ago   configured   agitated_hamilton
a1d948a4b61d    5cb3    ls foodir     0               12B      21 hours ago   exited       laughing_wing
eafe3e3c5bb3    5cb3    sleep 10000   0               72B      21 hours ago   exited       priceless_liskov

Local Volumes space usage:

VOLUME NAME   LINKS   SIZE
data          1       0B
```

Show only the total count for each type:
```
$ podman system df --format "{{.Type}}\t{{.Total}}"
Images          1
Containers      5
Local Volumes   1
```
Show disk usage in JSON format:
```
$ podman system df --format json
[
    {"Type":"Images","Total":12,"Active":3,"RawSize":13491151377,"RawReclaimable":922956674,"TotalCount":12,"Size":"13.49GB","Reclaimable":"923MB (7%)"},
    {"Type":"Containers","Total":4,"Active":0,"RawSize":209266,"RawReclaimable":209266,"TotalCount":4,"Size":"209.3kB","Reclaimable":"209.3kB (100%)"},
    {"Type":"Local Volumes","Total":6,"Active":1,"RawSize":796638905,"RawReclaimable":47800633,"TotalCount":6,"Size":"796.6MB","Reclaimable":"47.8MB (6%)"}
]
```
Show type and size in a custom format:
```
$ podman system df --format "{{.Type}}: {{.Size}} ({{.Reclaimable}} reclaimable)"

Images: 13.49GB (923MB (7%) reclaimable)
Containers: 209.3kB (209.3kB (100%) reclaimable)
Local Volumes: 796.6MB (47.8MB (6%) reclaimable)
```


## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**

## HISTORY
March 2019, Originally compiled by Qi Wang (qiwan at redhat dot com)
