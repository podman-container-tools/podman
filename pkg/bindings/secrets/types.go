package secrets

// ListOptions are optional options for inspecting secrets
//
//go:generate go run ../generator/generator.go ListOptions
type ListOptions struct {
	Filters map[string][]string
}

// InspectOptions are optional options for inspecting secrets
//
//go:generate go run ../generator/generator.go InspectOptions
type InspectOptions struct {
	ShowSecret *bool
}

// RemoveOptions are optional options for removing secrets
//
//go:generate go run ../generator/generator.go RemoveOptions
type RemoveOptions struct{}

// CreateOptions are optional options for Creating secrets
//
//go:generate go run ../generator/generator.go CreateOptions
type CreateOptions struct {
	Name       *string
	Driver     *string
	DriverOpts map[string]string
	Labels     map[string]string
	Replace    *bool
	Ignore     *bool
}
