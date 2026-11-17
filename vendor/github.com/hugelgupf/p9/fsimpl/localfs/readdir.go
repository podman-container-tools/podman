package localfs

import (
	"io"
	"path"

	"github.com/hugelgupf/p9/p9"
)

// Readdir implements p9.File.Readdir.
func (l *Local) Readdir(offset uint64, count uint32) (p9.Dirents, error) {
	var (
		p9Ents = make([]p9.Dirent, 0)
		cursor = uint64(0)
	)

	for len(p9Ents) < int(count) {
		singleEnt, err := l.file.Readdirnames(1)

		if err == io.EOF {
			return p9Ents, nil
		} else if err != nil {
			return nil, err
		}

		// we consumed an entry
		cursor++

		// cursor \in (offset, offset+count)
		if cursor < offset || cursor > offset+uint64(count) {
			continue
		}

		name := singleEnt[0]

		localEnt := Local{path: path.Join(l.path, name)}
		qid, _, err := localEnt.info()
		if err != nil {
			return p9Ents, err
		}
		p9Ents = append(p9Ents, p9.Dirent{
			QID:    qid,
			Type:   qid.Type,
			Name:   name,
			Offset: cursor,
		})
	}

	return p9Ents, nil
}
