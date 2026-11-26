% podman-system-hyperv-prep 1

## NAME
podman\-system\-hyperv\-prep - A Windows administrator command to prepare a host that is going to run Hyper-V based Podman machines

## SYNOPSIS
**podman system hyperv-prep** [*options*]

## DESCRIPTION
Prepare a Windows host to run Hyper-V based Podman machines by creating the
required registry entries in HKEY_LOCAL_MACHINE for Hyper-V VSock communication
and adding the current user to the Hyper-V Administrators group.

The registry entries are marked to persist even when all machines are removed,
and the group membership allows the user to manage Hyper-V virtual machines.
Together, these avoid the need for administrator privileges during normal machine
operations.

This command requires administrator privileges and is only available on Windows.
The **--status** option can be used without administrator privileges.

## OPTIONS

#### **--mounts**=*number*

Number of VSock entries for mounting purposes. Every mounted host folder of a running machine needs a dedicated VSock. There should be enough VSock to satisfy the needs of running Podman machines on the host. The default is the number of volumes mounted by default in a machine (the `volumes` option in the `[machine]` table of **containers.conf(5)**), which is **3** with the default configuration.

#### **--force**, **-f**

Skip confirmation prompts during reset. Only valid with **--reset**.

#### **--registry-only**

Only create the VSock registry entries or, with **--reset**, only remove them.
The current user's Hyper-V Administrators group membership is left unchanged.
Not valid with **--status**.

#### **--reset**

Remove all Podman VSock registry entries and remove the current user from the
Hyper-V Administrators group. Prompts for confirmation before each action unless
**--force** is specified. Use **--registry-only** to keep the group membership.

#### **--status**

Show the list of VSock registry entries and the current user's Hyper-V
Administrators group membership status.

## EXAMPLE

Create the required registry entries and add the current user to the Hyper-V
Administrators group:
```powershell
PS> podman system hyperv-prep
```

Create the required registry entries without changing group membership:
```powershell
PS> podman system hyperv-prep --registry-only
```

Show existing registry entries and group membership status:
```powershell
PS> podman system hyperv-prep --status
```

Remove all Podman VSock registry entries and the current user from the Hyper-V
Administrators group (with confirmation prompts):
```powershell
PS> podman system hyperv-prep --reset
```

Reset without confirmation prompts:
```powershell
PS> podman system hyperv-prep --reset --force
```

Remove the registry entries without changing group membership:
```powershell
PS> podman system hyperv-prep --reset --registry-only --force
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**
