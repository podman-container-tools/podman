% podman-machine-os-upgrade 1

## NAME
podman\-machine\-os\-upgrade - Upgrade a Podman Machine's OS

## SYNOPSIS
**podman machine os upgrade** [*options*] [vm]

## DESCRIPTION

Upgrade the OS of a Podman Machine

Automatically perform an upgrade of the Podman Machine OS according to the logic below.
To apply a custom image or an image with a specific digest, use **podman machine os apply** instead.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then the OS changes will be applied to `podman-machine-default`.

The machine must be started for this command to be run.

Note: This command is not supported with the WSL machine provider.

### UPGRADE LOGIC

The upgrade function compares the client version against the machine version using semantic versioning (major.minor
only, ignoring patch levels). When versions match, it also queries the online registry to check for image updates:

**Client version older than machine version (downgrade):**
Returns an error. Downgrading is not supported to prevent incompatibilities. Update your Podman client to match or exceed the machine version.

**Client version equals machine version (in-band update):**
Checks for updates to the same version stream by comparing the local OS image digest against the registry's digest. This handles two scenarios:
- Patch version updates (e.g., 6.0.1 → 6.0.2)
- OS image refreshes with the same Podman version (e.g., 6.0.1 → newer OS build with 6.0.1)

If an in-band update exists, the machine is updated to the new image using its digest reference. If no update is available, the function reports that the system is current.

**Client version newer than machine version (major/minor upgrade):**
Upgrades the machine OS to match the client's major.minor version. The upgrade targets a new version tag constructed from the client's major and minor version numbers (e.g., upgrading from 6.0.x to 6.1 when the client is version 6.1.0).

## OPTIONS

#### **--dry-run**, **-n**

Only perform a dry-run of checking for the upgrade.  No content is downloaded or applied. This option
cannot be used with --restart

#### **--format**, **-f**

Define a structured output format.  The only valid value for this is `json`.  Using this option
imples a dry-run. This option cannot be used with --restart.

#### **--help**

Print usage statement.

#### **--restart**

Restart VM after applying changes.


## EXAMPLES

Update the default Podman machine to the latest in-band version or update the machine to
match the client Podman version.
```
$ podman machine os upgrade
```

Same as above but specifying a specific machine.

```
$ podman machine os upgrade mymachine
```

Check if an update is available but do not download or apply the upgrade.
```
$ podman machine os upgrade -n
```

Check if an update is available but the response will be in JSON format.  Note this
exposes a boolean field that indicates if an update is available.

Note: using a format option implies a dry-run.
```
$ podman machine os upgrade -f json
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**, **[podman-machine-os(1)](podman-machine-os.1.md)**

## HISTORY
January 2026, Originally compiled by Brent Baude <bbaude@redhat.com>
