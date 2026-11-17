SHELL := bash
GO ?= go

ARCH ?= aarch64
ifeq ($(ARCH), aarch64)
	GOARCH:=arm64
else
	GOARCH:=$(ARCH)
endif
GVPROXY_VERSION=$(shell $(GO) list -m -f '{{.Version}}' github.com/containers/gvisor-tap-vsock)
VFKIT_VERSION ?= 0.6.4
KRUNKIT_VERSION ?= 1.3.2
GVPROXY_RELEASE_URL ?= https://github.com/containers/gvisor-tap-vsock/releases/download/$(GVPROXY_VERSION)/gvproxy-darwin
VFKIT_RELEASE_URL ?= https://github.com/crc-org/vfkit/releases/download/v$(VFKIT_VERSION)/vfkit-unsigned
KRUNKIT_RELEASE_URL ?= https://github.com/containers/krunkit/releases/download/v$(KRUNKIT_VERSION)/krunkit-podman-unsigned-$(KRUNKIT_VERSION).tgz
PACKAGE_DIR ?= out/packaging
TMP_BIN ?= tmp-bin
PACKAGE_ROOT ?= root
PKG_NAME := podman-installer-macos-$(GOARCH).pkg

default: pkginstaller

podman_version:
	make -B -C ../../ test/version/version

$(TMP_BIN)/gvproxy:
	mkdir -p $(TMP_BIN)
	cd $(TMP_BIN) && curl --fail -sLo gvproxy $(GVPROXY_RELEASE_URL)

$(TMP_BIN)/vfkit:
	mkdir -p $(TMP_BIN)
	cd $(TMP_BIN) && curl --fail -sLo vfkit $(VFKIT_RELEASE_URL)

$(TMP_BIN)/krunkit.tgz:
	mkdir -p $(TMP_BIN)
	cd $(TMP_BIN) && curl --fail -sLo krunkit.tgz $(KRUNKIT_RELEASE_URL)

packagedir: podman_version package_root Distribution welcome.html
	mkdir -p $(PACKAGE_DIR)
	cp -r Resources $(PACKAGE_DIR)/
	cp welcome.html $(PACKAGE_DIR)/Resources/
	cp Distribution $(PACKAGE_DIR)/
	cp -r scripts $(PACKAGE_DIR)/
	cp -r $(PACKAGE_ROOT) $(PACKAGE_DIR)/
	cp package.sh $(PACKAGE_DIR)/
	cd $(PACKAGE_DIR) && pkgbuild --analyze --root ./root component.plist
	../../test/version/version > $(PACKAGE_DIR)/VERSION
	echo -n $(ARCH) > $(PACKAGE_DIR)/ARCH
	cp ../../LICENSE $(PACKAGE_DIR)/Resources/LICENSE.txt
	cp vfkit.entitlements $(PACKAGE_DIR)/
	cp krunkit.entitlements $(PACKAGE_DIR)/

package_root: clean-pkgroot $(TMP_BIN)/gvproxy $(TMP_BIN)/vfkit $(TMP_BIN)/krunkit.tgz
	mkdir -p $(PACKAGE_ROOT)/podman/bin
	cp $(TMP_BIN)/gvproxy $(PACKAGE_ROOT)/podman/bin/
	cp $(TMP_BIN)/vfkit $(PACKAGE_ROOT)/podman/bin/
	tar xf $(TMP_BIN)/krunkit.tgz -C $(PACKAGE_ROOT)/podman
	chmod a+x $(PACKAGE_ROOT)/podman/bin/*
	# Leaving for future considerations
	# mkdir $(PACKAGE_ROOT)/podman/config
	# cp ../../pkg/machine/ocipull/policy.json $(PACKAGE_ROOT)/podman/config/policy.json

%: %.in podman_version
	@sed -e 's/__VERSION__/'$(shell ../../test/version/version)'/g' $< >$@

pkginstaller: packagedir
	cd $(PACKAGE_DIR) && ./package.sh ..

_notarize: pkginstaller
	xcrun notarytool submit --apple-id $(NOTARIZE_USERNAME) --password $(NOTARIZE_PASSWORD) --team-id=$(NOTARIZE_TEAM) -f json --wait out/$(PKG_NAME)

notarize: _notarize
	xcrun stapler staple out/$(PKG_NAME)

.PHONY: clean clean-pkgroot
clean:
	rm -rf $(TMP_BIN) $(PACKAGE_ROOT) $(PACKAGE_DIR) out Distribution welcome.html ../../test/version/version

clean-pkgroot:
	rm -rf $(PACKAGE_ROOT) $(PACKAGE_DIR) Distribution welcome.html
