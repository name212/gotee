// Copyright 2026
// license that can be found in the LICENSE file.

package cleaner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/name212/gotee/pkg/internal"
	"github.com/name212/gotee/pkg/tee"
)

type DummyWriteReaderCleaner struct{}

func (c *DummyWriteReaderCleaner) GetError(noWait ...bool) error {
	return nil
}

type ReaderWriterCleaner struct {
	errCh internal.ErrChan

	*tee.ClosedFlag
	err error

	readers map[string]io.Closer
	writers map[string]io.Closer

	closeReadersWait time.Duration
}

func NewReaderWriterCleaner(closeReadersWait time.Duration) *ReaderWriterCleaner {
	return &ReaderWriterCleaner{
		closeReadersWait: closeReadersWait,
		ClosedFlag:       tee.NewClosedFlag(),
		errCh:            make(internal.ErrChan, 1),
		readers:          make(map[string]io.Closer),
		writers:          make(map[string]io.Closer),
	}
}

func (c *ReaderWriterCleaner) Append(name string, reader, writer io.Closer) {
	c.readers[name] = reader
	c.writers[name] = writer
}

func (c *ReaderWriterCleaner) Close() {
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

func (c *ReaderWriterCleaner) closeOnly(tp string, closers map[string]io.Closer) error {
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

func (c *ReaderWriterCleaner) GetError(noWait ...bool) error {
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
