# Change Request

<!--
This template is used to propose and discuss major new features to be added to Podman, Buildah, Skopeo, Netavark, and associated libraries.
The creation of a design document prior to feature implementation is not mandatory, but is encouraged.
Before major features are implemented, a pull request should be opened against the Podman repository with a completed version of this template.
Discussion on the feature will occur in the pull request.
Merging the pull request will constitute approval by project maintainers to proceed with implementation work.
When the feature is completed and merged, this document should be removed to avoid cluttering the repository.
It will remain in the Git history for future retrieval if necessary.
-->

## **Short Summary**

Unify (and rework) our config file parsing logic to make the various config files all behave
the same parsing wise so users can better understand how it works.

## **Objective**

We have several config files such a containers.conf, storage.conf, registries.conf and more that
get all implement their own parsing logic and have a different feature set. The goal is to
consolidate the parsing into a separate package and then port all files to use that package
instead making them behave consistently.

## **Detailed Description:**

### General

Add new package to the storage library (`go.podman.io/storage/pkg/configfile`) which implements
the core logic of how to read config files. The goal of the package is to define with config paths
to use and in what order.

It will however not define the structs and fields used for the actual content in each file, these
stay where they are defined currently and the plan is to have the code there call into the `configfile`
package to read the files in the same way.

#### Search locations:

Linux:

- `/usr/share/containers/<name>.conf`
- `/usr/share/containers/<name>.conf.d/`
- `/usr/share/containers/<name>.rootful.conf.d/` (only when UID == 0)
- `/usr/share/containers/<name>.rootless.conf.d/` (only when UID > 0)
- `/usr/share/containers/<name>.rootless.conf.d/<UID>/` (only when UID > 0)

- `/etc/containers/<name>.conf`
- `/etc/containers/<name>.conf.d/`
- `/etc/containers/<name>.rootful.conf.d/` (only when UID == 0)
- `/etc/containers/<name>.rootless.conf.d/` (only when UID > 0)
- `/etc/containers/<name>.rootless.conf.d/<UID>/` (only when UID > 0)

- `$XDG_CONFIG_HOME/containers/<name>.conf`
- `$XDG_CONFIG_HOME/containers/<name>.conf.d/`
  (if $XDG_CONFIG_HOME is empty then it uses $HOME/.config)
  This homedir lookup will also be done by root [1].

Where `<name>` is `containers`, `storage` or `registries` for each config file.

The `<name>.rootless.conf.d/<UID>/` is a directory named by the user id. Only the user with this
exact uid match will read the config files in this directory.
The use case is for admin to be able to set a default for a specific user without having to write
into their home directory. Note this is not intended as security mechanism, the user home directory
config files will still have higher priority.

FreeBSD:

Same as Linux except `/usr/share` is `/usr/local/share` and `/etc` is `/usr/local/etc`.

Windows:

There is no `/usr` equivalent, for `/etc` we instead lookup `ProgramData` env and use that one.
And instead of `XDG_CONFIG_HOME` which isn't used on windows we use `APPDATA`.

MacOS:

Same as Linux.

#### Load order

I propose adopting the UAPI config file specification for loading config files (version 1.0):
https://uapi-group.org/specifications/specs/configuration_files_specification/

Based on that the files must be loaded in this order:

Read `$XDG_CONFIG_HOME/containers/<name>.conf`, only if this file doesn't exists read
`/etc/containers/<name>.conf`, and if that doesn't exists read `/usr/share/containers/<name>.conf`
As such setting an empty file on `$XDG_CONFIG_HOME/containers/<name>.conf` would cause us to ignore
all possible options that were set in the other files.
Note: This is different from the current containers.conf loading where we would have read all files.

Regardless of which file has been loaded above it then must read the drop-in locations in the following order:

