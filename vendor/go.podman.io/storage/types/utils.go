package types

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func expandEnvPath(path string, rootlessUID int) (string, error) {
	var err error
	path = strings.ReplaceAll(path, "$UID", strconv.Itoa(rootlessUID))
	path = os.ExpandEnv(path)
	newpath, err := filepath.EvalSymlinks(path)
	if err != nil {
		newpath = filepath.Clean(path)
	}
	return newpath, nil
}
