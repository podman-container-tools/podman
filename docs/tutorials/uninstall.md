# Complete Uninstall & Cleanup Guide

This guide details how to reset Podman's state, uninstall Podman using supported package managers and official installers, and clean up documented leftover configuration and storage directories across Linux, macOS, and Windows.

---

## Resetting Podman State

If you want to start fresh, reclaim disk space, or resolve configuration/storage issues without removing the Podman application, use Podman's built-in reset commands.

### Local Container Storage (`podman system reset`)

`podman system reset` stops and removes all containers, pods, images, volumes, networks, and build caches, and resets container storage.

For **rootless** users:
```bash
podman system reset --force
```

For **rootful** (system-wide) containers on Linux:
```bash
sudo podman system reset --force
```

### Podman Machines (`podman machine reset`)

On macOS and Windows (or systems using `podman machine`), reset all virtual machines and their disk images:

```bash
podman machine reset -f
```

---

## Uninstalling Podman

Binaries and packages should always be removed using the official package manager or uninstaller for your operating system, rather than manually deleting binary files.

### Linux

Use your distribution's package manager to remove Podman:

#### Fedora / RHEL / CentOS Stream
```bash
sudo dnf remove podman
```

#### Debian / Ubuntu
```bash
sudo apt-get purge podman
sudo apt-get autoremove
```

### macOS

If you installed Podman using the official `.pkg` installer or Podman Desktop, use the uninstaller provided by the application package or move the application to the Trash.

### Windows

Uninstall Podman using standard Windows application management:

1. Open **Settings** &rarr; **Apps** &rarr; **Installed apps** (or **Apps & features**).
2. Search for **Podman** and select **Uninstall**.

Alternatively, open **Control Panel** &rarr; **Programs and Features**, select **Podman**, and click **Uninstall**.

If a Podman machine was installed into WSL and not removed via `podman machine reset -f`, unregister it:
```powershell
wsl --unregister podman-machine-default
```

---

## Optional Cleanup

Official uninstallers and package managers typically retain user configuration and data directories to prevent accidental data loss. If you want to perform a complete cleanup of documented configuration and storage paths:

### Linux

#### Rootless User Paths
- Configuration files: `~/.config/containers`
- Container storage and image layers: `~/.local/share/containers`

```bash
rm -rf ~/.config/containers
rm -rf ~/.local/share/containers
```

#### Rootful System Paths
- System-wide configuration: `/etc/containers`
- System-wide container storage: `/var/lib/containers`

```bash
sudo rm -rf /etc/containers
sudo rm -rf /var/lib/containers
```

### macOS

- Client configuration: `~/.config/containers`
- Machine data and cache: `~/.local/share/containers`

```bash
rm -rf ~/.config/containers
rm -rf ~/.local/share/containers
```

### Windows

In PowerShell, remove the documented user configuration and data directories:

```powershell
Remove-Item -Recurse -Force "$env:APPDATA\containers" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\containers" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:USERPROFILE\.config\containers" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:USERPROFILE\.local\share\containers" -ErrorAction SilentlyContinue
```
