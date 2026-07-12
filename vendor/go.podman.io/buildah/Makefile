export GOPROXY=https://proxy.golang.org

APPARMORTAG := $(shell hack/apparmor_tag.sh)
STORAGETAGS := $(shell ./btrfs_installed_tag.sh) $(shell ./hack/libsubid_tag.sh)
SECURITYTAGS ?= seccomp $(APPARMORTAG)
TAGS ?= $(SECURITYTAGS) $(STORAGETAGS) $(shell ./hack/systemd_tag.sh) $(shell ./hack/sqlite_tag.sh)
ifeq ($(shell uname -s),FreeBSD)
endif
BUILDTAGS += $(TAGS) $(EXTRA_BUILD_TAGS)
PREFIX := /usr/local
BINDIR := $(PREFIX)/bin
BASHINSTALLDIR = $(PREFIX)/share/bash-completion/completions
BUILDFLAGS := -tags "$(BUILDTAGS)"
BUILDAH := buildah
SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)
SELINUXTYPE=container_runtime_exec_t
AS ?= as
STRIP ?= strip

GO := go
GO_LDFLAGS := $(shell if $(GO) version|grep -q gccgo; then echo "-gccgoflags"; else echo "-ldflags"; fi)
GO_GCFLAGS := $(shell if $(GO) version|grep -q gccgo; then echo "-gccgoflags"; else echo "-gcflags"; fi)
NPROCS := $(shell nproc)
export GO_BUILD=$(GO) build
export GO_TEST=$(GO) test -parallel=$(NPROCS)
RACEFLAGS ?= $(shell $(GO_TEST) -race ./pkg/dummy > /dev/null 2>&1 && echo -race)

COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),${COMMIT_NO}-dirty,${COMMIT_NO})
SOURCE_DATE_EPOCH ?= $(if $(shell date +%s),$(shell date +%s),$(error "date failed"))

# we get GNU make 3.x in MacOS build envs, which wants # to be escaped in
# strings, while the 4.x we have on Linux doesn't. this is the documented
# workaround
COMMENT := \#
SEQUOIA_SONAME_DIR =
EXTRA_LDFLAGS ?=
BUILDAH_LDFLAGS := $(GO_LDFLAGS) '-X main.GitCommit=$(GIT_COMMIT) -X main.buildInfo=$(SOURCE_DATE_EPOCH) -X go.podman.io/image/v5/signature/internal/sequoia.sequoiaLibraryDir="$(SEQUOIA_SONAME_DIR)" $(EXTRA_LDFLAGS)'

# This isn't what we actually build; it's a superset, used for target
# dependencies. Basically: all *.go and *.c files, except *_test.go,
# and except anything in a dot subdirectory. If any of these files is
# newer than our target (bin/buildah), a rebuild is triggered.
SOURCES=$(shell find . -path './.*' -prune -o \( \( -name '*.go' -o -name '*.c' \) -a ! -name '*_test.go' \) -print)

LINTFLAGS ?=

ifeq ($(BUILDDEBUG), 1)
  override GOGCFLAGS += -N -l
endif

# Managed by renovate.
export GOLANGCI_LINT_VERSION := 2.11.1

#   make all BUILDDEBUG=1
#     Note: Uses the -N -l go compiler options to disable compiler optimizations
#           and inlining. Using these build options allows you to subsequently
#           use source debugging tools like delve.
all: bin/buildah bin/imgtype bin/copy bin/inet bin/tutorial bin/dumpspec bin/passwd bin/crash bin/wait bin/grpcnoop docs

bin/buildah: $(SOURCES) internal/mkcw/embed/entrypoint_amd64.gz
	$(GO_BUILD) $(BUILDAH_LDFLAGS) $(GO_GCFLAGS) "$(GOGCFLAGS)" -o $@ $(BUILDFLAGS) ./cmd/buildah
	test -z "${SELINUXOPT}" || chcon --verbose -t $(SELINUXTYPE) $@

internal/mkcw/embed/entrypoint_amd64.gz: internal/mkcw/embed/entrypoint_amd64
	gzip -k9nf $^
internal/mkcw/embed/entrypoint_arm64.gz: internal/mkcw/embed/entrypoint_arm64
	gzip -k9nf $^
