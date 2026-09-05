package autoupdate

// AutoUpdateOptions are the options for running auto-update
//
//go:generate go run ../generator/generator.go AutoUpdateOptions
type AutoUpdateOptions struct {
	// Authfile to use when contacting registries.
	Authfile *string
	// Only check for but do not perform any update.  If an update is
	// pending, it will be indicated in the Updated field of
	// AutoUpdateReport.
	DryRun *bool
	// If restarting the service with the new image failed, restart it
	// another time with the previous image.
	Rollback *bool
	// Allow contacting registries over HTTP, or HTTPS with failed TLS
	// verification. Note that this does not affect other TLS connections.
	InsecureSkipTLSVerify *bool
}
