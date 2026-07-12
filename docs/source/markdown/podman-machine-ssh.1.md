% podman-machine-ssh 1

## NAME
podman\-machine\-ssh - SSH into a virtual machine

## SYNOPSIS
**podman machine ssh** [*options*] [*name*] [*command* [*arg* ...]]

## DESCRIPTION

SSH into a Podman-managed virtual machine and optionally execute a command
on the virtual machine. Unless using the default virtual machine, the
first argument must be the virtual machine name. The optional command to
execute can then follow. If no command is provided, an interactive session
with the virtual machine is established.

The exit code from ssh command is forwarded to the podman machine ssh caller, see [Exit Codes](#Exit-Codes).

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be SSH'd into.

Rootless only.

## OPTIONS

#### **--help**

Print usage statement.

#### **--username**=*name*

Username to use when SSH-ing into the VM.

## Exit Codes

The exit code from `podman machine ssh` gives information about why the command failed.
When `podman machine ssh` commands exit with a non-zero code,
the exit codes follow the `chroot` standard, see below:

  **125** The error is with podman **_itself_**

    $ podman machine ssh --foo; echo $?
    Error: unknown flag: --foo
    125

  **126** Executing a _contained command_ and the _command_ cannot be invoked

    $ podman machine ssh /etc; echo $?
    Error: fork/exec /etc: permission denied
    126

  **127** Executing a _contained command_ and the _command_ cannot be found

    $ podman machine ssh foo; echo $?
    Error: fork/exec /usr/bin/bogus: no such file or directory
    127

  **Exit code** _contained command_ exit code

    $ podman machine ssh /bin/sh -c 'exit 3'; echo $?
    3

## EXAMPLES

To get an interactive session with the default Podman machine:

SSH into the default Podman machine.
```
$ podman machine ssh
```

Run command inside the default Podman machine via ssh.
```
$ podman machine ssh myvm
```

Run command inside the specified Podman machine via ssh.
```
$ podman machine ssh myvm rpm -q podman
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
