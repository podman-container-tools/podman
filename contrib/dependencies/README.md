# Dependency analysis

## Requirements

The script assumes you have goda installed and in $PATH. You can install it with
```
go install github.com/loov/goda@latest
```

Check https://github.com/loov/goda for more details.

## Basic Usage

The dependencies.sh script is mainly a small wrapper script around goda to make certain operations easier to use as the goda expression syntax is quite special.

### List all the packages used by podman
```
$ ./contrib/dependencies/dependencies.sh list
```
If you like to use another command, i.e. podman-remote or quadlet the script supports the `--command`/`-c` option:
```
$ ./contrib/dependencies/dependencies.sh -c quadlet list
```

### Show dependency tree
```
$ ./contrib/dependencies/dependencies.sh tree
```
Note the output is general not very readable so it is unlikely that it helps much.

### Show what imports a specific package

To know what import a given package use

```
$ ./contrib/dependencies/dependencies.sh why ./pkg/ps/
github.com/containers/podman/v5/pkg/api/handlers/compat
github.com/containers/podman/v5/pkg/domain/infra/abi
github.com/containers/podman/v5/pkg/ps
```

This is useful to know from where the package is being used.

## Bloat check analysis

Golang binaries are big, often a single import can cause a large increase in binary size.
This section describes how to find such imports.

First, if you hit the bloat check on a PR it may be useful to run a diff between two branches.
The diff subcommand runs the list command on two branches and then shows you the diff. This is
useful to see all the new imports that might cause the bloat. It may be easy to spot something
that should not get dragged in.

```
$ ./contrib/dependencies/dependencies.sh diff main pr/26577
Switched to branch 'main'
Your branch is ahead of 'origin/master' by 17550 commits.
  (use "git push" to publish your local commits)
Switched to branch 'pr/26577'
285d284
< github.com/containers/podman/v5/cmd/podman/quadlet
318a318
> github.com/containers/podman/v5/pkg/bindings/artifacts
362d361
< github.com/containers/podman/v5/pkg/logiface
404d402
< github.com/containers/podman/v5/pkg/systemd/quadlet
```

If the diff from the imports is not clear there is the experimental
weight-diff command.

```
$ ./contrib/dependencies/dependencies.sh weight-diff main pr/26577
...
name                                                                                      diff     bin/podman.1    bin/podman
github.com/containers/podman/v5/pkg/domain/infra/abi.(*ContainerEngine).QuadletRemove     11025    11025           -
github.com/containers/podman/v5/pkg/domain/infra/abi.(*ContainerEngine).QuadletInstall    6233     6233            -
github.com/containers/podman/v5/pkg/bindings/images.Build                                 5760     38418           32658
github.com/containers/buildah.(*Builder).createMountTargets                               5684     5684            -
github.com/containers/podman/v5/pkg/domain/infra/abi.(*ContainerEngine).QuadletList       5485     5485            -
github.com/containers/buildah.(*Builder).Add                                              5231     21172           15941
github.com/containers/podman/v5/pkg/api/handlers/compat.handleBuildContexts               4254     4254            -
github.com/containers/podman/v5/pkg/bindings/artifacts.Extract                            2958     -               2958
github.com/containers/podman/v5/pkg/bindings/images.Build.func2                           2799     2799            -
github.com/containers/podman/v5/pkg/domain/infra/abi.(*ContainerEngine).installQuadlet    2743     2743            -
github.com/containers/buildah.(*Builder).makeContainerImageRef                            2208     8293            6085
github.com/containers/buildah.(*Builder).setupMounts                                      2087     4869            6956
github.com/containers/podman/v5/cmd/podman/quadlet.listFlags.AutocompleteFormat.func1     2074     2074            -
init                                                                                      2011     2016            5
github.com/containers/podman/v5/cmd/podman/quadlet.rm                                     1342     1342            -
github.com/containers/podman/v5/pkg/systemd/quadlet.AppendSubPaths                        1285     1285            -
github.com/containers/podman/v5/pkg/systemd/parser.(*UnitFile).Parse                      1263     1263            -
github.com/containers/buildah/imagebuildah.(*StageExecutor).intermediateImageExists       1248     3829            2581
github.com/containers/podman/v5/pkg/systemd/parser.(*UnitFileParser).parseGroup           1241     1241            -
github.com/containers/podman/v5/pkg/systemd/parser.(*UnitFileParser).parseKeyValuePair    1241     1241            -
github.com/containers/podman/v5/pkg/bindings/artifacts.Pull                               1221     -               1221
github.com/containers/podman/v5/pkg/bindings/artifacts.Add                                1209     -               1209
github.com/containers/podman/v5/pkg/systemd/quadlet.getRootlessDirs                       1199     1199            -
github.com/containers/podman/v5/pkg/domain/infra/tunnel.(*ImageEngine).ArtifactAdd        1198     59              1257
github.com/containers/podman/v5/pkg/domain/infra/abi.getAllQuadletPaths                   1189     1189            -
github.com/containers/podman/v5/cmd/podman/quadlet.outputTemplate                         1189     1189            -
github.com/containers/podman/v5/pkg/bindings/artifacts.Push                               1076     -               1076
```

