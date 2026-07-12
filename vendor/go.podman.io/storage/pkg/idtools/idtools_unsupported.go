//go:build !linux || !libsubid || !cgo

package idtools

func readSubuid(username string) ([]subIDRange, error) {
	return parseSubidFile(subuidFileName, username)
}

func readSubgid(username string) ([]subIDRange, error) {
	return parseSubidFile(subgidFileName, username)
}