internal/mkcw/embed/entrypoint_ppc64le.gz: internal/mkcw/embed/entrypoint_ppc64le
	gzip -k9nf $^
internal/mkcw/embed/entrypoint_s390x.gz: internal/mkcw/embed/entrypoint_s390x
	gzip -k9nf $^

ifneq ($(shell $(AS) --version | grep -E 'x86_64-([^-]+-)?linux'),)
internal/mkcw/embed/entrypoint_amd64: internal/mkcw/embed/asm/entrypoint_amd64.s
	$(AS) -o $(patsubst %.s,%.o,$^) $^
	$(LD) -o $@ $(patsubst %.s,%.o,$^)
	$(STRIP) $@
else
internal/mkcw/embed/entrypoint_amd64: internal/mkcw/embed/entrypoint_amd64.s internal/mkcw/embed/entrypoint.go
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "-E _start -s" -o $@ ./internal/mkcw/embed
endif

internal/mkcw/embed/entrypoint_arm64: internal/mkcw/embed/entrypoint_arm64.s internal/mkcw/embed/entrypoint.go
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags "-E _start -s" -o $@ ./internal/mkcw/embed
internal/mkcw/embed/entrypoint_ppc64le: internal/mkcw/embed/entrypoint_ppc64le.s internal/mkcw/embed/entrypoint.go
	GOOS=linux GOARCH=ppc64le $(GO) build -ldflags "-E _start -s" -o $@ ./internal/mkcw/embed
internal/mkcw/embed/entrypoint_s390x: internal/mkcw/embed/entrypoint_s390x.s internal/mkcw/embed/entrypoint.go
	GOOS=linux GOARCH=s390x $(GO) build -ldflags "-E _start -s" -o $@ ./internal/mkcw/embed

.PHONY: buildah
buildah: bin/buildah

ALL_CROSS_TARGETS := $(addprefix bin/buildah.,$(subst /,.,$(shell $(GO) tool dist list)))
LINUX_CROSS_TARGETS := $(filter-out %.loong64,$(filter bin/buildah.linux.%,$(ALL_CROSS_TARGETS)))
DARWIN_CROSS_TARGETS := $(filter bin/buildah.darwin.%,$(ALL_CROSS_TARGETS))
WINDOWS_CROSS_TARGETS := $(addsuffix .exe,$(filter bin/buildah.windows.%,$(ALL_CROSS_TARGETS)))
FREEBSD_CROSS_TARGETS := $(filter bin/buildah.freebsd.%,$(ALL_CROSS_TARGETS))
.PHONY: cross
cross: $(LINUX_CROSS_TARGETS) $(DARWIN_CROSS_TARGETS) $(WINDOWS_CROSS_TARGETS) $(FREEBSD_CROSS_TARGETS)

bin/buildah.%: $(SOURCES) internal/mkcw/embed/entrypoint_amd64.gz
	mkdir -p ./bin
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ -tags "containers_image_openpgp" ./cmd/buildah

bin/crash: $(SOURCES)
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/crash

bin/wait: $(SOURCES)
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/wait

bin/dumpspec: $(SOURCES)
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/dumpspec

bin/imgtype: $(SOURCES)
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/imgtype/imgtype.go

bin/copy: $(SOURCES)
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/copy/copy.go

bin/tutorial: $(SOURCES) internal/mkcw/embed/entrypoint_amd64.gz
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/tutorial/tutorial.go

bin/inet: tests/inet/inet.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/inet/inet.go

bin/passwd: tests/passwd/passwd.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/passwd/passwd.go

bin/grpcnoop: tests/rpc/noop/noop.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/rpc/noop/noop.go

