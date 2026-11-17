####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--ssh**=*default* | *id[=socket>*

SSH agent socket or keys to expose to the build.
The socket path can be left empty to use the value of `default=$SSH_AUTH_SOCK`

To later use the ssh agent, use the --mount option in a `RUN` instruction within a `Containerfile`:

`RUN --mount=type=ssh,id=id mycmd`
