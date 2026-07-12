//go:build windows

package specgenutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
)

func TestSeccompProfilePath(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	cwd_wsl, err := specgen.ConvertWinMountPath(cwd)
	assert.NoError(t, err)

	tests := []struct {
		originalPath string
		expectedPath string
	}{
		{`C:\Foo`, "/mnt/c/Foo"},
		{`C:\Foo`, "/mnt/c/Foo"},
		{`\\?\C:\Foo`, "/mnt/c/Foo"},
		{`/c/bar`, "/mnt/c/bar"},
		{`/c/bar`, "/mnt/c/bar"},
		{`/mnt/c/bar`, "/mnt/c/bar"},
		{`/test/this`, "/test/this"},
		{`c:/bar/something`, "/mnt/c/bar/something"},
		{`c`, cwd_wsl + "/c"},
		{`\\computer\loc`, `\\computer\loc`},
		{`\\.\drive\loc`, "/mnt/wsl/drive/loc"},
		{"unconfined", "unconfined"},
	}

	f := func(secopt string) (*specgen.SpecGenerator, error) {
		sg := specgen.NewSpecGenerator("nothing", false)
		err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
			SecurityOpt: []string{secopt},
		}, []string{},
		)
		return sg, err
	}

	for _, test := range tests {
		t.Run(test.originalPath, func(t *testing.T) {
			msg := "Checking: " + test.originalPath
			sg, err := f("seccomp=" + test.originalPath)
			assert.NoError(t, err, msg)
			assert.Equal(t, test.expectedPath, sg.SeccompProfilePath, msg)
		})
	}
}
