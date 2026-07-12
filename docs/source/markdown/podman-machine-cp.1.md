% podman-machine-cp 1

## NAME
podman\-machine\-cp - Securely copy contents between the host and the virtual machine

## SYNOPSIS
**podman machine cp** [*options*] *src_path* *dest_path*

## DESCRIPTION

Use secure copy (scp) to copy files or directories between the virtual machine
and your host machine.

`podman machine cp` does not support copying between two virtual machines,
which would require two machines running simultaneously.

Additionally, `podman machine cp` will automatically do a recursive copy of
files and directories.

## OPTIONS

#### **--help**

Print usage statement.

#### **--quiet**, **-q**

Suppress copy status output.

## EXAMPLES
Copy a file from your host to the running Podman Machine.
```
$ podman machine cp ~/configuration.txt podman-machine-default:~/configuration.txt
...
Copy Successful
```

Copy a file from the running Podman Machine to your host.
```
$ podman machine cp podman-machine-default:~/logs/log.txt ~/logs/podman-machine-default.txt
...
Copy Successful
```

Copy a directory from your host to the running Podman Machine.
```
$ podman machine cp ~/.config podman-machine-default:~/.config
...
Copy Successful
```

Copy a directory from the running Podman Machine to your host.
```
$ podman machine cp podman-machine-default:~/.config ~/podman-machine-default.config
...
Copy Successful
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
February 2025, Originally compiled by Jake Correnti <jcorrent@redhat.com>
