% podman-generate-spec 1

## NAME
podman\-generate\-spec - Generate Specgen JSON based on containers or pods

## SYNOPSIS
**podman generate spec** [*options*] *container | *pod*

## DESCRIPTION
**podman generate spec** generates SpecGen JSON from Podman Containers and Pods. This JSON can be printed to a file, directly to the command line, or both.

This JSON can then be used as input for the Podman API, specifically for Podman container and pod creation. Specgen is Podman's internal structure for formulating new container-related entities.

## OPTIONS

#### **--compact**, **-c**

Print the output in a compact, one line format. This is useful when piping the data to the Podman API

#### **--filename**, **-f**=**filename**

Output to the given file.

#### **--name**, **-n**

Rename the pod or container, so that it does not conflict with the existing entity. This is helpful when the JSON is to be used before the source pod or container is deleted.

## EXAMPLES

Generate Specgen JSON based on a container.
```
$ podman generate spec container1
{
 "name": "container1-clone",
 "command": [
  "/bin/sh"
 ],
 "env": {
  "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
  "container": "podman"
 },
 "sdnotifyMode": "container",
 "pidns": {
  "nsmode": "default"
 },
 "utsns": {
  "nsmode": "private"
 },
 "containerCreateCommand": [
  "podman",
  "run",
  "--name",
  "container1",
  "cea2ff433c61"
 ],
 "init_container_type": "",
 "image": "cea2ff433c610f5363017404ce989632e12b953114fefc6f597a58e813c15d61",
 "ipcns": {
  "nsmode": "default"
 },
 "shm_size": 65536000,
 "shm_size_systemd": 0,
 "selinux_opts": [
  "disable"
 ],
 "userns": {
  "nsmode": "default"
 },
 "idmappings": {
  "HostUIDMapping": true,
  "HostGIDMapping": true,
  "UIDMap": null,
  "GIDMap": null,
  "AutoUserNs": false,
  "AutoUserNsOpts": {
   "Size": 0,
   "InitialSize": 0,
   "PasswdFile": "",
   "GroupFile": "",
   "AdditionalUIDMappings": null,
   "AdditionalGIDMappings": null
  }
 },
 "umask": "0022",
 "cgroupns": {
  "nsmode": "default"
 },
 "netns": {
  "nsmode": "pasta"
 },
 "Networks": null,
 "use_image_hosts": false,
 "resource_limits": {}
}
```

Generate Specgen JSON based on a container. The output is single line.
```
$ podman generate spec --compact container1
{"name":"container1-clone","command":["/bin/sh"],...
```

Generate Specgen JSON based on a container, writing the output to the specified file.
```
$ podman generate spec --filename output.json container1
output.json
$ cat output.json
{
 "name": "container1-clone",
 "command": [
  "/bin/sh"
 ],
 "env": {
  "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
  "container": "podman"
 },
 "sdnotifyMode": "container",
 "pidns": {
  "nsmode": "default"
 },
 "utsns": {
  "nsmode": "private"
 },
 "containerCreateCommand": [
  "podman",
  "run",
  "--name",
  "container1",
  "cea2ff433c61"
 ],
 "init_container_type": "",
 "image": "cea2ff433c610f5363017404ce989632e12b953114fefc6f597a58e813c15d61",
 "ipcns": {
  "nsmode": "default"
 },
 "shm_size": 65536000,
 "shm_size_systemd": 0,
 "selinux_opts": [
  "disable"
 ],
 "userns": {
  "nsmode": "default"
 },
 "idmappings": {
  "HostUIDMapping": true,
  "HostGIDMapping": true,
  "UIDMap": null,
  "GIDMap": null,
  "AutoUserNs": false,
  "AutoUserNsOpts": {
   "Size": 0,
   "InitialSize": 0,
   "PasswdFile": "",
   "GroupFile": "",
   "AdditionalUIDMappings": null,
   "AdditionalGIDMappings": null
  }
 },
 "umask": "0022",
 "cgroupns": {
  "nsmode": "default"
 },
 "netns": {
  "nsmode": "pasta"
 },
 "Networks": null,
 "use_image_hosts": false,
 "resource_limits": {}
}
```
