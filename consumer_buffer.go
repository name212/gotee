// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"bytes"
)

var _ Consumer = &BufferConsumer{}

// BufferConsumer
// Consume all writes to bytes.Buffer
type BufferConsumer struct {
	*privateBaseConsumer
	buf *bytes.Buffer
}

func NewBufferConsumer(buf *bytes.Buffer, name ...string) *BufferConsumer {
	c := &BufferConsumer{
		buf: buf,
	}

	c.privateBaseConsumer = newPrivateBaseConsumer(c.buf.Write, name...)
	return c
}

func NewDefaultBufferConsumer(name ...string) *BufferConsumer {
	c := &BufferConsumer{
		buf: &bytes.Buffer{},
	}

	c.privateBaseConsumer = newPrivateBaseConsumer(c.buf.Write, name...)
	return c
}

func (c *BufferConsumer) Buffer() *bytes.Buffer {
	return c.buf
}
