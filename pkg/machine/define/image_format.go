package define

import "fmt"

type ImageFormat int64

const (
	Qcow ImageFormat = iota
	Vhdx
	Tar
	Raw
)

func (imf ImageFormat) Kind() string {
	switch imf {
	case Vhdx:
		return "vhdx"
	case Tar:
		return "tar"
	case Raw:
		return "raw"
	}
	return "qcow2"
}

func (imf ImageFormat) KindWithCompression() string {
	// All image formats are compressed with zstd
	return fmt.Sprintf("%s.zst", imf.Kind())
}
