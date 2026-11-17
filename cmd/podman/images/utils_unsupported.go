//go:build !linux

package images

func setupPipe() (string, func() <-chan error, error) {
	return "/dev/stdout", nil, nil
}
