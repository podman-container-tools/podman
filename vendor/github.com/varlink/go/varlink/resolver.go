package varlink

import "context"

// ResolverAddress is the well-known address of the varlink interface resolver,
// it translates varlink interface names to varlink service addresses.
const ResolverAddress = "unix:/run/org.varlink.resolver"

// Resolver resolves varlink interface names to varlink addresses
type Resolver struct {
	address string
	conn    *Connection
}

// Resolve resolves a varlink interface name to a varlink address.
func (r *Resolver) Resolve(ctx context.Context, iface string) (string, error) {
	type request struct {
		Interface string `json:"interface"`
	}
	type reply struct {
		Address string `json:"address"`
	}

	/* don't ask the resolver for itself */
	if iface == "org.varlink.resolver" {
		return r.address, nil
	}

	var rep reply
	err := r.conn.Call(ctx, "org.varlink.resolver.Resolve", &request{Interface: iface}, &rep)
	if err != nil {
		return "", err
	}

	return rep.Address, nil
}

// GetInfo requests information about the resolver.
func (r *Resolver) GetInfo(ctx context.Context, vendor *string, product *string, version *string, url *string, interfaces *[]string) error {
	type reply struct {
		Vendor     string
		Product    string
		Version    string
		URL        string
		Interfaces []string
	}

	var rep reply
	err := r.conn.Call(ctx, "org.varlink.resolver.GetInfo", nil, &rep)
	if err != nil {
		return err
	}

	if vendor != nil {
		*vendor = rep.Vendor
	}
	if product != nil {
		*product = rep.Product
	}
	if version != nil {
		*version = rep.Version
	}
	if url != nil {
		*url = rep.URL
	}
	if interfaces != nil {
		*interfaces = rep.Interfaces
	}

	return nil
}

// Close terminates the resolver.
func (r *Resolver) Close() error {
	return r.conn.Close()
}

// NewResolver returns a new resolver connected to the given address.
func NewResolver(ctx context.Context, address string) (*Resolver, error) {
	if address == "" {
		address = ResolverAddress
	}

	c, err := NewConnection(ctx, address)
	if err != nil {
		return nil, err
	}
	r := Resolver{
		address: address,
		conn:    c,
	}

	return &r, nil
}
