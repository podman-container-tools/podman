% podman-version 1

## NAME
podman\-version - Display the Podman version information

## SYNOPSIS
**podman version** [*options*]

## DESCRIPTION
Shows the following information: Remote API Version, Version, Go Version, Git Commit, Build Time,
OS, and Architecture.

## OPTIONS

#### **--format**, **-f**=*format*

Change output format to "json" or a Go template.

| **Placeholder**     | **Description**          |
| ------------------- | ------------------------ |
| .Client ...         | Version of local podman  |
| .Server ...         | Version of remote podman |

Each of the above fields branch deeper into further subfields
such as .Version, .APIVersion, .GoVersion, and more.

## Example

A sample output of the `version` command:
```
$ podman version
Version:      2.0.0
API Version:  1
Go Version:   go1.14.2
Git Commit:   4520664f63c3a7f9a80227715359e20069d95542
Built:        Tue May 19 10:48:59 2020
OS/Arch:      linux/amd64
```

Filtering out only the version:
```
$ podman version --format '{{.Client.Version}}'
2.0.0
```

#### **--help**, **-h**

Print usage statement

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
November 2018, Added --format flag by Tomas Tomecek <ttomecek@redhat.com>
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
