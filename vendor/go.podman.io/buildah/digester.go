package buildah

import (
	"archive/tar"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	digest "github.com/opencontainers/go-digest"
)

// tarFilterAction is returned by a newTarFilterer() filter callback to tell
// the filterer what to do with the archive entry it was passed.
type tarFilterAction int

const (
	// tarFilterKeep tells the filterer to emit the entry right away.
	tarFilterKeep tarFilterAction = iota
	// tarFilterSkip tells the filterer to skip the entry.
	tarFilterSkip
	// tarFilterDefer tells the filterer to defer the entry, and emit it right
	// before its unfiltered descendant entry, if any. Only supported for
	// entries without content.
	tarFilterDefer
)

type digester interface {
	io.WriteCloser
	ContentType() string
	Digest() digest.Digest
}

// A simple digester just digests its content as-is.
type simpleDigester struct {
	digester    digest.Digester
	hasher      hash.Hash
	contentType string
}

func newSimpleDigester(contentType string) digester {
	finalDigester := digest.Canonical.Digester()
	return &simpleDigester{
		digester:    finalDigester,
		hasher:      finalDigester.Hash(),
		contentType: contentType,
	}
}

func (s *simpleDigester) ContentType() string {
	return s.contentType
}

func (s *simpleDigester) Write(p []byte) (int, error) {
	return s.hasher.Write(p)
}

func (s *simpleDigester) Close() error {
	return nil
}

func (s *simpleDigester) Digest() digest.Digest {
	return s.digester.Digest()
}

// A tarFilterer passes a tarball through to an io.WriteCloser, potentially
// modifying headers as it goes.
type tarFilterer struct {
	wg         sync.WaitGroup
	pipeWriter *io.PipeWriter
	closedLock sync.Mutex
	closed     bool
	err        error
}

func (t *tarFilterer) Write(p []byte) (int, error) {
	n, err := t.pipeWriter.Write(p)
	if err != nil {
		t.closedLock.Lock()
		closed := t.closed
		t.closedLock.Unlock()
		err = fmt.Errorf("writing to tar filter pipe (closed=%v,err=%v): %w", closed, t.err, err)
	}
	return n, err
}

func (t *tarFilterer) Close() error {
	t.closedLock.Lock()
	if t.closed {
		t.closedLock.Unlock()
		return errors.New("tar filter is already closed")
	}
	t.closed = true
	t.closedLock.Unlock()
	err := t.pipeWriter.Close()
	t.wg.Wait()
	if err != nil {
		return fmt.Errorf("closing filter pipe: %w", err)
	}
	return t.err
}

// newTarFilterer passes one or more tar archives through to an io.WriteCloser
// as a single archive, potentially calling filter to modify headers and
// contents as it goes.
//
// Note: if "filter" indicates that a given item should be skipped, there is no
// guarantee that there will not be a subsequent item of type TypeLink, which
// is a hard link, which points to the skipped item as the link target.
func newTarFilterer(writeCloser io.WriteCloser, filter func(hdr *tar.Header) (action tarFilterAction, replaceContents bool, replacementContents io.Reader)) io.WriteCloser {
	pipeReader, pipeWriter := io.Pipe()
	tarWriter := tar.NewWriter(writeCloser)
	filterer := &tarFilterer{
		pipeWriter: pipeWriter,
	}
	filterer.wg.Go(func() {
		filterer.closedLock.Lock()
		closed := filterer.closed
		filterer.closedLock.Unlock()
		var deferred []*tar.Header
		var tarReader *tar.Reader
		writeEntry := func(hdr *tar.Header, replaceContents bool, replacementContents io.Reader) error {
			if err := tarWriter.WriteHeader(hdr); err != nil {
				return fmt.Errorf("writing tar header for %q: %w", hdr.Name, err)
			}
			if hdr.Size != 0 {
				var n int64
				var copyErr error
				if replaceContents {
					n, copyErr = io.CopyN(tarWriter, replacementContents, hdr.Size)
				} else {
					n, copyErr = io.Copy(tarWriter, tarReader)
				}
				if copyErr != nil {
					return fmt.Errorf("copying content for %q: %w", hdr.Name, copyErr)
				}
				if n != hdr.Size {
					return fmt.Errorf("filtering content for %q: expected %d bytes, got %d bytes", hdr.Name, hdr.Size, n)
				}
			}
			if err := tarWriter.Flush(); err != nil {
				return fmt.Errorf("flushing tar item padding for %q: %w", hdr.Name, err)
			}
			return nil
		}
		for !closed {
			tarReader = tar.NewReader(pipeReader)
			hdr, err := tarReader.Next()
			for err == nil {
				action := tarFilterKeep
				var replaceContents bool
				var replacementContents io.Reader
				if filter != nil {
					action, replaceContents, replacementContents = filter(hdr)
				}
				switch action {
				case tarFilterDefer:
					// Deferring is intended for parents, and parents do
					// not have content of their own. If the entry had
					// content, it would be necessary to store it alongside
					// the header, otherwise the parent header would be
					// incorrectly attributed to a further entry. But since
					// we do not have use for that functionality, we error.
					if hdr.Size != 0 {
						err = fmt.Errorf("filter deferred %q, which has a non-zero size (%d)", hdr.Name, hdr.Size)
						break
					}
					hdrCopy := *hdr
					deferred = append(deferred, &hdrCopy)
				case tarFilterKeep:
					// Emit deferred ancestors before the child
					// because tar extractors create missing parent
					// directories with default ownership otherwise.
					nameSpec := path.Clean(strings.TrimRight(hdr.Name, "/"))
					var remaining []*tar.Header
					for _, d := range deferred {
						deferredName := path.Clean(strings.TrimRight(d.Name, "/"))
						if strings.HasPrefix(nameSpec, deferredName+"/") {
							if err = writeEntry(d, false, nil); err != nil {
								break
							}
						} else {
							remaining = append(remaining, d)
						}
					}
					deferred = remaining

					// Emit the child.
					if err == nil {
						err = writeEntry(hdr, replaceContents, replacementContents)
					}
				}
				if err != nil {
					break
				}
				hdr, err = tarReader.Next()
			}
			deferred = nil
			if !errors.Is(err, io.EOF) {
				filterer.err = fmt.Errorf("reading tar archive: %w", err)
				break
			}
			filterer.closedLock.Lock()
			closed = filterer.closed
			filterer.closedLock.Unlock()
		}
		err1 := tarWriter.Close()
		err := writeCloser.Close()
		if err == nil {
			err = err1
		}
		if err != nil {
			pipeReader.CloseWithError(err)
		} else {
			pipeReader.Close()
		}
	})
	return filterer
}

