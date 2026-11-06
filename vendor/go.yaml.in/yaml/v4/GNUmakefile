# Copyright 2025 The go-yaml Project Contributors
# SPDX-License-Identifier: Apache-2.0

# Auto-install https://github.com/makeplus/makes at specific commit:
MAKES := .cache/makes
MAKES-LOCAL := .cache/local
MAKES-COMMIT ?= a5085e7f78210be125307728e8cd5665f383c9ad
$(shell [ -d $(MAKES) ] || ( \
  git clone -q https://github.com/makeplus/makes $(MAKES) && \
  git -C $(MAKES) reset -q --hard $(MAKES-COMMIT)))
ifneq ($(shell git -C $(MAKES) rev-parse HEAD), \
       $(shell git -C $(MAKES) rev-parse $(MAKES-COMMIT)))
$(error $(MAKES) is not at the correct commit: $(MAKES-COMMIT). \
	Remove $(MAKES) and try again.)
endif

include $(MAKES)/init.mk
include $(MAKES)/shellcheck.mk
include $(MAKES)/perl.mk
include $(MAKES)/uv.mk
include $(MAKES)/gh.mk

YAMLLINT-VERSION ?= 1.38.0
export UV_CACHE_DIR := $(LOCAL-CACHE)/uv
export UV_PYTHON_INSTALL_DIR := $(LOCAL-ROOT)/uv-python
export UV_TOOL_DIR := $(LOCAL-ROOT)/uv-tools
export UV_TOOL_BIN_DIR := $(LOCAL-BIN)

# Auto-install go unless GO_YAML_PATH is set:
ifdef GO_YAML_PATH
override export PATH := $(GO_YAML_PATH):$(PATH)
else
GO-VERSION ?= 1.27.2
endif
GO-VERSION-NEEDED := $(GO-VERSION)

# yaml-test-suite info:
YTS-URL ?= https://github.com/yaml/yaml-test-suite
YTS-TAG ?= data-2022-01-17
YTS-DIR := yts/testdata/$(YTS-TAG)

CLI-BINARY := go-yaml

# README recording tools are installed only when explicitly requested.
YAMLCAST-COMMIT := 0dfec58fc184e97b21239d0b54dc222bd8311cf1
YAMLCAST-YS-VERSION := 0.2.32
YAMLCAST-URL := https://codeload.github.com/ingydotnet/yamlcast

# Pager for viewing documentation:
PAGER ?= less -FRX

# Setup and include go.mk and shell.mk:

# We need to limit `find` to avoid dirs like `.cache/` and any git worktrees,
# as this makes `make` operations very slow:
REPO-DIRS := $(shell find * -maxdepth 0 -type d ! -name repos \
	       ! -exec test -f {}/.git \; -print)
GO-FILES := $(wildcard *.go) $(shell find $(REPO-DIRS) -name '*.go')

ifndef GO-VERSION-NEEDED
GO-NO-DEP-GO := true
endif

include $(MAKES)/go.mk

YAMLCAST-ROOT := $(LOCAL-ROOT)/yamlcast-$(YAMLCAST-COMMIT)
YAMLCAST := $(YAMLCAST-ROOT)/bin/yamlcast
YAMLCAST-ARCHIVE := $(LOCAL-CACHE)/yamlcast-$(YAMLCAST-COMMIT).tar.gz
YAMLCAST-READY := $(YAMLCAST-ROOT)/.cache/ready-$(YAMLCAST-YS-VERSION)

# Set this from the `make` command to override:
GOLANGCI-LINT-VERSION ?= v2.14.0
GOLANGCI-LINT-INSTALLER := \
  https://github.com/golangci/golangci-lint/raw/main/install.sh
GOLANGCI-LINT := $(LOCAL-BIN)/golangci-lint
GOLANGCI-LINT-VERSIONED := $(GOLANGCI-LINT)-$(GOLANGCI-LINT-VERSION)

SHELL-DEPS += $(GOLANGCI-LINT-VERSIONED)

ifdef GO-VERSION-NEEDED
GO-DEPS += $(GO)
else
SHELL-DEPS := $(filter-out $(GO),$(SHELL-DEPS))
endif

SHELL-NAME := makes go-yaml
include $(MAKES)/clean.mk
include $(MAKES)/shell.mk

MAKES-CLEAN := $(CLI-BINARY) $(GOLANGCI-LINT) *.so
MAKES-REALCLEAN := $(dir $(YTS-DIR))

