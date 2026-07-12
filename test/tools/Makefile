GO := go
GO_BUILD=$(GO) build

BUILDDIR := build

SOURCES := $(shell find . -name '*.go')

all: $(BUILDDIR)

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify
	$(GO) mod edit -toolchain none

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

.PHONY: $(BUILDDIR)
$(BUILDDIR): \
	$(BUILDDIR)/go-md2man \
	$(BUILDDIR)/swagger

$(BUILDDIR)/go-md2man: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/github.com/cpuguy83/go-md2man/v2

$(BUILDDIR)/swagger: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/github.com/go-swagger/go-swagger/cmd/swagger
