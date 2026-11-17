## How to build

```sh
$ make ARCH=aarch64 NO_CODESIGN=1 pkginstaller

# or to create signed pkg
$ make ARCH=aarch64 CODESIGN_IDENTITY=<ID> PRODUCTSIGN_IDENTITY=<ID> pkginstaller

# or to prepare a signed and notarized pkg for release
$ make ARCH=aarch64 CODESIGN_IDENTITY=<ID> PRODUCTSIGN_IDENTITY=<ID> NOTARIZE_USERNAME=<appleID> NOTARIZE_PASSWORD=<appleID-password> NOTARIZE_TEAM=<team-id> notarize
```

The generated pkg will be written to `out/podman-macos-installer-*.pkg`.
Currently the pkg installs `podman`, `vfkit`, `gvproxy` and `podman-mac-helper` to `/opt/podman`

## Uninstalling

```sh
$ sudo rm -rf /opt/podman
```

### Screenshot
<img width="626" alt="screenshot-macOS-pkg-podman" src="https://user-images.githubusercontent.com/8885742/157380992-2e3b1573-34a0-4aa0-bdc1-a85f4792a1d2.png">
