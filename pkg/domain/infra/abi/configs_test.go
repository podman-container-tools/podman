//go:build !remote && (linux || freebsd)

package abi

import (
	"errors"
	"io"
	"strings"
	"testing"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

type configTestReader struct {
	remaining int
	read      int
}

func (r *configTestReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	clear(p[:n])
	r.remaining -= n
	r.read += n
	return n, nil
}

func TestReadConfigData(t *testing.T) {
	t.Run("accepts data below limit", func(t *testing.T) {
		input := strings.Repeat("x", entities.MaxConfigSize-1)
		data, err := readConfigData(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != entities.MaxConfigSize-1 {
			t.Fatalf("expected %d bytes, got %d", entities.MaxConfigSize-1, len(data))
		}
	})

	for _, size := range []int{entities.MaxConfigSize, entities.MaxConfigSize + 1024} {
		t.Run("rejects data at or above limit", func(t *testing.T) {
			input := &configTestReader{remaining: size}
			_, err := readConfigData(input)
			if !errors.Is(err, entities.ErrConfigDataTooLarge) {
				t.Fatalf("expected ErrConfigDataTooLarge, got %v", err)
			}
			if input.read != entities.MaxConfigSize {
				t.Fatalf("read %d bytes, want limit of %d", input.read, entities.MaxConfigSize)
			}
		})
	}
}
