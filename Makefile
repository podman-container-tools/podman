###
### Makefile Navigation
###
#
# This file is organized based on approximate end-to-end workflow:
#
# 1.  Variables and common definitions are located at the top
#     to make finding them quicker.
# 2.  Main entry-point targets, like "default", "all", and "help"
# 3.  Targets for code formatting and validation
# 4.  Primary build targets, like podman and podman-remote
# 5.  Secondary build targets, shell completions, static and multi-arch.
# 6.  Targets that format and build documentation
# 7.  Testing targets
# 8.  Release and package-building targets
# 9.  Targets that install tools, utilities, binaries and packages
# 10. Uninstall / Cleanup targets
#
###
### Variables & Definitions
###

# Default shell `/bin/sh` has different meanings depending on the platform.
SHELL := $(shell command -v bash;)
GO ?= go
GO_LDFLAGS:= $(shell if $(GO) version|grep -q gccgo ; then echo "-gccgoflags"; else echo "-ldflags"; fi)
GOCMD = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO)
# Podman does not work w/o CGO_ENABLED, except in some very specific cases.
# Windows and Mac (both podman-remote client only) require CGO_ENABLED=0.
CGO_ENABLED ?= 1
# Default to the native OS type and architecture unless otherwise specified
NATIVE_GOOS := $(shell env -u GOOS $(GO) env GOOS)
GOOS ?= $(call err_if_empty,NATIVE_GOOS)
# Default to the native architecture type
NATIVE_GOARCH := $(shell env -u GOARCH $(GO) env GOARCH)
GOARCH ?= $(NATIVE_GOARCH)
COVERAGE_PATH ?= .coverage
DESTDIR ?=
EPOCH_TEST_COMMIT ?= $(shell git merge-base $${DEST_BRANCH:-main} HEAD)
HEAD ?= HEAD
PROJECT := go.podman.io/podman
GIT_BASE_BRANCH ?= origin/main
LIBPOD_INSTANCE := libpod_dev
PREFIX ?= /usr/local
RELEASE_PREFIX = /usr
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
LIBEXECPODMAN ?= ${LIBEXECDIR}/podman
MANDIR ?= ${PREFIX}/share/man
SHAREDIR_CONTAINERS ?= ${PREFIX}/share/containers
ETCDIR ?= /etc
LIBDIR ?= ${PREFIX}/lib
TMPFILESDIR ?= ${LIBDIR}/tmpfiles.d
USERTMPFILESDIR ?= ${PREFIX}/share/user-tmpfiles.d
SYSTEMDDIR ?= ${LIBDIR}/systemd/system
USERSYSTEMDDIR ?= ${LIBDIR}/systemd/user
SYSTEMDGENERATORSDIR ?= ${LIBDIR}/systemd/system-generators
USERSYSTEMDGENERATORSDIR ?= ${LIBDIR}/systemd/user-generators
SEQUOIA_SONAME_DIR =
REMOTETAGS ?= remote exclude_graphdriver_btrfs containers_image_openpgp
BUILDTAGS ?= \
	grpcnotrace \
	$(shell hack/apparmor_tag.sh) \
	$(shell hack/btrfs_installed_tag.sh) \
	$(shell hack/sqlite_tag.sh) \
	$(shell hack/systemd_tag.sh) \
	$(shell hack/libsubid_tag.sh) \
	$(if $(filter linux,$(GOOS)), seccomp,)
# allow downstreams to easily add build tags while keeping our defaults
BUILDTAGS += ${EXTRA_BUILDTAGS}
# N/B: This value is managed by Renovate, manual changes are
# possible, as long as they don't disturb the formatting
# (i.e. DO NOT ADD A 'v' prefix!)
GOLANGCI_LINT_VERSION := 2.12.2
PYTHON ?= $(shell command -v python3 python|head -n1)
PKG_MANAGER ?= $(shell command -v dnf yum|head -n1)
# ~/.local/bin is not in PATH on all systems
PRE_COMMIT = $(shell command -v bin/venv/bin/pre-commit ~/.local/bin/pre-commit pre-commit | head -n1)
ifeq ($(NATIVE_GOOS),freebsd)
SED=gsed
GREP=ggrep
MAN_L=	mandoc
else
SED=sed
GREP=grep
MAN_L=	man -l
endif

# This isn't what we actually build; it's a superset, used for target
# dependencies. Basically: all *.go and *.c files, except *_test.go,
# and except anything in a dot subdirectory. If any of these files is
# newer than our target (bin/podman{,-remote}), a rebuild is
# triggered.
SOURCES = $(shell find . -path './.*' -prune -o \( \( -name '*.go' -o -name '*.c' \) -a ! -name '*_test.go' \) -print) Makefile

BUILDTAGS_CROSS ?= containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay
OCI_RUNTIME ?= ""

