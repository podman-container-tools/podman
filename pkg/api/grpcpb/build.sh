#!/bin/bash
set -e
cd $(dirname ${BASH_SOURCE[0]})
TOP=../../..
PATH=${TOP}/test/tools/build:${PATH}
set -x
for proto in *.proto ; do
	protoc \
		--go_opt=paths=source_relative --go_out . \
		--go-grpc_opt=paths=source_relative --go-grpc_out . \
	${proto}
done

# Format generated files
gofumpt -w -- *.pb.go
