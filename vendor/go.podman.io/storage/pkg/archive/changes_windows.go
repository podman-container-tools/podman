package archive

import (
	"os"

	"go.podman.io/storage/pkg/system"
)

func statDifferent(oldStat *system.StatT, _ *FileInfo, newStat *system.StatT, _ *FileInfo) bool {
	// Don't look at size for dirs, its not a good measure of change
	if oldStat.Mtim() != newStat.Mtim() ||
		oldStat.Mode() != newStat.Mode() ||
		oldStat.Size() != newStat.Size() && !oldStat.Mode().IsDir() {
		return true
	}
	return false
}

func (info *FileInfo) isDir() bool {
	return info.parent == nil || info.stat.Mode().IsDir()
}

func getIno(_ os.FileInfo) uint64 {
	return 0
}

func hasHardlinks(_ os.FileInfo) bool {
	return false
}