- `/usr/share/containers/<name>.conf.d/`
- `/usr/share/containers/<name>.rootful.conf.d/` (UID == 0)
- `/usr/share/containers/<name>.rootless.conf.d/` (UID > 0)
- `/usr/share/containers/containers.rootless.conf.d/<UID>/` (UID > 0)

- `/etc/containers/<name>.conf.d/`
- `/etc/containers/<name>.rootful.conf.d/` (UID == 0)
- `/etc/containers/<name>.rootless.conf.d/` (UID > 0)
- `/etc/containers/containers.rootless.conf.d/<UID>/` (UID > 0)

- `$XDG_CONFIG_HOME/containers/<name>.conf.d/`

Only read files with the `.conf` file extension are read.

If there is a drop-in file with the same filename as in a prior location it will replace
the prior one and only the latest match is read. Once we have the list of all drop-in files
they get sorted lexicographic. The later files have a higher priority so they can overwrite
options set in a prior file.

##### Example

Consider the following files:

`/usr/share/containers/containers.conf` (overridden by `/etc/containers/containers.conf`):
```
field_1 = a
```

`/etc/containers/containers.conf`:
```
field_2 = b
```

`/usr/share/containers/containers.conf.d/10-vendor.conf` (overridden by `$XDG_CONFIG_HOME/containers/containers.conf.d/10-vendor.conf`):
```
field_3 = c
```

`/usr/share/containers/containers.conf.d/99-important.conf`:
```
field_4 = d
```

`/usr/share/containers/containers.rootless.conf.d/50-my.conf`:
```
field_5 = e
```

`$XDG_CONFIG_HOME/containers/containers.conf.d/10-vendor.conf`:
```
# empty
```

`$XDG_CONFIG_HOME/containers/containers.conf.d/33-opt.conf` (this is read but field_4 is overridden by `/usr/share/containers/containers.conf.d/99-important.conf` as `99-important.conf` is sorted later):
```
field_4 = user
field_6 = f
```

Now parsing this as user with UID 1000 results in this final config:

```
field_2 = b
field_4 = d
field_5 = e
field_6 = f
```

#### Environment Variables

The following two envs should be defined for each config file:

`CONTAINERS_<name>_CONF`: If set only read the given file in this env and nothing else.
`CONTAINERS_<name>_CONF_OVERRIDE`: If set append the given file as last file after parsing
all other files as normal. Useful to overwrite a single field for testing without overwriting
the rest of the system configuration.

As special case for containers.conf the name of the vars is `CONTAINERS_CONF` and `CONTAINERS_CONF_OVERRIDE`.

The handling of these should be part of the `configfile` package.

#### Appending arrays

The toml parser by default replaces arrays in each file which makes it impossible to append values in drop-ins, etc...

containers.conf already has a workaround for that with a custom syntax to trigger appending:
```
field = ["val", {append=true}]
```
I propose we adapt the same universally for the other config files.

https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md#appending-to-string-arrays

This means moving the `common/internal/attributedstring` into the new configfile package so all callers can use it.

#### Scope

##### containers.conf

No changes except `/etc/containers/containers.rootless.conf` search location has been removed. It has just been added
in 5.7 so I don't think it would cause major concerns to drop it again.

The reason to not support the main file with rootless/rootful now is that it is not obvious how this should interact
with the parsing, should it replace the main config file or act like a drop in?  As such I think it is better to not
support this and we should generally push all users to use a drop instead of editing the main file.

Also there was/is some discussion of splitting containers.conf in two files as currently there are fields in there
that are only read on the server side while others only get used on the client side which makes using it in a remote
context such as podman machine confusing. For now this is not part of this design doc, we may make another design docs
just for this in case we like to move forward on it.

containers.conf also supports "Modules", i.e. `podman --module name.conf ...` which adds additional drop-in files at
the end after the regular config files. This functionality should be preserved but don't expand module support to the
other files.

##### storage.conf

Deprecate `rootless_storage_path` option. With the `rootless/rootful` config location and admin could just use
`graphroot` in the location in the rootless file location. As such there is no need to special case these fields
in the parser.

