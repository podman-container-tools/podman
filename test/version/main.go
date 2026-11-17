package main

import (
	"fmt"

	"go.podman.io/podman/v6/version"
)

func main() {
	fmt.Print(version.Version.String())
}