// A tar digester digests an archive, modifying the headers it digests by
// calling a specified function to potentially modify the header that it's
// about to write.
type tarDigester struct {
	isOpen      bool
	nested      digester
	tarFilterer io.WriteCloser
}

func modifyTarHeaderForDigesting(hdr *tar.Header) (action tarFilterAction, replaceContents bool, replacementContents io.Reader) {
	zeroTime := time.Time{}
	hdr.ModTime = zeroTime
	hdr.AccessTime = zeroTime
	hdr.ChangeTime = zeroTime
	return tarFilterKeep, false, nil
}

func newTarDigester(contentType string) digester {
	nested := newSimpleDigester(contentType)
	digester := &tarDigester{
		isOpen:      true,
		nested:      nested,
		tarFilterer: newTarFilterer(nested, modifyTarHeaderForDigesting),
	}
	return digester
}

func (t *tarDigester) ContentType() string {
	return t.nested.ContentType()
}

func (t *tarDigester) Digest() digest.Digest {
	return t.nested.Digest()
}

func (t *tarDigester) Write(p []byte) (int, error) {
	return t.tarFilterer.Write(p)
}

func (t *tarDigester) Close() error {
	if t.isOpen {
		t.isOpen = false
		return t.tarFilterer.Close()
	}
	return nil
}

// CompositeDigester can compute a digest over multiple items.
type CompositeDigester struct {
	digesters []digester
	closer    io.Closer
}

// closeOpenDigester closes an open sub-digester, if we have one.
func (c *CompositeDigester) closeOpenDigester() {
	if c.closer != nil {
		c.closer.Close()
		c.closer = nil
	}
}

// Restart clears all state, so that the composite digester can start over.
func (c *CompositeDigester) Restart() {
	c.closeOpenDigester()
	c.digesters = nil
}

// Start starts recording the digest for a new item ("", "file", or "dir").
// The caller should call Hash() immediately after to retrieve the new
// io.WriteCloser.
func (c *CompositeDigester) Start(contentType string) {
	c.closeOpenDigester()
	switch contentType {
	case "":
		c.digesters = append(c.digesters, newSimpleDigester(""))
	case "file", "dir":
		digester := newTarDigester(contentType)
		c.closer = digester
		c.digesters = append(c.digesters, digester)
	default:
		panic(fmt.Sprintf(`unrecognized content type: expected "", "file", or "dir", got %q`, contentType))
	}
}

// Hash returns the hasher for the current item.
func (c *CompositeDigester) Hash() io.WriteCloser {
	num := len(c.digesters)
	if num == 0 {
		return nil
	}
	return c.digesters[num-1]
}

// Digest returns the content type and a composite digest over everything
// that's been digested.
func (c *CompositeDigester) Digest() (string, digest.Digest) {
	c.closeOpenDigester()
	num := len(c.digesters)
	switch num {
	case 0:
		return "", ""
	case 1:
		return c.digesters[0].ContentType(), c.digesters[0].Digest()
	default:
		var content strings.Builder
		for i, digester := range c.digesters {
			if i > 0 {
				content.WriteString(",")
			}
			contentType := digester.ContentType()
			if contentType != "" {
				contentType += ":"
			}
			content.WriteString(contentType + digester.Digest().Encoded())
		}
		return "multi", digest.Canonical.FromString(content.String())
	}
}
