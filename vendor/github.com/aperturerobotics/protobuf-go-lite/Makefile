SHELL:=bash
PROTOWRAP=hack/bin/protowrap
GOIMPORTS=hack/bin/goimports
GOFUMPT=hack/bin/gofumpt
GOLANGCI_LINT=hack/bin/golangci-lint
PROTOC_GEN_GO=hack/bin/protoc-gen-go-lite
GOLIST=go list -f "{{ .Dir }}" -m

export GO111MODULE=on
undefine GOOS
undefine GOARCH

all:

vendor:
	go mod vendor

$(GOIMPORTS):
	cd ./hack; \
	go build -v \
		-o ./bin/goimports \
		golang.org/x/tools/cmd/goimports

$(GOFUMPT):
	cd ./hack; \
	go build -v \
		-o ./bin/gofumpt \
		mvdan.cc/gofumpt

$(PROTOWRAP):
	cd ./hack; \
	go build -v \
		-o ./bin/protowrap \
		github.com/aperturerobotics/goprotowrap/cmd/protowrap

$(GOLANGCI_LINT):
	cd ./hack; \
	go build -v \
		-o ./bin/golangci-lint \
		github.com/golangci/golangci-lint/cmd/golangci-lint

$(PROTOC_GEN_GO):
	cd ./hack; \
	go build -v \
		-o ./bin/protoc-gen-go-lite \
		github.com/aperturerobotics/protobuf-go-lite/cmd/protoc-gen-go-lite

.PHONY: protoc-gen-go-lite-tool
protoc-gen-go-lite-tool:
	cd ./hack; \
	go build -v \
		-o ./bin/protoc-gen-go-lite \
		github.com/aperturerobotics/protobuf-go-lite/cmd/protoc-gen-go-lite

.PHONY: build
build: vendor
	go build -v

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --timeout=10m

.PHONY: fix
fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix --timeout=10m

.PHONY: format
format: $(GOFUMPT) $(GOIMPORTS)
	$(GOIMPORTS) -w ./
	$(GOFUMPT) -w ./

.PHONY: test
test:
	go test -v ./...

.PHONY: gengo
gengo: $(GOIMPORTS) $(PROTOWRAP) protoc-gen-go-lite-tool
	shopt -s globstar; \
	set -eo pipefail; \
	export PROJECT=$$(go list -m); \
	export PATH=$$(pwd)/hack/bin:$${PATH}; \
	mkdir -p $$(pwd)/vendor/$$(dirname $${PROJECT}); \
	rm $$(pwd)/vendor/$${PROJECT} || true; \
	ln -s $$(pwd) $$(pwd)/vendor/$${PROJECT} ; \
	protogen() { \
		$(PROTOWRAP) \
			-I $$(pwd)/vendor \
			-I $$(pwd) \
			--go-lite_out=$$(pwd)/vendor \
			--proto_path $$(pwd)/vendor \
			--print_structure \
			--only_specified_files \
			$$2 \
			$$(\
				git \
					ls-files "$$1" |\
					xargs printf -- \
					"$$(pwd)/vendor/$${PROJECT}/%s "); \
	}; \
	for d in ./types/known/*; do \
		protogen "$${d}/*.proto" "--go-lite_opt=features=marshal+marshal_strict+unmarshal+unmarshal_unsafe+size+equal+clone+text"; \
	done; \
	protogen "./types/descriptorpb/*.proto" ""; \
	protogen "./types/pluginpb/*.proto" ""; \
	protogen "./testproto/*.proto" ""; \
	rm $$(pwd)/vendor/$${PROJECT} || true
	$(GOIMPORTS) -w ./

.PHONY: check-gengo
check-gengo: gengo
	git diff --exit-code HEAD
	test -z "$$(git ls-files --others --exclude-standard)" || (git ls-files --others --exclude-standard; exit 1)
