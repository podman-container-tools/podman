package protectedroot

import (
	"encoding/json"
	"fmt"

	securejoin "github.com/cyphar/filepath-securejoin"
)

type PathRoot struct {
	path string
}

func NewPathRoot(path string) *PathRoot {
	return &PathRoot{path: path}
}

func (pr *PathRoot) Join(untrustedPath string) (string, error) {
	return securejoin.SecureJoin(pr.path, untrustedPath)
}

func (pr *PathRoot) PathWithoutProtection() string {
	return pr.path
}

func (pr *PathRoot) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), pr.path)
}

func (pr *PathRoot) MarshalJSON() ([]byte, error) {
	return json.Marshal(pr.path)
}

func (pr *PathRoot) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &pr.path)
}