That gives insight into the symbol sizes. There is likely not much that can be done to avoid it
though unless you manage to de-duplicate a lot of code somehow. Or avoid some of the new imports/symbols.

The `diff` and `weight-diff` commands accept two git references as argument to diff between,
i.e. branch names, git tags, or commit sha's should all work. Basically anything that git checkout accepts.

## Debloat exercise

When actively working to reduce binary size it is easiest to look into big dependencies that have not
many users which can be replaced with something else or maybe are not needed at all.

To find such dependencies use the cut command
```
$ ./contrib/dependencies/dependencies.sh cut
ID                                                                                   InDegree   Cut.PackageCount   Cut.AllFiles.Size   Cut.Go.Lines
github.com/containers/podman/v5/cmd/podman                                           0          905                42.5MB              964250
vendor/golang.org/x/crypto/internal/poly1305                                         0          1                  16.4KB              395
vendor/golang.org/x/net/http2/hpack                                                  0          1                  43.9KB              1471
...
github.com/containers/ocicrypt/keywrap/keyprovider                                   1          75                 1.9MB               56453
github.com/docker/docker/client                                                      1          37                 1.5MB               39437
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp                        1          22                 0.7MB               19647
github.com/containers/buildah/imagebuildah                                           1          14                 1.2MB               36431
github.com/containers/image/v5/pkg/cli/sigstore                                      1          14                 229.8KB             6343
github.com/containers/storage/drivers/register                                       1          12                 229.0KB             7109
go/types                                                                             1          12                 1.0MB               31977
```

Here you see a list of all imported packages:
 - InDegree means how many times it is imported.
 - Cut.PackageCount shows how many packages (including transitive imports) are getting drop by removing this package.
 - Cut.AllFiles.Size and Cut.Go.Lines show the info about the go source files of that package and its transitive imports.
   Note this is not the actual binary size. File size/Line count is not directly related to the resulting binary size.
   It is likely that if we only use a single function out of a big package we do not bloat the binary size by much.

Look for packages with a low InDegree count and a high PackageCount for best gains. A low important count makes it of
course easier to remove the dep somehow. A high PackageCount count means the binary size gain will likely be a lot.

Of course there is a log of basic functionality that we can just remove/replace so it requires best judgement to
actually find packages that we can remove.

A special case are the `vendor/golang.org/x/...` packages, these are actually vendored by the standard library directly
and not shared with our `golang.org/x/...` imports. As such they cause bloat but since they are used by the std lib we
cannot really get rid of them easily or change them as such we have to simple accept them.

The `go/types` package is a good example of finding something to be cut. It is only used once for one small function so
we can replace it easily. That is done in https://github.com/containers/buildah/pull/6253.

Because file size != binary size it may be worth to look into the actual symbol size of each package first before deciding
if we really gain much by removing it to spend

This is done with the weight command:
```
$ ./contrib/dependencies/dependencies.sh weight
   1213750    1125084 /github.com/containers/podman/v5/libpod [syms 1699]
                28143 t (*Container).generateSpec
                18790 t (*Runtime).setupContainer
...
    476690     476690 /go/types [syms 1100]
                21630 t (*Checker).builtin
...
```

The output lists each packages total size and then the individual symbol sizes sorted by the biggest packages first.

Here we can see is that `go/types` is recorded with `476690` bytes so it is a good candidate to cut.

Overall there is no silver bullet to this, it mostly relies on experience which parts of the code would be good
candidates for removal/reworks to reduce the number of dependencies.
