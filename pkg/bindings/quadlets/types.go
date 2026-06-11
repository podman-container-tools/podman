package quadlets

// ListOptions are optional options for listing quadlets
//
//go:generate go run ../generator/generator.go ListOptions
type ListOptions struct {
	Filters map[string][]string
}

// ExistsOptions are optional options for checking if a quadlet exists
//
//go:generate go run ../generator/generator.go ExistsOptions
type ExistsOptions struct{}

// PrintOptions are optional options for printing quadlet file contents
//
//go:generate go run ../generator/generator.go PrintOptions
type PrintOptions struct{}

// InstallOptions are optional options for installing quadlets
//
//go:generate go run ../generator/generator.go InstallOptions
type InstallOptions struct {
	Replace       *bool
	ReloadSystemd *bool
}

// RemoveOptions are optional options for removing quadlets
//
//go:generate go run ../generator/generator.go RemoveOptions
type RemoveOptions struct {
	Force         *bool
	All           *bool
	Ignore        *bool
	ReloadSystemd *bool
}
