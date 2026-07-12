####> This option file is used in:
####>   podman farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--sbom-merge-strategy**=*method*

If more than one **--sbom-scanner-command** value is being used, use the
specified method to merge the output from later commands with output from
earlier commands.  Recognized values include:

 - cat
     Concatenate the files.
 - merge-cyclonedx-by-component-name-and-version
     Merge the "component" fields of JSON documents, ignoring values from
     documents when the combination of their "name" and "version" values is
     already present.  Documents are processed in the order in which they are
     generated, which is the order in which the commands that generate them
     were specified.
 - merge-spdx-by-package-name-and-versioninfo
     Merge the "package" fields of JSON documents, ignoring values from
     documents when the combination of their "name" and "versionInfo" values is
     already present.  Documents are processed in the order in which they are
     generated, which is the order in which the commands that generate them
     were specified.
