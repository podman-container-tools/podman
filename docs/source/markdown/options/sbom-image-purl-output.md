####> This option file is used in:
####>   podman farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--sbom-image-purl-output**=*path*

When generating SBOMs, scan them for PURL ([package
URL](https://github.com/package-url/purl-spec/blob/master/PURL-SPECIFICATION.rst))
information, and save a list of found PURLs to the specified path in the output
image.  There is no default.
