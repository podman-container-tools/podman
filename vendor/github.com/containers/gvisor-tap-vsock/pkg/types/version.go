package types

import (
	"flag"
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	// set using the '-X github.com/containers/gvisor-tap-vsock/pkg/types.gitVersion' linker flag
	gitVersion = ""
	// set through .gitattributes when `git archive` is used
	// see https://icinga.com/blog/2022/05/25/embedding-git-commit-information-in-go-binaries/
	gitArchiveVersion = "$Format:%(describe)$"
)

type version struct {
	binaryName  string
	showVersion bool
}

func NewVersion(binaryName string) *version { //nolint:revive
	return &version{
		binaryName: binaryName,
	}
}

func (ver *version) String() string {
	return fmt.Sprintf("%s version %s", ver.binaryName, moduleVersion())
}

func (ver *version) AddFlag() {
	flag.BoolVar(&ver.showVersion, "version", false, "Print version information")
}

func (ver *version) ShowVersion() bool {
	return ver.showVersion
}

func moduleVersion() string {
	switch {
	// This will be substituted when building from a GitHub tarball
	case !strings.HasPrefix(gitArchiveVersion, "$Format:"):
		return gitArchiveVersion
	// This will be set when building from git using make
	case gitVersion != "":
		if !strings.HasPrefix(gitVersion, "v") {
			// if an annotated tag is found, the git describe string will be similar to:
			// v0.7.2-15-g2c897d90
			// When using shallow clones, the commit being built
			// may not have an annotated tag in its history,
			// `git describe` will only be the abbreviated commit hash in this case:
			// 2c897d90
			return fmt.Sprintf("git%s", gitVersion)

		}
		return gitVersion
	// moduleVersionFromBuildInfo() will be set when using `go install`
	default:
		return moduleVersionFromBuildInfo()
	}
}

func moduleVersionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}