# The 'sort' below is crucial: without it, 'make docs' behaves differently
# on the first run than on subsequent ones, because the generated .md
MANPAGES_SOURCE_DIR = docs/source/markdown
MANPAGES_MD_IN ?= $(wildcard $(MANPAGES_SOURCE_DIR)/*.md.in)
MANPAGES_MD_GENERATED ?= $(MANPAGES_MD_IN:%.md.in=%.md)
MANPAGES_MD ?= $(sort $(wildcard $(MANPAGES_SOURCE_DIR)/*.md) $(MANPAGES_MD_GENERATED) $(MANPAGES_SOURCE_DIR)/podman-troubleshooting.7.md $(MANPAGES_SOURCE_DIR)/podman-rootless.7.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)
MANPAGES_DEST ?= $(subst markdown,man, $(subst source,build,$(MANPAGES)))

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
ZSHINSTALLDIR=${PREFIX}/share/zsh/site-functions
FISHINSTALLDIR=${PREFIX}/share/fish/vendor_completions.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)


COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),$(call err_if_empty,COMMIT_NO)-dirty,$(COMMIT_NO))
DATE_FMT = %s
ifdef SOURCE_DATE_EPOCH
	BUILD_INFO ?= $(shell date -u -d "@$(call err_if_empty,SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u "+$(DATE_FMT)")
else
	BUILD_INFO ?= $(shell date "+$(DATE_FMT)")
endif
LIBPOD := ${PROJECT}/v6/libpod
GOFLAGS ?= -trimpath
LDFLAGS_PODMAN ?= \
	$(if $(GIT_COMMIT),-X $(LIBPOD)/define.gitCommit=$(GIT_COMMIT),) \
	$(if $(BUILD_INFO),-X $(LIBPOD)/define.buildInfo=$(BUILD_INFO),) \
	$(if $(BUILD_ORIGIN),-X "$(LIBPOD)/define.buildOrigin=$(BUILD_ORIGIN)",) \
	-X $(LIBPOD)/config._installPrefix=$(PREFIX) \
	-X $(LIBPOD)/config._etcDir=$(ETCDIR) \
	-X $(PROJECT)/v6/pkg/systemd/quadlet._binDir=$(BINDIR) \
	-X go.podman.io/image/v5/signature/internal/sequoia.sequoiaLibraryDir='"$(SEQUOIA_SONAME_DIR)"' \
	-X go.podman.io/common/pkg/config.additionalHelperBinariesDir=$(HELPER_BINARIES_DIR)\
	$(EXTRA_LDFLAGS)
LDFLAGS_PODMAN_STATIC ?= \
	$(LDFLAGS_PODMAN) \
	-extldflags=-static
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := v2.3.3
# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=90m
# By default, run test/e2e
GINKGOWHAT ?= test/e2e/.
GINKGO_PARALLEL=y
GINKGO ?= ./bin/ginkgo

# Allow control over some Ginkgo parameters
GINKGO_FLAKE_ATTEMPTS ?= 0
GINKGO_NO_COLOR ?= y

# The type of transport to use for testing remote service.
# Must be one of unix, tcp, tls, mtls
export REMOTESYSTEM_TRANSPORT ?= unix
export REMOTEINTEGRATION_TRANSPORT ?= unix

# Conditional required to produce empty-output if binary not built yet.
RELEASE_VERSION = $(shell if test -x test/version/version; then test/version/version; fi)
RELEASE_NUMBER = $(shell echo "$(call err_if_empty,RELEASE_VERSION)" | sed -e 's/^v\(.*\)/\1/')

# Logs all output from server during remote system testing to this file
PODMAN_SERVER_LOG ?= /dev/null

# Ensure GOBIN is not set so the default (`go env GOPATH`/bin) is used.
override undefine GOBIN
# This must never include the 'hack' directory
export PATH := $(shell $(GO) env GOPATH)/bin:$(PATH)

GOMD2MAN ?= ./test/tools/build/go-md2man

# There are many possibly unexpected places where podman is used.  For example
# by OpenWRT for routers and other similar small "edge" devices.  Testing builds
# for otherwise non-mainstream architectures ensures we catch platform-specific
# toolchain shenanigans early, for example:
# https://github.com/containers/podman/issues/8782
CROSS_BUILD_TARGETS := \
	bin/podman.cross.linux.amd64 \
	bin/podman.cross.linux.ppc64le \
	bin/podman.cross.linux.arm \
	bin/podman.cross.linux.arm64 \
	bin/podman.cross.linux.386 \
	bin/podman.cross.linux.s390x \
	bin/podman.cross.linux.loong64 \
	bin/podman.cross.linux.mips \
	bin/podman.cross.linux.mipsle \
	bin/podman.cross.linux.mips64 \
	bin/podman.cross.linux.mips64le \
	bin/podman.cross.linux.riscv64 \
	bin/podman.cross.freebsd.amd64 \
	bin/podman.cross.freebsd.arm64

# Dereference variable $(1), return value if non-empty, otherwise raise an error.
err_if_empty = $(if $(strip $($(1))),$(strip $($(1))),$(error Required variable $(1) value is undefined, whitespace, or empty))

ifeq ($(call err_if_empty,GOOS),windows)
BINSFX := .exe
SRCBINDIR := bin/windows
CGO_ENABLED := 0
else ifeq ($(GOOS),darwin)
BINSFX :=
SRCBINDIR := bin/darwin
CGO_ENABLED := 0
else ifeq ($(GOOS),freebsd)
BINSFX := -remote
SRCBINDIR := bin
RELEASE_PREFIX = /usr/local
else
BINSFX := -remote
SRCBINDIR := bin
endif
# Necessary for nested-$(MAKE) calls and docs/remote-docs.sh
export GOOS GOARCH CGO_ENABLED BINSFX SRCBINDIR

# Need to use CGO for mDNS resolution, but cross builds need CGO disabled
# See https://github.com/golang/go/issues/12524 for details
DARWIN_GCO := 0
ifeq ($(call err_if_empty,NATIVE_GOOS),darwin)
ifdef HOMEBREW_PREFIX
	DARWIN_GCO := 1
endif
endif

# gvisor-tap-vsock version for gvproxy.exe and win-sshproxy.exe downloads
# the upstream project ships pre-built binaries since version 0.7.1
GVPROXY_VERSION=$(shell $(GO) list -m -f '{{.Version}}' github.com/containers/gvisor-tap-vsock)

###
### Primary entry-point targets
###

.PHONY: default
default: all

.PHONY: all
all: binaries docs

.PHONY: binaries
ifeq ($(GOOS),freebsd)
binaries: podman podman-remote podman-testing ## (FreeBSD) Build podman, podman-remote, and podman-testing binaries
else ifneq (, $(findstring $(GOOS),darwin windows))
binaries: podman-remote ## (macOS/Windows) Build podman-remote (client) only binaries
else
binaries: podman podman-remote podman-testing podmansh rootlessport quadlet ## (Linux) Build podman, podman-remote, podmansh, rootlessport, and quadlet binaries
endif

# Extract text following double-# for targets, as their description for
# the `help` target.  Otherwise these simple-substitutions are resolved
# at reference-time (due to `=` and not `=:`).
_HLP_TGTS_RX = '^[[:print:]]+:.*?\#\# .*$$'
_HLP_TGTS_CMD = $(GREP) -E $(_HLP_TGTS_RX) $(MAKEFILE_LIST)
_HLP_TGTS_LEN = $(shell $(call err_if_empty,_HLP_TGTS_CMD) | cut -d : -f 1 | wc -L 2>/dev/null || echo "PARSING_ERROR")
# Separated condition for Darwin
ifeq ($(NATIVE_GOOS)$(_HLP_TGTS_LEN),darwinPARSING_ERROR)
ifneq (,$(wildcard /usr/local/bin/gwc))
_HLP_TGTS_LEN = $(shell $(call err_if_empty,_HLP_TGTS_CMD) | cut -d : -f 1 | gwc -L)
else
$(warning On Darwin (MacOS) installed coreutils is necessary)
$(warning Use 'brew install coreutils' command to install coreutils on your system)
endif
endif
_HLPFMT = "%-$(call err_if_empty,_HLP_TGTS_LEN)s %s\n"
.PHONY: help
help: ## (Default) Print listing of key targets with their descriptions
	@printf $(_HLPFMT) "Target:" "Description:"
	@printf $(_HLPFMT) "--------------" "--------------------"
	@$(_HLP_TGTS_CMD) | sort | \
		awk 'BEGIN {FS = ":(.*)?## "}; \
			{printf $(_HLPFMT), $$1, $$2}'

###
### Linting/Formatting/Code Validation targets
###

.PHONY: .gitvalidation
.gitvalidation: .install.gitvalidation
	@echo "Validating vs commit '$(call err_if_empty,EPOCH_TEST_COMMIT)'"
	GIT_CHECK_EXCLUDE="./vendor:./test/tools/vendor:docs/make.bat:test/buildah-bud/buildah-tests.diff:test/e2e/quadlet/remap-keep-id2.container" ./test/tools/build/git-validation -run short-subject -range $(EPOCH_TEST_COMMIT)..$(HEAD)

.PHONY: lint
lint: golangci-lint
ifeq ($(PRE_COMMIT),)
	@echo "FATAL: pre-commit was not found, make .install.pre-commit to installing it." >&2
	@exit 2
endif
	$(PRE_COMMIT) run -a

.PHONY: golangci-lint
golangci-lint: .install.golangci-lint
	hack/golangci-lint.sh

.PHONY: test/checkseccomp/checkseccomp
test/checkseccomp/checkseccomp: $(wildcard test/checkseccomp/*.go)
	$(GOCMD) build $(BUILDFLAGS) $(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o $@ ./test/checkseccomp

.PHONY: test/testvol/testvol
test/testvol/testvol: $(wildcard test/testvol/*.go)
	$(GOCMD) build -o $@ ./test/testvol

.PHONY: volume-plugin-test-img
volume-plugin-test-img:
	./bin/podman build --network none -t quay.io/libpod/volume-plugin-test-img:$$(date +%Y%m%d) -f ./test/testvol/Containerfile .

.PHONY: test/goecho/goecho
test/goecho/goecho: $(wildcard test/goecho/*.go)
	$(GOCMD) build $(BUILDFLAGS) $(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' -o $@ ./test/goecho

# The ./test/version/version binary is executed in other make steps
# so we have to make sure the version binary is built for NATIVE_GOARCH.
test/version/version: version/version.go
	GOARCH=$(NATIVE_GOARCH) $(GO) build -o $@ ./test/version/

.PHONY: codespell
codespell:
	# Configuration for codespell is in .codespellrc
	codespell -w

# Code validation target that **DOES NOT** require building podman binaries
.PHONY: validate-source
validate-source: lint .gitvalidation swagger-check tests-expect-exit

# Code validation target that **DOES** require building podman binaries
.PHONY: validate-binaries
validate-binaries: man-page-check validate.completions

.PHONY: validate
validate: validate-source validate-binaries

# The image used below is generated manually from contrib/validatepr/Containerfile in this podman repo.  The builds are
# not automated right now.  The hope is that eventually the quay.io/libpod/fedora_podman is multiarch and can replace this
# image in the future.
.PHONY: validatepr
validatepr: ## Go Format and lint, which all code changes must pass
	$(PODMANCMD) run --rm --init --tmpfs /tmp \
		-v $(CURDIR):/go/src/github.com/containers/podman \
		-v validatepr-gocache:/root/.cache/go-build \
		-v validatepr-gomodcache:/root/go/pkg/mod \
		-v validatepr-lintcache:/root/.cache/golangci-lint \
		-v validatepr-precommitcache:/root/.cache/pre-commit \
		--security-opt label=disable \
		--network=host \
		-it \
		-w /go/src/github.com/containers/podman \
		quay.io/libpod/validatepr:latest  \
		make .validatepr

.PHONY: .validatepr
.validatepr:
	env BUILDTAGS="$(BUILDTAGS)" REMOTETAGS="$(REMOTETAGS)" contrib/validatepr/validatepr.sh

.PHONY: build-all-new-commits
build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(call err_if_empty,GIT_BASE_BRANCH) -x "$(MAKE)"

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify
	$(GO) mod edit -toolchain none


# We define *-in-container targets for the following make targets. This allow the targets to be run in a container.
# Note that the PODMANCMD can also be overridden to allow a different container CLI to be used on systems where podman is not already available.
IN_CONTAINER_TARGETS = vendor validate
PODMANCMD ?= podman
IN_CONTAINER = $(patsubst %,%-in-container,$(IN_CONTAINER_TARGETS))

.PHONY: $(IN_CONTAINER)
$(IN_CONTAINER): %-in-container:
	$(PODMANCMD) run --rm --env HOME=/root \
		-v $(CURDIR):/src -w /src \
		--security-opt label=disable \
		quay.io/libpod/validatepr:latest \
		make $(*)


###
### Primary binary-build targets
###

# Make sure to warn in case we're building without the systemd buildtag.
bin/podman: $(SOURCES) go.mod go.sum
ifeq (,$(findstring systemd,$(BUILDTAGS)))
	@echo "Podman is being compiled without the systemd build tag. \
		Install libsystemd on Ubuntu or systemd-devel on rpm based \
		distro for journald support."
endif
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "$(BUILDTAGS)" \
		-o $@ ./cmd/podman
	test -z "${SELINUXOPT}" || chcon -t container_runtime_exec_t $@

# Disambiguate Linux vs Darwin/Windows platform binaries under distinct "bin" dirs
$(SRCBINDIR):
	mkdir -p $(SRCBINDIR)

# '|' is to ignore SRCBINDIR mtime; see: info make 'Types of Prerequisites'
$(SRCBINDIR)/podman$(BINSFX): $(SOURCES) go.mod go.sum | $(SRCBINDIR)
ifeq ($(GOOS)/$(GOARCH),darwin/amd64)
	$(error Podman 6 dropped darwin/amd64 support. Use an ARM64 Mac or cross-compile for a different GOOS/GOARCH.)
endif
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${REMOTETAGS}" \
		-o $@ ./cmd/podman

$(SRCBINDIR)/podman-remote-static-linux_%: GOARCH = $(patsubst $(SRCBINDIR)/podman-remote-static-linux_%,%,$@)
$(SRCBINDIR)/podman-remote-static-linux_%: GOOS = linux
$(SRCBINDIR)/podman-remote-static $(SRCBINDIR)/podman-remote-static-linux_amd64 $(SRCBINDIR)/podman-remote-static-linux_arm64: $(SRCBINDIR) $(SOURCES) go.mod go.sum
	CGO_ENABLED=0 \
	$(GO) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN_STATIC)' \
		-tags "${REMOTETAGS}" \
		-o $@ ./cmd/podman

.PHONY: podman
podman: bin/podman

# This will map to the right thing on Linux, Windows, and Mac.
.PHONY: podman-remote
podman-remote: $(SRCBINDIR)/podman$(BINSFX)

$(SRCBINDIR)/quadlet: $(SOURCES) go.mod go.sum
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${BUILDTAGS}" \
		-o $@ ./cmd/quadlet

.PHONY: quadlet
quadlet: bin/quadlet

.PHONY: podman-remote-static podman-remote-static-linux_amd64 podman-remote-static-linux_arm64
podman-remote-static: $(SRCBINDIR)/podman-remote-static
podman-remote-static-linux_amd64: $(SRCBINDIR)/podman-remote-static-linux_amd64
podman-remote-static-linux_arm64: $(SRCBINDIR)/podman-remote-static-linux_arm64

.PHONY: podman-winpath
podman-winpath: $(SOURCES) go.mod go.sum
	CGO_ENABLED=0 \
		GOOS=windows \
		$(GO) build \
		$(BUILDFLAGS) \
		-ldflags -H=windowsgui \
		-o bin/windows/winpath.exe \
		./cmd/winpath

.PHONY: podman-mac-helper
podman-mac-helper: ## Build podman-mac-helper for macOS
	CGO_ENABLED=0 \
		GOOS=darwin \
		GOARCH=$(GOARCH) \
		$(GO) build \
		$(BUILDFLAGS) \
		-o bin/darwin/podman-mac-helper \
		./cmd/podman-mac-helper

bin/rootlessport: $(SOURCES) go.mod go.sum
	CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build \
		$(BUILDFLAGS) \
		-o $@ ./cmd/rootlessport

.PHONY: rootlessport
rootlessport: bin/rootlessport

# podmansh calls `podman exec` into the `podmansh` container when used as
# os.Args[0] and is intended to be set as a login shell for users.
# Run: `man 1 podmansh` for details.
podmansh: bin/podman
	if [ ! -f bin/podmansh ]; then ln -s podman bin/podmansh; fi

$(SRCBINDIR)/podman-testing: $(SOURCES) go.mod go.sum
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${BUILDTAGS}" \
		-o $@ ./cmd/podman-testing

.PHONY: podman-testing
podman-testing: $(SRCBINDIR)/podman-testing

###
### Secondary binary-build targets
###

.PHONY: generate-bindings
generate-bindings: .install.golangci-lint
	$(GOCMD) generate ./pkg/bindings/... ;

# Do the cross build with the OS/ARCH extrcted from the target name, i.e.
# pass a path like "podman.cross.linux.amd64". This target is used by
# local-cross to build all CROSS_BUILD_TARGETS.
bin/podman.cross.%: $(SOURCES)
	TARGET="$*"; \
	GOOS="$${TARGET%%.*}"; \
	GOARCH="$${TARGET##*.}"; \
	CGO_ENABLED=0 \
		$(GO) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags '$(BUILDTAGS_CROSS)' \
		-o "$@" ./cmd/podman

.PHONY: local-cross
local-cross: $(CROSS_BUILD_TARGETS) ## Cross compile podman binary for multiple architectures

.PHONY: cross
cross: local-cross

# Simple target to check that we can build all binaries for another arch,
# the resulting binaries are not meant to be usable this is just for
# testing if it builds, it depends on the caller to set GOOS/GOARCH.
.PHONY: cross-binaries
cross-binaries:
	$(MAKE) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		BUILDTAGS="$(BUILDTAGS_CROSS)" clean-binaries binaries

.PHONY: completions
completions: podman podman-remote
	# key = shell, value = completion filename
	declare -A outfiles=([bash]=%s [zsh]=_%s [fish]=%s.fish [powershell]=%s.ps1);\
	for shell in $${!outfiles[*]}; do \
	    for remote in "" "-remote"; do \
		podman="podman$$remote"; \
		outfile=$$(printf "completions/$$shell/$${outfiles[$$shell]}" $$podman); \
		./bin/$$podman completion $$shell >| $$outfile; \
	    done;\
	done

###
### Documentation targets
###

pkg/api/swagger.yaml: .install.swagger
	make -C pkg/api

$(MANPAGES_MD_GENERATED): %.md: %.md.in $(MANPAGES_SOURCE_DIR)/options/*.md
	hack/markdown-preprocess

$(MANPAGES_SOURCE_DIR)/podman-troubleshooting.7.md: troubleshooting.md
	( echo "% podman-troubleshooting 7"; echo; sed -e '/logo.*\.png/d' <$< ) >$@.tmp.$$ && \
		mv $@.tmp.$$ $@

$(MANPAGES_SOURCE_DIR)/podman-rootless.7.md: rootless.md
	( echo "% podman-rootless 7"; echo; sed -e '/logo.*\.png/d' <$< ) >$@.tmp.$$ && \
		mv $@.tmp.$$ $@

$(MANPAGES): OUTFILE=$(subst source/markdown,build/man,$@)
$(MANPAGES): %: %.md .install.md2man docdir

# This does a bunch of filtering needed for man pages:
#  1. Convert all markdown site links to plain text:
#           [foo](https://www.....)   ->   foo
#  2. Strip man-page targets like '[podman(1)](podman.1.md)'
#     to just '[podman(1)]', because man pages have no link mechanism;
#  3. Then remove the brackets: '[podman(1)]' -> 'podman(1)';
#  4. Remove HTML-ish stuff like '<sup>..</sup>' and '<a>..</a>'
#  5. Replace "\" (backslash) at EOL with two spaces (no idea why)
# Then two sanity checks:
#  1. test for "included file options/blahblah"; this indicates a failure
#     in the markdown-preprocess tool; and
#  2. run 'man -l' against the generated man page, and check for tables
#     with an empty right-hand column followed by an empty left-hand
#     column on the next line. (Technically, on the next-next line,
#     because the next line must be table borders). This is a horrible
#     unmaintainable rats-nest of duplication, obscure grep options, and
#     ASCII art. I (esm) believe the cost of releasing corrupt man pages
#     is higher than the cost of carrying this kludge.
#
	@$(SED) -e 's/\[\([^]]*\)](http[^)]\+)/\1/g'         \
	        -e 's/\((podman[^)]*\.md\(#.*\)\?)\)//g'     \
	        -e 's/\[\(podman[^]]*\)\]/\1/g'              \
	        -e 's;<\(/\)\?\(a\|a\s\+[^>]*\|sup\)>;;g'    \
	        -e 's/\\$$/  /g' $<                         |\
	$(GOMD2MAN) -out $(OUTFILE)
	@if grep 'included file options/' $(OUTFILE); then \
		echo "FATAL: man pages must not contain ^^^^ in $(OUTFILE)"; exit 1; \
	fi
	@if $(MAN_L) $(OUTFILE)| $(GREP) -Pazoq '│\s+│\n\s+├─+┼─+┤\n\s+│\s+│'; then  \
		echo "FATAL: $< has a too-long table column; use 'man -l $(OUTFILE)' and look for empty table cells."; exit 1; \
	fi

.PHONY: docdir
docdir:
	mkdir -p docs/build/man

.PHONY: docs
docs: $(MANPAGES) ## Generate documentation
	@ln -sf $(CURDIR)/docs/source/markdown/links/* docs/build/man/

# docs/remote-docs.sh requires a locally executable 'podman-remote' binary
# in addition to the target-architecture binary (if different). That's
# what the NATIVE_GOOS make does in the first line.
podman-remote-%-docs: podman-remote
	$(MAKE) clean-binaries
	$(MAKE) podman-remote GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH)
	$(eval GOOS := $*)
	$(MAKE) docs $(MANPAGES)
	rm -rf docs/build/remote
	mkdir -p docs/build/remote
	ln -sf $(CURDIR)/docs/source/markdown/links docs/build/man/
	docs/remote-docs.sh \
		$(GOOS) \
		docs/build/remote/$* \
		$(if $(findstring windows,$*),docs/source/markdown,docs/build/man)

.PHONY: man-page-check
man-page-check: man-page-checker xref-helpmsgs-manpages xref-quadlet-docs xref-quadlet-docs

man-page-checker: bin/podman docs
	hack/man-page-checker

xref-helpmsgs-manpages: bin/podman docs
	hack/xref-helpmsgs-manpages

man-page-table-check: docs
	hack/man-page-table-check

xref-quadlet-docs: docs
	hack/xref-quadlet-docs

.PHONY: swagger-check
swagger-check:
	hack/swagger-check

.PHONY: swagger
swagger: pkg/api/swagger.yaml

.PHONY: docker-docs
docker-docs: docs
	(cd docs; ./dckrman.sh ./build/man/*.1)

# Workaround vim syntax highlighting bug: "

###
### Utility and Testing targets
###

.PHONY: validate.completions
validate.completions: SHELL:=/usr/bin/env bash # Set shell to bash for this target
validate.completions: completions
	# Check if the files can be loaded by the shell
	. completions/bash/podman
	if [ -x /bin/zsh ]; then /bin/zsh completions/zsh/_podman; fi
	if [ -x /bin/fish ]; then /bin/fish completions/fish/podman.fish; fi

# Note: Assumes test/python/requirements.txt is installed & available
.PHONY: run-docker-py-tests
run-docker-py-tests:
	touch test/__init__.py
	env CONTAINERS_CONF=$(CURDIR)/test/apiv2/containers.conf pytest --disable-warnings test/python/docker/
	rm -f test/__init__.py

.PHONY: localunit
localunit: test/goecho/goecho test/version/version
	rm -rf ${COVERAGE_PATH} && mkdir -p ${COVERAGE_PATH}
	UNIT=1 $(GINKGO) \
		-r \
		$(TESTFLAGS) \
		--skip-package test/e2e,pkg/bindings,hack,pkg/machine/e2e,pkg/machine/wsl,pkg/machine/hyperv \
		--cover \
		--covermode atomic \
		--coverprofile coverprofile \
		--output-dir ${COVERAGE_PATH} \
		--tags "$(BUILDTAGS)" \
		--succinct
	$(GO) tool cover -html=${COVERAGE_PATH}/coverprofile -o ${COVERAGE_PATH}/coverage.html
	$(GO) tool cover -func=${COVERAGE_PATH}/coverprofile > ${COVERAGE_PATH}/functions
	cat ${COVERAGE_PATH}/functions | sed -n 's/\(total:\).*\([0-9][0-9].[0-9]\)/\1 \2/p'

.PHONY: test
test: localunit localintegration remoteintegration localsystem remotesystem  ## Run unit, integration, and system tests.

.PHONY: ginkgo-run
# e2e tests need access to podman-registry
ginkgo-run: PATH := $(PATH):$(CURDIR)/hack
ginkgo-run: .install.ginkgo
	$(GINKGO) version
	$(GINKGO) -vv $(TESTFLAGS) --tags "$(TAGS) remote" $(GINKGOTIMEOUT) --flake-attempts $(GINKGO_FLAKE_ATTEMPTS) \
		--trace $(if $(findstring y,$(GINKGO_NO_COLOR)),--no-color,) \
		$(if $(findstring y,$(GINKGO_PARALLEL)),-p,) \
		$(if $(FOCUS),--focus "$(FOCUS)" --silence-skips,) \
		$(if $(FOCUS_FILE),--focus-file "$(FOCUS_FILE)" --silence-skips,) $(GINKGOWHAT)

.PHONY: ginkgo
ginkgo:
	$(MAKE) ginkgo-run TAGS="$(BUILDTAGS)"


.PHONY: ginkgo-remote
ginkgo-remote:
	$(MAKE) ginkgo-run TAGS="$(REMOTETAGS) remote_testing"

.PHONY: testbindings
# bindings tests need access to podman-registry
testbindings: PATH := $(PATH):$(CURDIR)/hack
testbindings: .install.ginkgo
	$(GINKGO) -v $(TESTFLAGS) --tags "$(TAGS) remote" $(GINKGOTIMEOUT) --trace --no-color --timeout 30m  -v -r ./pkg/bindings/test

.PHONY: localintegration
localintegration: test-binaries ginkgo

.PHONY: remoteintegration
remoteintegration: test-binaries ginkgo-remote

.PHONY: localmachine
localmachine:
	# gitCommit needed by logformatter, to link to sources
	@echo /define.gitCommit=$(GIT_COMMIT)
	$(MAKE) ginkgo-run GINKGO_PARALLEL=n TAGS="$(REMOTETAGS)" GINKGO_FLAKE_ATTEMPTS=0 FOCUS_FILE=$(FOCUS_FILE) GINKGOWHAT=pkg/machine/e2e/.

.PHONY: localsystem
localsystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	PODMAN=$(CURDIR)/bin/podman QUADLET=$(CURDIR)/bin/quadlet bats -T --filter-tags '!ci:parallel' test/system/
	PODMAN=$(CURDIR)/bin/podman QUADLET=$(CURDIR)/bin/quadlet bats -T --filter-tags ci:parallel -j $$(nproc) test/system/


.PHONY: remotesystem
remotesystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	PODMAN=$(CURDIR)/bin/podman-remote QUADLET=$(CURDIR)/bin/quadlet \
		bats -T --filter-tags '!ci:parallel' test/system/
	PODMAN=$(CURDIR)/bin/podman-remote QUADLET=$(CURDIR)/bin/quadlet \
		bats -T --filter-tags ci:parallel -j $$(nproc) test/system/

.PHONY: localapiv2-bash
localapiv2-bash:
	env PODMAN=./bin/podman stdbuf -o0 -e0 ./test/apiv2/test-apiv2

.PHONY: localapiv2-python
localapiv2-python:
	env CONTAINERS_CONF=$(CURDIR)/test/apiv2/containers.conf PODMAN=./bin/podman \
		pytest --verbose --disable-warnings ./test/apiv2/python

# Order is important running python tests first causes the bash tests
# to fail, see 12-imagesMore.  FIXME order of tests should not matter
.PHONY: localapiv2
localapiv2: localapiv2-bash localapiv2-python

.PHONY: remoteapiv2
remoteapiv2:
	true

.PHONY: system.test-binary
system.test-binary: .install.ginkgo
	$(GO) test -c ./test/system

.PHONY: test-binaries
test-binaries: test/checkseccomp/checkseccomp test/goecho/goecho test/version/version
	@echo "Canonical source version: $(call err_if_empty,RELEASE_VERSION)"

.PHONY: tests-included
tests-included:
	contrib/cirrus/pr-should-include-tests

.PHONY: tests-expect-exit
tests-expect-exit:
	@if grep -E --line-number 'Expect.*ExitCode' test/e2e/*.go | grep -E -v ', ".*"\)'; then \
		echo "^^^ Unhelpful use of Expect(ExitCode())"; \
		echo "   Please use '.Should(Exit(...))' pattern instead."; \
		echo "   If that's not possible, please add an annotation (description) to your assertion:"; \
		echo "        Expect(...).To(..., \"Friendly explanation of this check\")"; \
		exit 1; \
	fi

###
### Release/Packaging targets
###

.PHONY: podman-release
podman-release: podman-release-$(GOARCH).tar.gz  # Build all Linux binaries for $GOARCH, docs., and installation tree, into a tarball.

# The following two targets are nuanced and complex:
# Cross-building the podman-remote documentation requires a functional
# native architecture executable.  However `make` only deals with
# files/timestamps, it doesn't understand if an existing binary will
# function on the system or not.  This makes building cross-platform
# releases incredibly accident-prone and fragile.  The only practical
# way to deal with this, is via multiple conditional (nested) `make`
# calls along with careful manipulation of `$GOOS` and `$GOARCH`.

podman-release-%.tar.gz: test/version/version
	$(eval tmpsubdir := $(shell mktemp -d podman_tmp_XXXX))
	$(eval releasedir := podman-v$(call err_if_empty,RELEASE_NUMBER))
	$(eval _dstargs := "DESTDIR=$(tmpsubdir)/$(releasedir)" "PREFIX=$(RELEASE_PREFIX)")
	$(eval GOARCH := $*)
	mkdir -p "$(call err_if_empty,tmpsubdir)/$(releasedir)"
	$(MAKE) GOOS=$(GOOS) GOARCH=$(NATIVE_GOARCH) \
		clean-binaries docs podman-remote-$(GOOS)-docs
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then \
		$(MAKE) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
			BUILDTAGS="$(BUILDTAGS_CROSS)" clean-binaries binaries; \
	else \
		$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) binaries; \
	fi
	$(MAKE) $(_dstargs) install.bin install.remote install.man install.systemd
	tar -czvf $@ --xattrs -C "$(tmpsubdir)" "./$(releasedir)"
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then $(MAKE) clean-binaries; fi
	-rm -rf "$(tmpsubdir)"

podman-remote-release-%.zip: test/version/version ## Build podman-remote for %=$GOOS_$GOARCH, and docs. into an installation zip.
	$(eval tmpsubdir := $(shell mktemp -d podman_tmp_XXXX))
	$(eval releasedir := podman-$(call err_if_empty,RELEASE_NUMBER))
	$(eval _dstargs := "DESTDIR=$(tmpsubdir)/$(releasedir)" "PREFIX=$(RELEASE_PREFIX)")
	$(eval GOOS := $(firstword $(subst _, ,$*)))
	$(eval GOARCH := $(lastword $(subst _, ,$*)))
	$(eval _GOPLAT := GOOS=$(call err_if_empty,GOOS) GOARCH=$(call err_if_empty,GOARCH))
	mkdir -p "$(call err_if_empty,tmpsubdir)/$(releasedir)"
	$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		clean-binaries podman-remote-$(GOOS)-docs
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then \
		$(MAKE) CGO_ENABLED=0 $(GOPLAT) BUILDTAGS="$(BUILDTAGS_CROSS)" \
			clean-binaries podman-remote; \
	else \
		$(MAKE) $(GOPLAT) podman-remote; \
	fi
	if [[ "$(GOOS)" == "windows" ]]; then \
		$(MAKE) $(GOPLAT) TMPDIR="" win-gvproxy-$(GOARCH); \
	fi
	if [[ "$(GOOS)" == "darwin" ]]; then \
		$(MAKE) $(GOPLAT) podman-mac-helper;\
	fi
	cp -r ./docs/build/remote/$(GOOS) "$(tmpsubdir)/$(releasedir)/docs/"
	$(MAKE) $(GOPLAT) $(_dstargs) SELINUXOPT="" install.remote
	cd "$(tmpsubdir)" && \
		zip --recurse-paths "$(CURDIR)/$@" "./$(releasedir)"
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then $(MAKE) clean-binaries; fi
	-rm -rf "$(tmpsubdir)"

# Downloads pre-built gvproxy and win-sshproxy helpers. See comment on GVPROXY_VERSION declaration
.PHONY: win-gvproxy
win-gvproxy: win-gvproxy-amd64 # Keep this target for backwards compatibility

win-gvproxy-%: test/version/version
	$(eval GOARCH := $*)
	$(eval GVPROXY_FILENAME := $(if $(filter arm64,$(GOARCH)), gvproxy-windows-arm64.exe,gvproxy-windowsgui.exe))
	$(eval SSHPROXY_FILENAME :=  $(if $(filter arm64,$(GOARCH)), win-sshproxy-arm64.exe, win-sshproxy.exe))
	mkdir -p bin/windows/
	curl --fail -sSL -o bin/windows/gvproxy.exe --retry 5 https://github.com/containers/gvisor-tap-vsock/releases/download/$(GVPROXY_VERSION)/$(GVPROXY_FILENAME)
	curl --fail -sSL -o bin/windows/win-sshproxy.exe --retry 5 https://github.com/containers/gvisor-tap-vsock/releases/download/$(GVPROXY_VERSION)/$(SSHPROXY_FILENAME)

.PHONY: rpm
rpm:  ## Build rpm packages
	$(MAKE) -C rpm

###
### Installation targets
###

# Remember that rpms install exec to /usr/bin/podman while a `make install`
# installs them to /usr/local/bin/podman which is likely before. Always use
# a full path to test installed podman or you risk to call another executable.
.PHONY: rpm-install
rpm-install: package  ## Install rpm packages
	$(call err_if_empty,PKG_MANAGER) -y install rpm/RPMS/*/*.rpm
	/usr/bin/podman version
	/usr/bin/podman info  # will catch a broken conmon

.PHONY: install
install: install.bin install.remote install.man install.systemd  ## Install binaries to system locations

.PHONY: install.remote
install.remote:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 $(SRCBINDIR)/podman$(BINSFX) \
		$(DESTDIR)$(BINDIR)/podman$(BINSFX)
	test "${GOOS}" != "windows" || \
		install -m 755 $(SRCBINDIR)/win-sshproxy.exe $(DESTDIR)$(BINDIR)
	test "${GOOS}" != "windows" || \
		install -m 755 $(SRCBINDIR)/gvproxy.exe $(DESTDIR)$(BINDIR)
	test "${GOOS}" != "darwin" || \
		install -m 755 $(SRCBINDIR)/podman-mac-helper $(DESTDIR)$(BINDIR)
	test -z "${SELINUXOPT}" || \
		chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman-remote \
		bin/podman-remote

.PHONY: install.bin
install.bin:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman $(DESTDIR)$(BINDIR)/podman
	ln -sf podman $(DESTDIR)$(BINDIR)/podmansh
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman bin/podman
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(LIBEXECPODMAN)
ifneq ($(NATIVE_GOOS),freebsd)
	install ${SELINUXOPT} -m 755 bin/rootlessport $(DESTDIR)$(LIBEXECPODMAN)/rootlessport
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(LIBEXECPODMAN)/rootlessport bin/rootlessport
	install ${SELINUXOPT} -m 755 bin/quadlet $(DESTDIR)$(LIBEXECPODMAN)/quadlet
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(SYSTEMDGENERATORSDIR)
	ln -sfr $(DESTDIR)$(LIBEXECPODMAN)/quadlet $(DESTDIR)$(SYSTEMDGENERATORSDIR)/podman-system-generator
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(USERSYSTEMDGENERATORSDIR)
	ln -sfr $(DESTDIR)$(LIBEXECPODMAN)/quadlet $(DESTDIR)$(USERSYSTEMDGENERATORSDIR)/podman-user-generator
	install ${SELINUXOPT} -m 755 -d $(DESTDIR)${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/tmpfile/podman.conf $(DESTDIR)${TMPFILESDIR}/podman.conf
endif

.PHONY: install.testing
install.testing:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman-testing $(DESTDIR)$(BINDIR)/podman-testing

.PHONY: install.man
install.man:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man7
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES_DEST)) $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 docs/source/markdown/links/*1 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES_DEST)) $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/source/markdown/links/*5 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.7,$(MANPAGES_DEST)) $(DESTDIR)$(MANDIR)/man7

.PHONY: install.completions
install.completions:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman $(DESTDIR)${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman-remote $(DESTDIR)${BASHINSTALLDIR}
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)${ZSHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/zsh/_podman $(DESTDIR)${ZSHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/zsh/_podman-remote $(DESTDIR)${ZSHINSTALLDIR}
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)${FISHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/fish/podman.fish $(DESTDIR)${FISHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/fish/podman-remote.fish $(DESTDIR)${FISHINSTALLDIR}
	# There is no common location for powershell files so do not install them. Users have to source the file from their powershell profile.

.PHONY: install.docker
install.docker:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	$(eval INTERPOLATED_DOCKER_SCRIPT := $(shell mktemp))
	env BINDIR=${BINDIR} ETCDIR=${ETCDIR} envsubst '$$BINDIR;$$ETCDIR' < docker/docker.in > ${INTERPOLATED_DOCKER_SCRIPT}
	install ${SELINUXOPT} -m 755 ${INTERPOLATED_DOCKER_SCRIPT} $(DESTDIR)$(BINDIR)/docker
	rm ${INTERPOLATED_DOCKER_SCRIPT}
	install ${SELINUXOPT} -m 755 -d $(DESTDIR)${SYSTEMDDIR}  $(DESTDIR)${USERSYSTEMDDIR} $(DESTDIR)${TMPFILESDIR} $(DESTDIR)${USERTMPFILESDIR}
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)${ETCDIR}/profile.d
	install ${SELINUXOPT} -m 644 docker/podman-docker.sh $(DESTDIR)${ETCDIR}/profile.d/podman-docker.sh
	install ${SELINUXOPT} -m 644 docker/podman-docker.csh $(DESTDIR)${ETCDIR}/profile.d/podman-docker.csh
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-docker.conf -t $(DESTDIR)${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-docker.conf -t $(DESTDIR)${USERTMPFILESDIR}

.PHONY: install.docker-docs
install.docker-docs:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.1 -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.5 -t $(DESTDIR)$(MANDIR)/man5

.PHONY: install.docker-full
install.docker-full: install.docker install.docker-docs

.PHONY: install.systemd
ifneq (,$(findstring systemd,$(BUILDTAGS)))
PODMAN_GENERATED_UNIT_FILES = contrib/systemd/system/podman-auto-update.service \
		    contrib/systemd/system/podman.service \
		    contrib/systemd/system/podman-restart.service \
		    contrib/systemd/system/podman-kube@.service \
		    contrib/systemd/system/podman-clean-transient.service

%.service: %.service.in
	sed -e 's;@@PODMAN@@;$(BINDIR)/podman;g' $< >$@.tmp.$$ \
		&& mv -f $@.tmp.$$ $@

install.systemd: $(PODMAN_GENERATED_UNIT_FILES)
	install ${SELINUXOPT} -m 755 -d $(DESTDIR)${SYSTEMDDIR}  $(DESTDIR)${USERSYSTEMDDIR}
	for unit in $^ \
				contrib/systemd/system/podman-auto-update.timer \
				contrib/systemd/system/podman.socket; do \
		install ${SELINUXOPT} -m 644 $$unit $(DESTDIR)${USERSYSTEMDDIR}/$$(basename $$unit); \
		install ${SELINUXOPT} -m 644 $$unit $(DESTDIR)${SYSTEMDDIR}/$$(basename $$unit); \
	done
	# HACK; as rootless this unit will not work due the requires on a non existing target
	# as the user session does not see system units. We could define two different units
	# but this seems much more complicated then this small fixup here.
	# https://github.com/containers/podman/issues/23790
	sed -i '/Requires=/d' $(DESTDIR)${USERSYSTEMDDIR}/podman-clean-transient.service
	sed -i '/After=/d' $(DESTDIR)${USERSYSTEMDDIR}/podman-clean-transient.service

	# Important this unit should only be installed for the user session and is thus not added to the loop above.
	install ${SELINUXOPT} -m 644 contrib/systemd/user/podman-user-wait-network-online.service \
		$(DESTDIR)${USERSYSTEMDDIR}/podman-user-wait-network-online.service
	rm -f $^
else
install.systemd:
endif

.PHONY: install.tools
install.tools: .install.golangci-lint ## Install needed tools
	$(MAKE) -C test/tools

.PHONY: .install.ginkgo
.install.ginkgo:
	$(GO) build -o $(GINKGO) ./vendor/github.com/onsi/ginkgo/v2/ginkgo

.PHONY: .install.gitvalidation
.install.gitvalidation:
	$(MAKE) -C test/tools build/git-validation

.PHONY: .install.golangci-lint
.install.golangci-lint:
	VERSION=$(GOLANGCI_LINT_VERSION) ./hack/install_golangci.sh

.PHONY: .install.swagger
.install.swagger:
	$(MAKE) -C test/tools build/swagger

.PHONY: .install.md2man
.install.md2man:
	if [ ! -x "$(GOMD2MAN)" ]; then \
		$(MAKE) -C test/tools build/go-md2man GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH); \
	fi

.PHONY: .install.pre-commit
.install.pre-commit:
	if [ -z "$(PRE_COMMIT)" ]; then \
		$(PYTHON) -m pip install --user pre-commit; \
	fi

.PHONY: release-artifacts
release-artifacts: clean-binaries
	mkdir -p release/
	$(MAKE) podman-remote-release-darwin_arm64.zip
	mv podman-remote-release-darwin_arm64.zip release/
	$(MAKE) podman-remote-release-windows_amd64.zip
	mv podman-remote-release-windows_amd64.zip release/
	$(MAKE) podman-remote-release-windows_arm64.zip
	mv podman-remote-release-windows_arm64.zip release/
	$(MAKE) podman-remote-static-linux_amd64
	tar -cvzf podman-remote-static-linux_amd64.tar.gz bin/podman-remote-static-linux_amd64
	$(MAKE) podman-remote-static-linux_arm64
	tar -cvzf podman-remote-static-linux_arm64.tar.gz bin/podman-remote-static-linux_arm64
	mv podman-remote-static-linux*.tar.gz release/

.PHONY: uninstall
uninstall:
	for i in $(filter %.1,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man5/$$(basename $${i}); \
	done
	# Remove podman and remote bin
	rm -f $(DESTDIR)$(BINDIR)/podman
	rm -f $(DESTDIR)$(BINDIR)/podman-remote
	# Remove related config files
	rm -f $(DESTDIR)${TMPFILESDIR}/podman.conf
	rm -f $(DESTDIR)${SYSTEMDDIR}/podman.service
	rm -f $(DESTDIR)${SYSTEMDDIR}/podman.socket
	rm -f $(DESTDIR)${USERSYSTEMDDIR}/podman.socket
	rm -f $(DESTDIR)${USERSYSTEMDDIR}/podman.service

.PHONY: clean-binaries
clean-binaries: ## Remove platform/architecture specific binary files
	rm -rf \
		bin

.PHONY: clean
clean: clean-binaries ## Clean all make artifacts
	rm -rf \
		$(wildcard podman-*.msi) \
		$(wildcard podman-remote*.zip) \
		$(wildcard podman_tmp_*) \
		$(wildcard podman*.tar.gz) \
		build \
		test/checkseccomp/checkseccomp \
		test/goecho/goecho \
		test/version/version \
		test/__init__.py \
		docs/build \
		.venv
	make -C docs clean
