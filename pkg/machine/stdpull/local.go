package stdpull

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/compression"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/storage/pkg/fileutils"
)

type StdDiskPull struct {
	finalPath *define.VMFile
	inputPath *define.VMFile
}

func NewStdDiskPull(inputPath string, finalpath *define.VMFile) (*StdDiskPull, error) {
	ip, err := define.NewMachineFile(inputPath, nil)
	if err != nil {
		return nil, err
	}
	return &StdDiskPull{inputPath: ip, finalPath: finalpath}, nil
}

func (s *StdDiskPull) Get() error {
	if err := fileutils.Exists(s.inputPath.GetPath()); err != nil {
		// could not find disk
		return err
	}
	logrus.Debugf("decompressing (if needed) %s to %s", s.inputPath.GetPath(), s.finalPath.GetPath())
	return compression.Decompress(s.inputPath, s.finalPath.GetPath())
}
