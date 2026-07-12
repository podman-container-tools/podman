package registry

import (
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var (
	json     jsoniter.API
	jsonSync sync.Once
)

// JSONLibrary provides a "encoding/json" compatible API
func JSONLibrary() jsoniter.API {
	jsonSync.Do(func() {
		json = jsoniter.ConfigCompatibleWithStandardLibrary
	})
	return json
}
