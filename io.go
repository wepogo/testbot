package testbot

import "io"

type flushWriter struct {
	w io.Writer
	f func()
}

// FlushWriter calls flush after every write.
func FlushWriter(w io.Writer, flush func()) io.Writer {
	return &flushWriter{w, flush}
}

func (w *flushWriter) Write(p []byte) (int, error) {
	defer w.f()
	return w.w.Write(p)
}
