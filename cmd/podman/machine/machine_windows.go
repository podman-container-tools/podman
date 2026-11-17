package machine

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func isUnixSocket(file os.DirEntry) bool {
	// Assume a socket on Windows, since sock mode is not supported yet https://github.com/golang/go/issues/33357
	return !file.Type().IsDir() && strings.HasSuffix(file.Name(), ".sock")
}

func rootlessOnly(_ *cobra.Command, _ []string) error {
	// Rootless is not relevant on Windows. In the future rootless.IsRootless
	// could be switched to return true on Windows, and other codepaths migrated

	return nil
}
