.PHONY: all
all: check

.PHONY: check
check: lint build test check-symbols

# Previous bugs have made the tests freeze until the timeout. Golang default
# timeout for tests is 10 minutes, which is too long, considering current tests
# can be executed in less than 1 second. Reduce the timeout, so problems can
# be noticed earlier in the CI.
TEST_TIMEOUT=10s

.PHONY: test
test: test-build test-run

.PHONY: test-build
test-build:
	go test -c

.PHONY: test-run
test-run:
	./libseccomp-golang.test -test.v -test.timeout $(TEST_TIMEOUT)

# All libseccomp functions available in v2.3.1, the minimum version supported
# by this package. Since these are always present, they are the only ones that
# may be referenced strongly; anything added later has to be referenced weakly
# and called via a compat_* wrapper, so that a binary compiled against a newer
# libseccomp can still be loaded and used with an older run-time library. See
# seccomp_compat.h for details.
LIBSECCOMP_V231_SYMBOLS = \
	seccomp_arch_add \
	seccomp_arch_exist \
	seccomp_arch_native \
	seccomp_arch_remove \
	seccomp_attr_get \
	seccomp_attr_set \
	seccomp_export_bpf \
	seccomp_export_pfc \
	seccomp_init \
	seccomp_load \
	seccomp_merge \
	seccomp_release \
	seccomp_reset \
	seccomp_rule_add_array \
	seccomp_rule_add_exact_array \
	seccomp_syscall_priority \
	seccomp_syscall_resolve_name \
	seccomp_syscall_resolve_name_arch \
	seccomp_syscall_resolve_num_arch \
	seccomp_version

.PHONY: check-symbols
check-symbols: test-build
	@# A strong reference to a symbol the run-time library does not have makes
	@# the binary fail to load, so anything post-v2.3.1 has to stay weak.
	@command -v nm >/dev/null 2>&1 || { echo "nm not found; skipping symbol check"; exit 0; }
	@strong=$$(nm -D --undefined-only ./libseccomp-golang.test \
		| awk '$$1 == "U" && $$2 ~ /^seccomp_/ { print $$2 }' \
		| grep -vx $(patsubst %,-e %,$(LIBSECCOMP_V231_SYMBOLS)) || true); \
	if [ -n "$$strong" ]; then \
		echo "Error: strong references to libseccomp symbols added after v2.3.1:"; \
		echo "$$strong" | sed 's/^/	/'; \
		echo "Declare them weak and call them via a compat_* wrapper (see seccomp_compat.h)."; \
		exit 1; \
	fi
	@# A weak reference stays weak no matter who calls it, so the check above
	@# can not see a direct call from Go, which would crash on NULL rather
	@# than fail to load. Only seccomp_compat.c may call these directly, and
	@# only after checking for NULL.
	@direct=$$(grep -n -o 'C\.seccomp_[a-z0-9_]*' *.go \
		| sed 's/C\.//' \
		| grep -v $(patsubst %,-e ':%$$',$(LIBSECCOMP_V231_SYMBOLS)) || true); \
	if [ -n "$$direct" ]; then \
		echo "Error: direct calls to libseccomp symbols added after v2.3.1:"; \
		echo "$$direct" | sed 's/^/	/'; \
		echo "Call them via a compat_* wrapper instead (see seccomp_compat.h)."; \
		exit 1; \
	fi

.PHONY: lint
lint:
	golangci-lint run

.PHONY: build
build:
	go build