SHELL-SCRIPTS = \
  util/common.bash \
  cmd/go-yaml/demo/demo-plugins.sh \
  $(shell grep -rl '^.!/usr/bin/env bash' util | \
          grep -v '\.sw')

COVER-TESTS := \
 . \
 ./cmd/... \
 ./internal/... \

# v=1 for verbose tests; release uses v as its version.
MAKE := $(MAKE) --no-print-directory

v ?=
cover ?=
fuzz ?=
time ?= 60s
opts ?=

TEST-OPTS := \
$(if $v, -v)\
$(if $(cover), --cover)\
$(if $(fuzz), --fuzz=FuzzEncodeFromJSON --fuzztime=$(time))\
$(if $(opts), $(opts))\

# Test rules:
test: test-main test-internal test-cmd test-cli-build test-v3-hooks \
  test-yts-all test-shell test-release
	@echo 'ALL TESTS PASS'

check:
	$(MAKE) fmt
	$(MAKE) tidy
	$(MAKE) lint
	$(MAKE) lint-yaml
	$(MAKE) test

test-main: $(GO-DEPS)
	go test .$(TEST-OPTS)
	@echo 'ALL MAIN FILES PASS'

test-cmd: $(GO-DEPS)
	go test ./cmd/...$(TEST-OPTS)
	@echo 'ALL CMD FILES PASS'

test-internal: $(GO-DEPS)
	go test ./internal/...$(TEST-OPTS)
	@echo 'ALL INTERNAL FILES PASS'

test-cover: $(GO-DEPS)
	go test . $(COVER-TESTS) -vet=off --cover$(TEST-OPTS)

test-yts: $(GO-DEPS) $(YTS-DIR)
	go test ./yts$(TEST-OPTS)

test-yts-all: $(GO-DEPS) $(YTS-DIR)
	@echo 'Testing yaml-test-suite'
	util/yaml-test-suite all

test-yts-fail: $(GO-DEPS) $(YTS-DIR)
	@echo 'Testing yaml-test-suite failures'
	util/yaml-test-suite fail

test-shell: $(SHELLCHECK)
	shellcheck $(SHELL-SCRIPTS)
	@echo 'ALL SHELL FILES PASS'

test-count: $(GO-DEPS)
	util/test-count

yts-dir: $(YTS-DIR)

get-test-data: $(YTS-DIR)

# Install golangci-lint for GitHub Actions:
golangci-lint-install: $(GOLANGCI-LINT)

# Keep related checkouts and their fixtures outside core checks.
GO-PACKAGES := . ./cmd/... ./internal/... ./plugin/... ./yts/... ./example/... \
  ./util/build-cli/...
export GOLANGCI_LINT_CACHE := $(CURDIR)/.cache/golangci-lint

fmt: $(GOLANGCI-LINT-VERSIONED)
	$< fmt $(GO-FILES)

lint: $(GOLANGCI-LINT-VERSIONED)
	$< run $(GO-PACKAGES)

lint-yaml: $(UV)
	$(UV) tool run --from yamllint==$(YAMLLINT-VERSION) yamllint .

tidy: $(GO-DEPS)
	go mod tidy

# Pass configuration paths as environment values, not shell source.
export GO_YAML_BUILD_CONFIG = $(CONFIG)
export GO_YAML_BUILD_PLUGIN = $(PLUGIN)
export GO_YAML_BUILD_PLUGIN_PACKAGE = $(PLUGIN-PACKAGE)
export GO_YAML_BUILD_GO = $(GO)
export GO_YAML_BUILD_PERL = $(PERL)
REFERENCE-PARSER-LOCAL ?= 0
export GO_YAML_REFERENCE_PARSER_LOCAL = $(REFERENCE-PARSER-LOCAL)

cli: $(GO)
ifneq ($(strip $(CONFIG)$(PLUGIN)$(PLUGIN-PACKAGE)),)
cli: $(PERL)
	go run ./util/build-cli
else
	go build -o $(CLI-BINARY) ./cmd/go-yaml
endif

$(CLI-BINARY): $(GO)
	go build -o $@ ./cmd/$@

# Create release notes as a draft after the annotated tag is published.
export GO_YAML_RELEASE_VERSION = $(v)

