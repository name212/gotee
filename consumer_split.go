// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"bufio"

	"github.com/name212/gotee/internal"
	"github.com/name212/gotee/scan"
)

var (
	_ Consumer = &SplitConsumer{}
)

// PartsHandler
// interface for handle splited parts by SplitConsumer
type PartsHandler interface {
	// Handle
	// handle splited parts by SplitConsumer
	// last is true when split function return bufio.ErrFinalToken
	// scanErr - if scan returns error
	// unhandled - is true if receive last token or close operation
	// but scanner has unhandled data. because use non-block scanner
	// we scanner has not split data in own store
	// and you can decide handle this or not by unhandled flag
	Handle(part []byte, unhandled, last, scanErr bool) error
}

// SplitConsumer
// Consummer that split collect inputs, split
// by bufio.SplitFunc and pass to PartsHandler
// This consumer use none-block scan.NonBlockScanner
// and will not create gouritines.
// If got last token from scanner (bufio.SplitFunc returns bufio.ErrFinalToken)
// consumer returns ErrClosed from Write call
// WARNING after close consumer PartsHandler can get unhandled bytes from scanner
// WARNING! By default consumer not copy input
// For copy input before call function
// use WithCopyInput(true) after create consumer
type SplitConsumer struct {
	*privateBaseConsumer

	flushed *ClosedFlag

	scanner *scan.NonBlockScanner
	handler *scannerHandler
}

func NewSplitConsumer(split bufio.SplitFunc, handler PartsHandler, name ...string) *SplitConsumer {
	if internal.IsNil(split) {
		split = bufio.ScanLines
	}

	tokenHandler := newScannerHandler(handler)

	scanner := scan.NewNonBlockScanner(tokenHandler)
	scanner.Split(split)

	c := &SplitConsumer{
		scanner:             scanner,
		handler:             tokenHandler,
		flushed:             NewClosedFlag(),
	}

	c.privateBaseConsumer = newPrivateBaseConsumer(c.write, name...).withClose(c.close)
	return c
}

func (c *SplitConsumer) write(input []byte) (int, error) {
	receiveLastToken, scanErr := c.scanner.Scan(input)
	if err := c.handler.getErr(); err != nil {
		return 0, err
	}

	hasScanErr := !internal.IsNil(scanErr)
	if receiveLastToken || hasScanErr {
		const isLast = true
		flushErr := internal.AppendErr(scanErr, c.flush(isLast, internal.IsNil(scanErr)))
		// not handle error because already flush
		_ = c.Close()
		l := 0
		if !hasScanErr {
			l = len(input)
		}
		return l, internal.AppendErr(flushErr, ErrClosed)
	}

	return len(input), nil
}

func (c *SplitConsumer) flush(last bool, scanError bool) error {
	if c.flushed.SetClosed() {
		return nil
	}

	unhandled := c.scanner.Unhandled()
	if len(unhandled) > 0 {
		const isUnhandled = true
		return c.handler.partsHandler.Handle(CopyBytes(unhandled), isUnhandled, last, scanError)
	}

	return nil
}

func (c *SplitConsumer) close() error {
	const (
		isClose = false
		scanEror = false
	)
	return c.flush(isClose, scanEror)
}

type scannerHandler struct {
	partsHandler PartsHandler
	err          error
}

func newScannerHandler(partsHandler PartsHandler) *scannerHandler {
	return &scannerHandler{
		partsHandler: partsHandler,
	}
}

func (h *scannerHandler) NewToken(token []byte, isLast bool) {
	if h.err != nil {
		return
	}

	const (
		unhandled = false
		scanError = false
	)

	if err := h.partsHandler.Handle(CopyBytes(token), unhandled, isLast, scanError); err != nil {
		h.err = err
	}
}

func (h *scannerHandler) getErr() error {
	return h.err
}
