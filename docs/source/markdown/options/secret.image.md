####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Secret=id=id[,src=envOrFile][,env=ENV][,type=file | env]`
<< else >>
#### **--secret**=**id=id[,src=*envOrFile*][,env=*ENV*][,type=*file* | *env*]**
<< endif >>

Pass secret information to be used in the Containerfile for building images
in a safe way that will not end up stored in the final image, or be seen in other stages.
The value of the secret will be read from an environment variable or file named
by the "id" option, or named by the "src" option if it is specified, or from an
environment variable specified by the "env" option. See [EXAMPLES](#examples).
The secret will be mounted in the container at `/run/secrets/id` by default.

To later use the secret, use the --mount flag in a `RUN` instruction within a `Containerfile`:

`RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret`

The location of the secret in the container can be overridden using the
"target", "dst", or "destination" option of the `RUN --mount` flag.

`RUN --mount=type=secret,id=mysecret,target=/run/secrets/myothersecret cat /run/secrets/myothersecret`

Note: changing the contents of secret files will not trigger a rebuild of layers that use said secrets.
