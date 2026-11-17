package compat

// We import `moby/moby/api` here to pull-in vendor/github.com/moby/moby/api/swagger.yaml.
// It is used by our tests and without using it from some .go file, the `go mod vendor`
// will not pull it in.
import _ "github.com/moby/moby/api"
