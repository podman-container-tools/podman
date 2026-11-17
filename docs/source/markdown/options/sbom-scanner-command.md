####> This option file is used in:
####>   podman farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--sbom-scanner-command**=*image*

Generate SBOMs by running the specified command from the scanner image.  If
multiple commands are specified, they are run in the order in which they are
specified.  These text substitutions are performed:
  - {ROOTFS}
      The root of the built image's filesystem, bind mounted.
  - {CONTEXT}
      The build context and additional build contexts, bind mounted.
  - {OUTPUT}
      The name of a temporary output file, to be read and merged with others or copied elsewhere.
