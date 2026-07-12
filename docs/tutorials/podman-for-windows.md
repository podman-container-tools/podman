<!-- markdownlint-configure-file { "heading-style": { "style": "setext_with_atx" } } -->
<!-- markdownlint-disable MD041 -->
![The Podman logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

Podman for Windows
==================

While "containers are Linux," Podman also runs on Mac and Windows, where it
provides a native CLI and embeds a guest Linux system to launch your
containers. This guest is referred to as a Podman machine and is managed with
the `podman machine` command.

On Windows, each Podman machine is backed by a virtualized Windows Subsystem for
Linux (WSLv2) distribution or an Hyper-V virtual machine.

The Podman command can be run directly from your Windows PowerShell (or CMD)
prompt, where it remotely communicates with the podman service running in the
guest environment. In addition to command-line access, Podman also listens for
Docker API clients, supporting direct usage of Docker-based tools and
programmatic access from your language of choice.

Table of Contents
------------------

- [Prerequisites](#prerequisites)
- [Installing Podman](#installing-podman)
  - [Directories, files and registry keys used by Podman on Windows](#directories-files-and-registry-keys-used-by-podman-on-windows)
- [Machine Init Process](#machine-init-process)
- [Starting Machine](#starting-machine)
- [First Podman Command](#first-podman-command)
- [Port Forwarding](#port-forwarding)
- [Using API Forwarding](#using-api-forwarding)
- [Rootful & Rootless](#rootful--rootless)
- [Configuring the Machine Provider](#configuring-the-machine-provider)
- [Volume Mounting](#volume-mounting)
- [Listing Podman Machine(s)](#listing-podman-machines)
- [Accessing the Podman Linux Environment](#accessing-the-podman-linux-environment)
- [Using SSH](#using-ssh)
- [Using the WSL Command](#using-the-wsl-command)
- [Using Windows Terminal Integration](#using-windows-terminal-integration)
- [Stopping a Podman Machine](#stopping-a-podman-machine)
- [Removing a Podman Machine](#removing-a-podman-machine)
- [Uninstalling Podman](#uninstalling-podman)
- [Troubleshooting](#troubleshooting)
  - [Installing WSL Manually](#installing-wsl-manually)
- [Install Certificate Authority](#install-certificate-authority)

Prerequisites
-------------

Because Podman uses WSLv2 or Hyper-V recent features, you need Windows 11 or
later. Internally, WSL and Hyper-V use virtualization, so your system must
support and have hardware virtualization enabled. If you are running Windows on
a VM, you must have a VM that supports nested virtualization.

Hyper-V is only available on Windows Enterprise, Pro, or Education editions (not
Home). The command to initialize the first Hyper-V Podman machine, and the command
to remove the last one, both require administrator privileges because they create
and delete machine-scope registry keys. Additionally, all Hyper-V machine commands
require that the current user is a member of the Hyper-V Administrators group. To
avoid these requirements, an administrator can run `podman system hyperv-prep`
once to pre-create the registry entries and add the current user to the Hyper-V
Administrators group, after which no machine operation requires administrator
privileges.

It is also recommended to install the modern "Windows Terminal," which
provides a superior user experience to the standard PowerShell and CMD
prompts, as well as a WSL prompt, should you want it.

You can install it by searching the Windows Store or by running the following
`winget` command:

`winget install Microsoft.WindowsTerminal`

Installing Podman
-----------------

Installing the Windows Podman client begins by downloading the Podman Windows
installer. The Windows installer is built with each Podman release and can be
downloaded from the official
[GitHub release page](https://github.com/containers/podman/releases).
Be sure to download a Podman 6.0 or later release for the capabilities discussed
in this guide.

The Windows installer is provided as an MSI package (e.g.,
`podman-installer-windows-arm64.msi`). The installer supports both user-scope
and machine-scope installations:

- **User scope (per-user)**: No administrator privileges required. Files are
  installed in the user's profile directory (`%LOCALAPPDATA%\Programs\Podman`),
  and the PATH is updated only for the current user. This is the default scope.

- **Machine scope (per-machine)**: Requires administrator privileges. Files are
  installed in `%PROGRAMFILES%\Podman`, and the PATH is updated for all users.

During installation, you can select the virtualization provider (WSL or Hyper-V)
that Podman will use for machines. The installer will create a configuration
file at `%APPDATA%\containers\containers.conf.d\99-podman-machine-provider.conf`
(for user scope) or `%PROGRAMDATA%\containers\containers.conf.d\99-podman-machine-provider.conf`
(for machine scope) with the selected provider.

![Installing Podman 6.0.0](podman-win-install.jpg)

Once installed, relaunch a new terminal. After this point, `podman.exe` will be
present on your PATH, and you will be able to run the `podman machine init`
command to create your first machine.

**Note:** WSLv2 or Hyper-V must be installed before creating Podman machines. If
WSL is not installed, you can install it manually by running `wsl --install`
from an administrator PowerShell prompt. The Podman installer no longer
automatically installs WSL. If the Hyper-V feature is not enabled, you can
enable it by running
`Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All` from
an administrator PowerShell prompt.

### Directories, files and registry keys used by Podman on Windows

The following tables list the directories, files and registry keys used by
Podman on Windows.

| Directory or file                                                            | Description                                |
|------------------------------------------------------------------------------|--------------------------------------------|
| `%LocalAppData%\Programs\Podman\`                                            | Installation directory                     |
| `%APPDATA%\containers\containers.conf.d\99-podman-machine-provider.conf`     | Installer created configuration file       |
| `%APPDATA%\containers\containers.conf`                                       | Client main configuration file             |
| `%APPDATA%\containers\podman-connections.json`                               | Client connections configuration file      |
| `%USERPROFILE%\.local\share\containers\podman\machine`                       | Machines data directory                    |
| `%USERPROFILE%\.config\containers\podman\machine\`                           | Machines configuration directory           |
| `%USERPROFILE%\.local\share\containers\storage\podman\`                      | Containers and images storage layers       |
| `%ProgramFiles%\Podman\`                                                     | Machine-scope installation directory       |
| `%ProgramData%\containers\containers.conf.d\99-podman-machine-provider.conf` | Machine-scope installer created conf file  |
| `%ProgramData%\containers\containers.conf`                                   | Machine-scope client configuration file    |

Table: Directories and files used by Podman on Windows

| Key                                                                                            | Description                                                                        |
|------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------|
| `HKCU:\SOFTWARE\Podman`                                                                        | Installation directory path                                                        |
| `HKLM:\SOFTWARE\Podman`                                                                        | Machine-scope Installation directory path                                          |
| `HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices` | Hyper-V socket registry entries (`{PORT_HEX}-FACB-11E6-BD58-64006A7986D3` pattern) |

Table: Registry keys used by Podman on Windows

Machine Init Process
--------------------

The `podman machine init` command will pull a custom Fedora OCI image (Fedora
CoreOS when using Hyper-V) as an OCI artifact from `quay.io/podman/machine-os`.
The image is customized to run Podman.

```powershell
PS C:\Users\User> podman machine init
Looking up Podman Machine image at quay.io/podman/machine-os:6.0 to create VM
Getting image source signatures
Copying blob 24c97bc42489 done   |
Copying config 44136fa355 done   |
Writing manifest to image destination
24c97bc424897b38d15d0e229b7a27487a1f8ed8ec8c019ccf2bb18add970db5
Extracting compressed file: podman-machine-default-arm64: done
Importing operating system into WSL (this may take a few minutes on a new WSL install)...
The operation completed successfully.
Configuring system...
Machine init complete
To start your machine run:

        podman machine start
```

You can also specify the virtualization provider when initializing a machine:

```powershell
PS C:\Users\User> podman machine init --provider wsl
```

or

```powershell
PS C:\Users\User> podman machine init --provider hyperv
```

**Note:** Hyper-V requires administrator privileges to initialize the first
podman machine. Similarly it requires administrator privileges to remove the
last machine. That's because these operations create and delete machine-scope
registry keys, required to support the communication between the guest OS and
the host. Additionally, all Hyper-V machine commands require that the current
user is a member of the Hyper-V Administrators group. To avoid these
requirements, run `podman system hyperv-prep` once from an administrator
terminal to pre-create persistent registry entries and add the current user
to the Hyper-V Administrators group, so that all subsequent machine operations
can run without administrator privileges.

Starting Machine
----------------

After the machine init process completes, it can then be started and stopped
as desired:

```powershell
PS C:\Users\User> podman machine start

Starting machine "podman-machine-default"

This machine is currently configured in rootless mode. If your containers
require root permissions (e.g. ports < 1024), or if you run into compatibility
issues with non-podman clients, you can switch using the following command:

        podman machine set --rootful

API forwarding listening on: npipe:////./pipe/docker_engine

Docker API clients default to this address. You do not need to set DOCKER_HOST.
Machine "podman-machine-default" started successfully
```

First Podman Command
--------------------

From this point on, podman commands operate similarly to how they would on
Linux.

For a quick working example with a small image, you can run the Linux date
command on PowerShell.

```powershell
PS C:\Users\User> podman run ubi9-micro date
Thu May 5 21:56:42 UTC 2022
```

If you get this error instead:

```
Cannot connect to Podman. Please verify your connection to the Linux system using `podman system connection list`, or try `podman machine init` and `podman machine start` to manage a new Linux VM
Error: unable to connect to Podman socket: Get "http://d/v5.7.1/libpod/_ping": ssh: rejected: connect failed (open failed)
```

try to restart the machine in rootful mode like this:

```
podman machine stop
podman machine set --rootful
podman machine start
podman run ubi8-micro date
```

Port Forwarding
---------------

Port forwarding also works as expected; ports will be bound against localhost
(127.0.0.1).

**Note:** When running as rootless (the default), you must use a port
greater than 1023. See the Rootful and Rootless section for more details.

To launch httpd, you can run:

```powershell
PS C:\Users\User> podman run --rm -d -p 8080:80 --name httpd docker.io/library/httpd
f708641300564a6caf90c145e64cd852e76f77f6a41699478bb83a162dceada9
```

A curl command against localhost on the PowerShell prompt will return a
successful HTTP response:

```powershell
PS C:\Users\User> Invoke-WebRequest -UseBasicParsing http://localhost:8080/

StatusCode        : 200
StatusDescription : OK
Content           : <html><body><h1>It works!</h1></body></html>
[...]
```

As with Linux, to stop, run:

`podman stop httpd`

Using API Forwarding
--------------------

API forwarding allows Docker API tools and clients to use Podman as if it was
Docker. Provided there is no other service listening on the Docker API pipe;
no special settings will be required.

```powershell
PS C:\Users\User> docker run -it fedora echo "Hello Podman!"
Hello Podman!
```

Otherwise, after starting the machine, you will be notified of an environment
variable you can set for tools to point to podman. Alternatively, you can shut
down both the conflicting service and podman, then finally run `podman machine
start` to restart, which should grab the Docker API address.

```powershell
Another process was listening on the default Docker API pipe address.
You can still connect Docker API clients by setting DOCKER HOST using the
following PowerShell command in your terminal session:

        $Env:DOCKER_HOST = 'npipe:////./pipe/podman-machine-default'

Or in a classic CMD prompt:

        set DOCKER_HOST=npipe:////./pipe/podman-machine-default

Alternatively, terminate the other process and restart podman machine.
Machine "podman-machine-default" started successfully

PS C:\Users\User> $Env:DOCKER_HOST = 'npipe:////./pipe/podman-machine-default'
PS C:\Users\User>.\docker.exe version --format '{{(index .Server.Components 0).Name}}'
Podman Engine
```

Rootful & Rootless
------------------

On the embedded guest environment, Podman can either be run under the root user
(rootful) or a non-privileged user (rootless). For behavioral consistency with
Podman on Linux, rootless is the default.

**Note:** Rootful and Rootless containers are distinct and isolated from one
another. Podman commands against one (e.g., podman ps) will not represent
results/state for the other.

While most containers run fine in a rootless setting, you may find a case
where the container only functions with root privileges. If this is the case,
you can switch the machine to rootful by stopping it and using the set
command:

```powershell
podman machine stop
podman machine set --rootful
```

To restore rootless execution, set rootful to false:

```powershell
podman machine stop
podman machine set --rootful=false
```

Another case in which you may wish to use rootful execution is binding a port
less than 1024. However, future versions of Podman will likely drop this to a
lower number to improve compatibility with defaults on system port services (such
as MySQL)

Configuring the Machine Provider
--------------------------------

Podman on Windows supports two virtualization providers: WSL and Hyper-V. The
provider can be configured in several ways:

1. **During installation**: The MSI installer allows you to select the provider
   during installation and creates a configuration file automatically.

2. **Via configuration file**: You can manually create or edit the configuration
   file at:

   - User scope: `%APPDATA%\containers\containers.conf`
   - Machine scope: `%PROGRAMDATA%\containers\containers.conf`

   Add the following content:

   ```toml
   [machine]
   provider = "wsl"
   ```

   or

   ```toml
   [machine]
   provider = "hyperv"
   ```

3. **Via environment variable**: Set `CONTAINERS_MACHINE_PROVIDER` to `wsl` or
   `hyperv`.

4. **Via command line**: Specify the provider when initializing a machine:

   ```powershell
   podman machine init --provider wsl
   ```

**Note:** WSL and Hyper-V machines cannot run simultaneously. You must stop
machines using one provider before starting machines with the other.

Volume Mounting
---------------

Podman supports volume mounts from Windows paths into Linux containers. This
supports several notation schemes, including:

Windows Style Paths:

`podman run --rm -v c:\Users\User\myfolder:/myfolder ubi9-micro ls /myfolder`

Unixy Windows Paths:

`podman run --rm -v /c/Users/User/myfolder:/myfolder ubi9-micro ls /myfolder`

Linux paths local to the WSL filesystem:

`podman run --rm -v /var/myfolder:/myfolder ubi9-micro ls /myfolder`

All of the above conventions work, whether running on a Windows prompt or the
WSL Linux shell. Although when using Windows paths on Linux, appropriately quote
or escape the Windows path portion of the argument.

Listing Podman Machine(s)
-------------------------

To list the available podman machine instances and their current resource
usage, use the `podman machine ls` command:

```powershell
PS C:\Users\User> podman machine ls


NAME                    VM TYPE     CREATED         LAST UP            CPUS        MEMORY      DISK SIZE
wsl-default             wsl         2 hours ago     Currently running  12          16G         768MB
hyperv-default*         hyperv      16 minutes ago  Never              6           2GiB        100GiB
```

Since WSL shares the same virtual machine and Linux kernel across multiple
distributions, the CPU and Memory values represent the total resources shared
across running systems. The opposite applies to the Disk value. It is
independent and represents the amount of storage for each individual
distribution.
The CPU, memory and disk size values for an Hyper-V machines instead, represent
the number of vCPUs, memory and disk size allocated to the machine. Those values
can be configured when creating the machine using the `--cpus`, `--memory` and
`--disk-size` options. Or edited later using the `podman machine set` command.

Accessing the Podman Linux Environment
--------------------------------------

While using the podman.exe client on the Windows environment provides a
seamless native experience supporting the usage of local desktop tools and
APIs, there are a few scenarios in which you may wish to access the Linux
environment:

- Updating to the latest stable packages on the embedded Fedora instance
- Using Linux development tools directly
- Using a workflow that relies on EXT4 filesystem performance or behavior
  semantics

There are three mechanisms to access the embedded WSL distribution:

1. SSH using `podman machine ssh`
2. WSL command on the Windows PowerShell prompt
3. Windows Terminal Integration

### Using SSH

SSH access provides a similar experience as Podman on Mac. It immediately
drops you into the appropriate user based on your machine's rootful/rootless
configuration (root in the former, 'user' in the latter). The --username
option can be used to override with a specific user.

An example task using SSH is updating your Linux environment to pull down the
latest OS bugfixes:

`podman machine ssh sudo dnf upgrade -y`

### Using the WSL Command

The `wsl` command provides direct access to the Linux system. Unless you have no
other distributions of WSL installed, it's recommended to use the `-d` option
with the name of your podman machine (podman-machine-default is the default):

```powershell
PS C:\Users\User> wsl -d podman-machine-default
```

You will be automatically entered into a nested process namespace where
systemd is running. If you need to access the parent namespace, hit `ctrl-d`
or type exit. This also means to log out, you need to exit twice.

```bash
[user@WINPC /]$ podman --version
podman version 6.0.0
```

To access commands that require root privileges, you can prefix the `wsl`
command with `sudo` (the default user is sudoer):

```bash
wsl -d podman-machine-default sudo systemctl status
```

Accessing the WSL instance as a specific user using `wsl -u` is not recommended
since commands will execute against the incorrect namespace.

### Using Windows Terminal Integration

Entering WSL is a 2-click operation. Simply click the drop-down tag, and pick
'podman-machine-default,' where you will be entered directly as the default
user.

![Using WSL in Windows Terminal](podman-wsl-term.jpg)

```powershell
[user@WINPC /]$ podman info --format '{{.Store.RunRoot}}'
/run/user/1000/containers
```

Stopping a Podman Machine
-------------------------

To stop a running podman machine, use the `podman machine stop` command:

```powershell
PS C:\Users\User> podman machine stop
Machine "podman-machine-default" stopped successfully
```

Removing a Podman Machine
-------------------------

To remove a machine, use the `podman machine rm` command:

```powershell
PS C:\Users\User> podman machine rm

The following files will be deleted:

C:\Users\User\.ssh\podman-machine-default
C:\Users\User\.ssh\podman-machine-default.pub
C:\Users\User\.local\share\containers\podman\machine\wsl\podman-machine-default_fedora-35-x86_64.tar
C:\Users\User\.config\containers\podman\machine\wsl\podman-machine-default.json
C:\Users\User\.local\share\containers\podman\machine\wsl\wsldist\podman-machine-default


Are you sure you want to continue? [y/N] y
```

Uninstalling Podman
-------------------

Podman can be uninstalled from the Windows Control Panel. Administrator
privileges are required if Podman was installed for the machine, rather than for
a user.

The uninstaller does not clean up Podman data and configuration resources. These
must be cleaned up manually.

Troubleshooting
---------------

### Installing WSL Manually

If WSL is not installed on your system, you must install it manually before
creating Podman machines. To install WSL:

1. Launch PowerShell as administrator

   ```powershell
   Start-Process powershell -Verb RunAs
   ```

2. Run the WSL install command

   ```powershell
   wsl --install
   ```

3. Reboot your system if prompted
4. After reboot, continue with `podman machine init`

If you encounter issues with WSL installation, you can attempt to reset your
WSL system state:

1. Launch PowerShell as administrator

   ```powershell
   Start-Process powershell -Verb RunAs
   ```

2. Disable WSL Features

   ```powershell
   dism.exe /online /disable-feature /featurename:Microsoft-Windows-Subsystem-Linux /norestart
   dism.exe /online /disable-feature /featurename:VirtualMachinePlatform /norestart
   ```

3. Reboot
4. Run manual WSL install

   ```powershell
   wsl --install
   ```

5. Continue with `podman machine init`

Install Certificate Authority
------------------------------

Instructions for installing a CA certificate can be found [in the dedicated
article](podman-install-certificate-authority.md).