String arrays in the config will need to get switched to the attributedstring type, as described under Appending arrays.

Callers should only use `DefaultStoreOptions()` which will parse all files as described and returns the
final StoreOptions struct and cache it.

And remove many public APIs there such as `ReloadConfigurationFile()`, `UpdateStoreOptions()`, `Save()` and more.
I do not see a need for these in our tools podman/buildah/skopeo or even cri-o so let's just get rid of them to be
able to simplify the code over there. If external users really do need that we can consider re-adding them at a later
point.

##### registries.conf

Remove V1 config layout to simplify the parsing logic. If we do major config changes we might as well take the
chance and remove this old format.
Currently the V1 format is already rejected for drop-in files so this just effects the main config file.

Additionally there might be a few challenges here, c/image uses the SystemContext struct which allows
users to set `RootForImplicitAbsolutePaths`, `SystemRegistriesConfPath`, `SystemRegistriesConfDirPath`.
We must continue to support them as they are used by various tools.
For `RootForImplicitAbsolutePaths` we update it to check bot the `/usr` and `/etc` locations.
When `SystemRegistriesConfPath` or `SystemRegistriesConfDirPath` are used don't do the normal parsing
and just read the file/directory specified there and ignore the environment variables.

As described under the Environment Variables section the handling for `CONTAINERS_REGISTRIES_CONF` is moved
out of Podman, and common/libimage into the actual `configfile` package. As such all users of c/image will
be able to use this without having each caller specify their own env.
As part of this I propose removing support for the old `REGISTRIES_CONFIG_PATH` env which was never documented
and replaced by `CONTAINERS_REGISTRIES_CONF` in commit c9ef2607104a0b17e5146b3ee01852edb7d3d688 (over 4 years ago).
Currently there is an issue because Buildah doesn't support it [4].

String arrays in the config will need to get switched to the attributedstring type, as described under Appending arrays.
To avoid breaking public consumers of `V2RegistriesConf` we should use a new type instead and then copy values accordingly.
As part of this deprecate the `V2RegistriesConf` type and avoid expanding it to not expose so much "internal" details.

##### registries.d

Note registries.d is confusingly a completely different config file format from registries.conf.d.

Right now it only uses `$HOME/.config/containers/registries.d` or `/etc/containers/registries.d`

For consistency it would be best if it uses all drop in paths like shown above without the "main"
file as this only support drop-in locations.

Additionally there seems to exit code duplication in `podman/pkg/trust`. We should find a way to
not duplicate this logic at all in podman again.

##### certs.d

Same point as for registries.d. Make it support the new lookup locations. Note this isn't a
traditional config file but rather each entry is a directory with the registry name and then
contains certificates. So we should share code for the same lookup locations but there will
not be much code sharing otherwise for this.

There is also an open issue for proper XDG_CONFIG_HOME support [2] which should get fixed
as part of this.

##### policy.json

This is a json file so I don't think the normal drop-in logic would be particular useful here.
For consistency it should read also `/usr/share/containers/policy.json` if the /etc file doesn't
exists.

It is also missing the XDG_CONFIG_HOME support[3] so fix that as well.

We got an issue for drop-in support [5]. However I explicitly consider this out of scope for now
due the increased complexity and short time line for podman 6. Drop-in support could still be
added later as I don't consider that a breaking change, it only adds new functionality.

## Podman

`podman info` currently prints the storage.conf location in the output, given that with the new loading
this is no longer a single file it makes no sense to keep displaying a single file path. We never
printed the containers.conf path there either so just remove it.

Also get rid of the config reload functionality of the podman system service. There are various
problems with that:
It never actually worked with storage.conf, the store object is not recreated and cannot be made
safely so. The comments from Matt on the PR which added this remain unsolved:
https://github.com/containers/podman/pull/7311
Instead it directly writes into the storageConfig which is not race free (see below) but also means
the actual store object and storage config settings are out of sync which means we could run into all
sorts of unknown bugs because of that.

