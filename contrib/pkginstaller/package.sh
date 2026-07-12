#!/bin/bash

set -euxo pipefail

BASEDIR=$(dirname "$0")
OUTPUT=$1
CODESIGN_IDENTITY=${CODESIGN_IDENTITY:--}
PRODUCTSIGN_IDENTITY=${PRODUCTSIGN_IDENTITY:-mock}
NO_CODESIGN=${NO_CODESIGN:-0}
HELPER_BINARIES_DIR="/opt/podman/bin"
MACHINE_POLICY_JSON_DIR="/opt/podman/config"
BUILD_ORIGIN="pkginstaller"

tmpBin="contrib/pkginstaller/tmp-bin"

binDir="${BASEDIR}/root/podman/bin"
libDir="${BASEDIR}/root/podman/lib"
docDir="${BASEDIR}/root/podman/docs/man/man1"

version=$(cat "${BASEDIR}/VERSION")
arch=$(cat "${BASEDIR}/ARCH")

function build_podman() {
  pushd "$1"

  make podman-remote-darwin-docs
  mkdir -p "contrib/pkginstaller/out/packaging/${docDir}"
  cp -v docs/build/remote/darwin/*.1 "contrib/pkginstaller/out/packaging/${docDir}"

  case ${goArch} in
  arm64)
    build_podman_arch ${goArch}
    cp "${tmpBin}/podman-${goArch}"  "contrib/pkginstaller/out/packaging/${binDir}/podman"
    cp "${tmpBin}/podman-mac-helper-${goArch}" "contrib/pkginstaller/out/packaging/${binDir}/podman-mac-helper"
    ;;
  *)
    echo -n "Unknown arch: ${goArch}"
    ;;
  esac

  popd
}

function build_podman_arch(){
    make -B GOARCH="$1" podman-remote HELPER_BINARIES_DIR="${HELPER_BINARIES_DIR}" BUILD_ORIGIN="${BUILD_ORIGIN}"
    make -B GOARCH="$1" podman-mac-helper
    mkdir -p "${tmpBin}"
    cp bin/darwin/podman "${tmpBin}/podman-$1"
    cp bin/darwin/podman-mac-helper "${tmpBin}/podman-mac-helper-$1"
}

function sign() {
  local opts=""
  entitlements="${BASEDIR}/$(basename "$1").entitlements"
  if [ -f "${entitlements}" ]; then
      opts="--entitlements ${entitlements}"
  fi
  if [ ! "${NO_CODESIGN}" -eq "1" ]; then
      opts="$opts --options runtime"
  fi
  codesign --deep --sign "${CODESIGN_IDENTITY}" --timestamp --force ${opts} "$1"
}

goArch="${arch}"
if [ "${goArch}" = aarch64 ]; then
  goArch=arm64
fi

build_podman "../../../../"

sign "${binDir}/podman"
sign "${binDir}/gvproxy"
sign "${binDir}/vfkit"
sign "${binDir}/podman-mac-helper"

sign "${binDir}/krunkit"
sign "${libDir}/libkrun.dylib"
sign "${libDir}/libvirglrenderer.1.dylib"
sign "${libDir}/libepoxy.0.dylib"
sign "${libDir}/libMoltenVK.dylib"

pkgbuild --identifier com.redhat.podman --version "${version}" \
  --scripts "${BASEDIR}/scripts" \
  --root "${BASEDIR}/root" \
  --install-location /opt \
  --component-plist "${BASEDIR}/component.plist" \
  "${OUTPUT}/podman.pkg"

productbuild --distribution "${BASEDIR}/Distribution" \
  --resources "${BASEDIR}/Resources" \
  --package-path "${OUTPUT}" \
  "${OUTPUT}/podman-unsigned.pkg"
rm "${OUTPUT}/podman.pkg"

if [ ! "${NO_CODESIGN}" -eq "1" ]; then
  productsign --timestamp --sign "${PRODUCTSIGN_IDENTITY}" "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
else
  mv "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
fi
