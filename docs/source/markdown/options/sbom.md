####> This option file is used in:
####>   podman farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--sbom**=*preset*

Generate SBOMs (Software Bills Of Materials) for the output image by scanning
the working container and build contexts using the named combination of scanner
image, scanner commands, and merge strategy.  Must be specified with one or
more of **--sbom-image-output**, **--sbom-image-purl-output**, **--sbom-output**,
and **--sbom-purl-output**.  Recognized presets, and the set of options which
they equate to:

 - "syft", "syft-cyclonedx":
     --sbom-scanner-image=ghcr.io/anchore/syft
     --sbom-scanner-command="/syft scan -q dir:{ROOTFS} --output cyclonedx-json={OUTPUT}"
     --sbom-scanner-command="/syft scan -q dir:{CONTEXT} --output cyclonedx-json={OUTPUT}"
     --sbom-merge-strategy=merge-cyclonedx-by-component-name-and-version
 - "syft-spdx":
     --sbom-scanner-image=ghcr.io/anchore/syft
     --sbom-scanner-command="/syft scan -q dir:{ROOTFS} --output spdx-json={OUTPUT}"
     --sbom-scanner-command="/syft scan -q dir:{CONTEXT} --output spdx-json={OUTPUT}"
     --sbom-merge-strategy=merge-spdx-by-package-name-and-versioninfo
 - "trivy", "trivy-cyclonedx":
     --sbom-scanner-image=ghcr.io/aquasecurity/trivy
     --sbom-scanner-command="trivy filesystem -q {ROOTFS} --format cyclonedx --output {OUTPUT}"
     --sbom-scanner-command="trivy filesystem -q {CONTEXT} --format cyclonedx --output {OUTPUT}"
     --sbom-merge-strategy=merge-cyclonedx-by-component-name-and-version
 - "trivy-spdx":
     --sbom-scanner-image=ghcr.io/aquasecurity/trivy
     --sbom-scanner-command="trivy filesystem -q {ROOTFS} --format spdx-json --output {OUTPUT}"
     --sbom-scanner-command="trivy filesystem -q {CONTEXT} --format spdx-json --output {OUTPUT}"
     --sbom-merge-strategy=merge-spdx-by-package-name-and-versioninfo
