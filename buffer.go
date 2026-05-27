// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"bytes"
	"io"
)

var (
	_ io.Reader = &ClosableReaderBuffer{}
	_ io.Closer = &ClosableReaderBuffer{}
)

type ClosableReaderBuffer struct {
	*bytes.Buffer
	closed *ClosedFlag
}

func NewClosableReaderBuffer(buf *bytes.Buffer) *ClosableReaderBuffer {
	return &ClosableReaderBuffer{
		closed: NewClosedFlag(),
		Buffer: buf,
	}
}

func (b *ClosableReaderBuffer) Read(p []byte) (int, error) {
	if b.closed.IsClosed() {
		return 0, io.EOF
	}

	return b.Buffer.Read(p)
}

func (b *ClosableReaderBuffer) Close() error {
	b.closed.SetClosed()
	return nil
}