.PHONY: clean
clean:
	$(RM) -r bin tests/testreport/testreport tests/conformance/testdata/mount-targets/true internal/mkcw/embed/entrypoint_arm64 internal/mkcw/embed/entrypoint_ppc64le internal/mkcw/embed/entrypoint_s390x internal/mkcw/embed/entrypoint_arm64.gz internal/mkcw/embed/entrypoint_ppc64le.gz internal/mkcw/embed/entrypoint_s390x.gz internal/mkcw/embed/asm/*.o
	$(MAKE) -C docs clean

.PHONY: docs
docs: install.tools ## build the docs on the host
	$(MAKE) -C docs

codespell:
	codespell -w

.PHONY: validate
validate: all install.tools lint-entrypoint
	./tests/validate/whitespace.sh
	./hack/xref-helpmsgs-manpages
	./tests/validate/pr-should-include-tests

.PHONY: lint-entrypoint
lint-entrypoint: internal/mkcw/embed/entrypoint_amd64.gz
	$(GO) run tests/validate/was-not-go-compiled.go internal/mkcw/embed/entrypoint_amd64.gz

.PHONY: install.tools
install.tools:
	$(MAKE) -C tests/tools

.PHONY: install
install: bin/buildah
	install -d -m 755 $(DESTDIR)/$(BINDIR)
	install -m 755 bin/buildah $(DESTDIR)/$(BINDIR)/buildah
	$(MAKE) -C docs install

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)/$(BINDIR)/buildah
	rm -f $(PREFIX)/share/man/man1/buildah*.1
	rm -f $(DESTDIR)/$(BASHINSTALLDIR)/buildah

.PHONY: install.completions
install.completions:
	install -m 755 -d $(DESTDIR)/$(BASHINSTALLDIR)
	install -m 644 contrib/completions/bash/buildah $(DESTDIR)/$(BASHINSTALLDIR)/buildah

.PHONY: test-conformance
test-conformance: tests/conformance/testdata/mount-targets/true internal/mkcw/embed/entrypoint_amd64.gz
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover -timeout 60m ./tests/conformance

.PHONY: test-integration
test-integration: all install.tools
	cd tests; ./test_runner.sh

tests/testreport/testreport: tests/testreport/testreport.go
	$(GO_BUILD) $(GO_LDFLAGS) "-linkmode external -extldflags -static" -tags "$(STORAGETAGS) $(SECURITYTAGS)" -o tests/testreport/testreport ./tests/testreport/testreport.go

tests/conformance/testdata/mount-targets/true: tests/conformance/testdata/mount-targets/true.go
	$(GO_BUILD) $(GO_LDFLAGS) "-linkmode external -extldflags -static" -o tests/conformance/testdata/mount-targets/true tests/conformance/testdata/mount-targets/true.go

.PHONY: test-unit
test-unit: tests/testreport/testreport internal/mkcw/embed/entrypoint_amd64.gz
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover $(RACEFLAGS) $(shell $(GO) list ./... | grep -v vendor | grep -v tests | grep -v cmd | grep -v chroot | grep -v copier) -timeout 45m
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)"        $(RACEFLAGS) ./chroot ./copier -timeout 60m
	tmp=$(shell mktemp -d) ; \
	mkdir -p $$tmp/root $$tmp/runroot; \
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover $(RACEFLAGS) ./cmd/buildah -args --root $$tmp/root --runroot $$tmp/runroot --storage-driver vfs --signature-policy $(shell pwd)/tests/policy.json --registries-conf $(shell pwd)/tests/registries.conf

vendor-in-container:
	goversion=$(shell sed -e '/^go /!d' -e '/^go /s,.* ,,g' go.mod) ; \
	if test -d `$(GO) env GOCACHE` && test -w `$(GO) env GOCACHE` ; then \
		podman run --privileged --rm --env HOME=/root -v `$(GO) env GOCACHE`:/root/.cache/go-build --env GOCACHE=/root/.cache/go-build -v `pwd`:/src -w /src docker.io/library/golang:$$goversion make vendor ; \
	else \
		podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:$$goversion make vendor ; \
	fi

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify
	if test -n "$(strip $(shell $(GO) env GOTOOLCHAIN))"; then go mod edit -toolchain none ; fi

.PHONY: lint
lint: install.tools internal/mkcw/embed/entrypoint_amd64.gz
	./tests/tools/build/golangci-lint run $(LINTFLAGS)
	./tests/tools/build/golangci-lint run --tests=false $(LINTFLAGS)

.PHONY: fmt
fmt: install.tools
	./tests/tools/build/golangci-lint fmt $(LINTFLAGS)

# CAUTION: This is not a replacement for RPMs provided by your distro.
# Only intended to build and test the latest unreleased changes.
.PHONY: rpm
rpm:  ## Build rpm packages
	$(MAKE) -C rpm
