//go:build !linux

package archive

func getWhiteoutConverter(_ WhiteoutFormat, _ any, _ *TarOptions) tarWhiteoutConverter {
	return nil
}

func GetFileOwner(path string) (uint32, uint32, uint32, error) {
	return 0, 0, 0, nil
}