Then the code as it is not race free according to the go memory model. Reassigning the config struct
to the runtime is unsafe as there are concurrent reads. Running the service with the go race detector
shows this.
```
WARNING: DATA RACE
Write at 0x00c0003c2f00 by goroutine 38:
  github.com/containers/podman/v6/libpod.(*Runtime).reloadContainersConf()
      /home/pholzing/go/src/github.com/containers/podman/libpod/runtime.go:1072 +0x9c
  github.com/containers/podman/v6/libpod.(*Runtime).Reload()
      /home/pholzing/go/src/github.com/containers/podman/libpod/runtime.go:1054 +0x2e
  github.com/containers/podman/v6/pkg/domain/infra.StartWatcher.func1()
      /home/pholzing/go/src/github.com/containers/podman/pkg/domain/infra/runtime_libpod.go:302 +0x7a

Previous read at 0x00c0003c2f00 by goroutine 11323:
  github.com/containers/podman/v6/libpod.(*Runtime).GetConfigNoCopy()
      /home/pholzing/go/src/github.com/containers/podman/libpod/runtime.go:642 +0x304
```
The way to fix this would be to put this config access behind a mutex but this would mean a big code
change for IMO no real gain.

Lastly this feature has been proposed by another dev in https://github.com/containers/podman/issues/6255.
It is unclear to me if there is a single end user using this considering it does not work properly and we
never had any reports about this AFAIK. The podman system service is not a daemon so any user who want the
config files to be reloaded can just stop the service and start it again without causing downtime for
the running containers.

Overall not doing this greatly simplifies the code complexity.

## **Use cases**

A better way to configure podman and a better understanding for users how to do it without having
to worry that each config file behaves differently.
Since we then have only once place that defines the load order we can have a single man page
documenting the load order as described above. All the config files man pages can then just
refer to that and we don't have to duplicate so much docs.

By adding `/usr/share/containers` locations for all config files vendors can properly ship default
configurations there without causing package/user conflicts in `/etc` when admins also want to set
a default config.
This helps "Image Mode" or "Atomic" distributions which tend to prefer using /usr for configuration
when possible, i.e.
https://bootc-dev.github.io/bootc/building/guidance.html#configuration-in-usr-vs-etc
Given the increased importance of podman on such distributions it makes sense to support /usr configs
universally.

## **Target Podman Release**

6.0 (Because this is a breaking change a major release is required and this work must be finished in time)

## **Link(s)**


- [1] https://github.com/containers/podman/issues/27227
- [2] https://github.com/containers/container-libs/issues/183
- [3] https://github.com/containers/container-libs/issues/202
- [4] https://github.com/containers/buildah/issues/6468
- [5] https://github.com/containers/container-libs/issues/527
- https://github.com/containers/container-libs/issues/164
- https://github.com/containers/container-libs/issues/476

- https://github.com/containers/container-libs/issues/234


## **Stakeholders**


- [x] Podman Users
- [x] Podman Developers
- [x] Buildah Users
- [x] Buildah Developers
- [x] Skopeo Users
- [x] Skopeo Developers
- [x] Podman Desktop
- [x] CRI-O
- [x] Storage library
- [x] Image library
- [x] Common library
- [ ] Netavark and aardvark-dns

## ** Assignee(s) **

- Paul Holzinger (@Luap99)

## **Impacts**

### **CLI**

The cli should not change based on this.

### **Libpod**

No changes to libpod.

### **Others**

The major work will happen in the container-libs monorepo as this contains the file parsing logic for all.

## **Further Description (Optional):**

<!--
Is there anything not covered above that needs to be mentioned?
-->

## **Test Descriptions (Optional):**

The code should be designed in a way to be unit testable and that they get parsed in the right order.
