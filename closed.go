// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/name212/gotee/internal"
)

// ClosedFlag
// Wrapper around atomic.Bool
// for prevent multiple calls for operation
type ClosedFlag struct {
	closed atomic.Bool
}

func NewClosedFlag() *ClosedFlag {
	return &ClosedFlag{}
}

func (c *ClosedFlag) IsClosed() bool {
	return c.closed.Load()
}

// SerClosed
// Set closed flag for true
// and returns true if flag already set to closed
func (c *ClosedFlag) SetClosed() bool {
	shouldClose := c.closed.CompareAndSwap(false, true)
	return !shouldClose
}

type noWriteReaderCleaner struct{}

func (c *noWriteReaderCleaner) GetError(noWait ...bool) error {
	return nil
}

type readerWriterCleaner struct {
	errCh errChan

	*ClosedFlag
	err error

	readers map[string]io.Closer
	writers map[string]io.Closer

	closeReadersWait time.Duration
}

func newReaderWriterCleaner(closeReadersWait time.Duration) *readerWriterCleaner {
	return &readerWriterCleaner{
		closeReadersWait: closeReadersWait,
		ClosedFlag:       NewClosedFlag(),
		errCh:            make(errChan, 1),
		readers:          make(map[string]io.Closer),
		writers:          make(map[string]io.Closer),
	}
}

func (c *readerWriterCleaner) append(name string, reader, writer io.Closer) {
	c.readers[name] = reader
	c.writers[name] = writer
}

func (c *readerWriterCleaner) close() {
	if c.IsClosed() {
		return
	}

	// first close writers
	err := c.closeOnly("writer", c.writers)

	// wait some time for reads
	if c.closeReadersWait > 0 {
		time.Sleep(c.closeReadersWait)
	}

	err = internal.AppendErr(err, c.closeOnly("reader", c.readers))

	c.errCh <- err
	close(c.errCh)
}

func (c *readerWriterCleaner) closeOnly(tp string, closers map[string]io.Closer) error {
	var resErr error
	for name, closer := range closers {
		if err := closer.Close(); err != nil {
			if !pipeClosed(err) {
				resErr = internal.AppendErr(resErr, fmt.Errorf("cannot close %s for %s: %w", tp, name, err))
			}
		}
	}

	return resErr
}

func (c *readerWriterCleaner) GetError(noWait ...bool) error {
	if c.IsClosed() {
		return c.err
	}

	if len(noWait) > 0 && noWait[0] {
		select {
		case c.err = <-c.errCh:
		default:
		}
	} else {
		c.err = <-c.errCh
	}

	c.SetClosed()

	return c.err
}

func pipeClosed(err error) bool {
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	if errors.Is(err, os.ErrClosed) {
		return true
	}

	if errors.Is(err, fs.ErrClosed) {
		return true
	}

	if fsError, ok := err.(*fs.PathError); ok {
		if errors.Is(fsError.Err, fs.ErrClosed) {
			return true
		}
	}

	if osError, ok := err.(*os.PathError); ok {
		if errors.Is(osError.Err, os.ErrClosed) {
			return true
		}
	}

	return false
}
