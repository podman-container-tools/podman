//go:build tools

package tools

// Importing the packages here will allow to vendor those via
// `go mod vendor`.

import (
	_ "github.com/cpuguy83/go-md2man/v2"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
)
