//go:build linux

package define

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
)

// Mount potions for bind
var BindOptions = []string{TypeBind}
