//go:build freebsd

package define

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "nullfs"
)

// Mount potions for bind
var BindOptions = []string{}
