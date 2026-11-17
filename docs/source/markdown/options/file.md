####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `File=Containerfile`
<< else >>
#### **--file**, **-f**=*Containerfile*
<< endif >>


Specifies a Containerfile which contains instructions for building the image,
either a local file or an **http** or **https** URL.  If more than one
Containerfile is specified, *FROM* instructions are only accepted from the
last specified file.

<< if is_quadlet >>
Note that for a given relative path to a Containerfile, or when using a `http(s)://` URL, also set
`SetWorkingDirectory=` in order for `podman build` to find a valid context directory for the
resources specified in the Containerfile.

Note that setting a `File=` field is mandatory for a `.build` file, unless `SetWorkingDirectory` (or
a `WorkingDirectory` in the `Service` group) has also been set.
<< else >>
If a build context is not specified, and at least one Containerfile is a
local file, the directory in which it resides is used as the build
context.
<< endif >>

Specifying the option << 'File=-' if is_quadlet else '`-f -`' >> causes
the Containerfile contents to be read from stdin.
