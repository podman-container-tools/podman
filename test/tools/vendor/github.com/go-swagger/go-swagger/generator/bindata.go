// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"embed"
	"io/fs"
)

//go:embed templates
var _bindata embed.FS

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0)
	_ = fs.WalkDir(_bindata, "templates", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		names = append(names, path)
		return nil
	})
	return names
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	return _bindata.ReadFile(name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}
