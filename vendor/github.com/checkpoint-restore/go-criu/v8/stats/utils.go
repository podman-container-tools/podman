package stats

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
)

func readStatisticsFile(imgDir *os.File, fileName string) (*StatsEntry, error) {
	buf, err := os.ReadFile(filepath.Join(imgDir.Name(), fileName))
	if err != nil {
		return nil, err
	}

	if binary.LittleEndian.Uint32(buf[PrimaryMagicOffset:SecondaryMagicOffset]) != ImgServiceMagic {
		return nil, errors.New("primary magic not found")
	}

	if binary.LittleEndian.Uint32(buf[SecondaryMagicOffset:SizeOffset]) != StatsMagic {
		return nil, errors.New("secondary magic not found")
	}

	payloadSize := binary.LittleEndian.Uint32(buf[SizeOffset:PayloadOffset])

	st := &StatsEntry{}
	if err := st.UnmarshalVT(buf[PayloadOffset : PayloadOffset+payloadSize]); err != nil {
		return nil, err
	}

	return st, nil
}

func CriuGetDumpStats(imgDir *os.File) (*DumpStatsEntry, error) {
	st, err := readStatisticsFile(imgDir, StatsDump)
	if err != nil {
		return nil, err
	}

	return st.GetDump(), nil
}

func CriuGetRestoreStats(imgDir *os.File) (*RestoreStatsEntry, error) {
	st, err := readStatisticsFile(imgDir, StatsRestore)
	if err != nil {
		return nil, err
	}

	return st.GetRestore(), nil
}
