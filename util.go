package xflags

import (
	"io"
)

type aggregatedWriter struct {
	w   io.Writer
	n   int64
	err error
}

func newAggregatedWriter(w io.Writer) *aggregatedWriter {
	if ag, ok := w.(*aggregatedWriter); ok {
		return ag
	}
	return &aggregatedWriter{w: w}
}

func (w *aggregatedWriter) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err = w.w.Write(p)
	w.n += int64(n)
	w.err = err
	return
}

func (w *aggregatedWriter) N() int64                     { return w.n }
func (w *aggregatedWriter) Err() error                   { return w.err }
func (w *aggregatedWriter) Result() (n int64, err error) { return w.n, w.err }
