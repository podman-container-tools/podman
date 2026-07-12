//go:build !amd64 && !arm64

package main

func getProvider() (string, error) {
	return "", nil
}
