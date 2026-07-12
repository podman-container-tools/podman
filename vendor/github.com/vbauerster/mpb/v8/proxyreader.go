package mpb

import (
	"io"
	"time"
)

type proxyReadCloser struct {
	r   io.Reader
	bar *Bar
}

func (x proxyReadCloser) Read(p []byte) (int, error) {
	n, err := x.r.Read(p)
	x.bar.IncrBy(n)
	return n, err
}

func (x proxyReadCloser) Close() error {
	if rc, ok := x.r.(io.ReadCloser); ok {
		return rc.Close()
	}
	return nil
}

type proxyWriterTo struct {
	proxyReadCloser
	wt io.WriterTo
}

func (x proxyWriterTo) WriteTo(w io.Writer) (int64, error) {
	return x.wt.WriteTo(proxyWriteCloser{w, x.bar})
}

type ewmaProxyReadCloser struct {
	r   io.Reader
	bar *Bar
}

func (x ewmaProxyReadCloser) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := x.r.Read(p)
	x.bar.EwmaIncrBy(n, time.Since(start))
	return n, err
}

func (x ewmaProxyReadCloser) Close() error {
	if rc, ok := x.r.(io.ReadCloser); ok {
		return rc.Close()
	}
	return nil
}

type ewmaProxyWriterTo struct {
	ewmaProxyReadCloser
	wt io.WriterTo
}

func (x ewmaProxyWriterTo) WriteTo(w io.Writer) (int64, error) {
	return x.wt.WriteTo(ewmaProxyWriteCloser{w, x.bar})
}

func newProxyReader(r io.Reader, b *Bar, hasEwma bool) io.ReadCloser {
	if hasEwma {
		epr := ewmaProxyReadCloser{r, b}
		if wt, ok := r.(io.WriterTo); ok {
			return ewmaProxyWriterTo{epr, wt}
		}
		return epr
	}
	pr := proxyReadCloser{r, b}
	if wt, ok := r.(io.WriterTo); ok {
		return proxyWriterTo{pr, wt}
	}
	return pr
}