release: $(GH)
	@version=$$GO_YAML_RELEASE_VERSION; \
	part='(0|[1-9][0-9]*)'; \
	[[ $$version =~ ^$$part\.$$part\.$$part$$ ]] || \
	  { echo 'Usage: make release v=4.0.0' >&2; exit 1; }; \
	notes=docs/releases/v$$version.md; \
	test -s "$$notes" || \
	  { echo "Missing or empty release notes: $$notes" >&2; exit 1; }; \
	$(GH) release create "v$$version" \
	  --repo yaml/go-yaml --verify-tag --draft \
	  --title "go-yaml v$$version" --notes-file "$$notes"

test-release: $(PERL)
	$(PERL) util/release.t

# Render the README image with the current CLI and demo sources.
# Chrome's Unix socket path is too long under the project-local TMPDIR.
update: cli yamlcast
	@TMPDIR=/tmp YAMLCAST_ROOT=$(YAMLCAST-ROOT) \
	PATH=$(YAMLCAST-ROOT)/.cache/local/bin:$(PATH) \
	  $(YAMLCAST) docs/demo/cli.yaml

yamlcast: $(YAMLCAST-READY)

$(YAMLCAST-READY): $(YAMLCAST)
	$(MAKE) -C $(YAMLCAST-ROOT) \
	  YAMLSCRIPT-VERSION=$(YAMLCAST-YS-VERSION) shell CMD=true
	touch $@

$(YAMLCAST): $(YAMLCAST-ARCHIVE)
	mkdir -p $(YAMLCAST-ROOT)/.cache
	tar -xzf $< -C $(YAMLCAST-ROOT) --strip-components=1
	ln -sfn $(MAKES) $(YAMLCAST-ROOT)/.cache/makes
	touch $@

$(YAMLCAST-ARCHIVE):
	curl+ $(YAMLCAST-URL)/tar.gz/$(YAMLCAST-COMMIT) > $@.tmp
	mv $@.tmp $@

run-examples: $(GO)
	@for dir in example/*/; do \
	  if test -f "$${dir}go.mod"; then \
	    (set -x; go -C "$$dir" run .) || \
	    { echo "$$dir failed"; break; }; \
	  else \
	    (set -x; go run "$${dir}main.go") || \
	    { echo "$$dir failed"; break; }; \
	  fi; \
	done

# CLI documentation (go doc) - view in terminal:
doc: $(GO-DEPS)
	@go doc -all . | $(PAGER)

# HTTP documentation server - opens browser:
doc-http: $(GO-DEPS)
	go doc -http -all

# Setup rules:
$(YTS-DIR):
	git clone -q $(YTS-URL) $@
	git -C $@ checkout -q $(YTS-TAG)

# Downloads golangci-lint binary and moves to versioned path
# (.cache/local/bin/golangci-lint-<version>).
$(GOLANGCI-LINT-VERSIONED): $(GO-DEPS)
	curl -sSfL $(GOLANGCI-LINT-INSTALLER) | \
	  bash -s -- -b $(LOCAL-BIN) $(GOLANGCI-LINT-VERSION)
	mv $(GOLANGCI-LINT) $@

# Moves golangci-lint-<version> to golangci-lint for CI requirement
$(GOLANGCI-LINT): $(GOLANGCI-LINT-VERSIONED)
	cp $< $@

# Optional plugin dependencies stay outside the core module.
PLUGIN-WORK = $(CURDIR)/.cache/plugin-work/go.work

prepare-reference-parser: $(PERL) $(GO-DEPS)
	GO_YAML_BUILD_REFERENCE_PARSER=true \
	GO_YAML_REFERENCE_PARSER_LOCAL=1 \
	GO_YAML_REFERENCE_PARSER_VERSION=v0.2.6 \
	$(PERL) util/prepare-plugins

test-reference-parser: prepare-reference-parser
	GOWORK=$(PLUGIN-WORK) CGO_ENABLED=0 \
	  go test ./plugin/yamlparser/reference/... \
	  ./.cache/cli-plugins/...$(TEST-OPTS)
	GO_YAML_TEST_REFERENCE_PARSER=1 \
	GO_YAML_REFERENCE_PARSER_LOCAL=1 \
	go test ./util/build-cli \
	  -run TestConfiguredReferenceCLI$(TEST-OPTS)

test-cli-build: $(GO-DEPS)
	go test ./util/build-cli$(TEST-OPTS)

# Real v3 types are confined to a test-only module under testdata.
test-v3-hooks: $(GO-DEPS)
	cd internal/libyaml/testdata/v3hooks && \
	  GOWORK=off go test ./...$(TEST-OPTS)
