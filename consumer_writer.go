// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import "io"

var (
	_ Consumer = &WriteCloserConsumer{}
	_ Consumer = &WriterConsumer{}
)

// WriteCloserConsumer
// Wrapper fot io.WriteCloser
type WriteCloserConsumer struct {
	*privateBaseConsumer
	writer io.WriteCloser
}

func NewWriteCloserConsumer(w io.WriteCloser, name ...string) *WriteCloserConsumer {
	c := &WriteCloserConsumer{
		writer: w,
	}

	c.privateBaseConsumer = newPrivateBaseConsumer(w.Write, name...).withClose(w.Close)
	return c
}

func (c *WriteCloserConsumer) Writer() io.WriteCloser {
	return c.writer
}

// WriterConsumer
// Wrapper fot io.Writer
type WriterConsumer struct {
	*privateBaseConsumer
	writer io.Writer
}

func NewWriterConsumer(w io.Writer, name ...string) *WriterConsumer {
	c := &WriterConsumer{
		writer: w,
	}

	c.privateBaseConsumer = newPrivateBaseConsumer(w.Write, name...)
	return c
}

func (c *WriterConsumer) Writer() io.Writer {
	return c.writer
}
