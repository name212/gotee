// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"context"
	"fmt"
	"io"

	"github.com/name212/gotee/internal"
)

const (
	// DefaultConsumerBufferedWrites
	// Default count of non-blockin reads
	DefaultConsumerBufferedWrites = 100
	// DefaultReadBufSize
	// Default read buffersize
	DefaultReadBufSize = 16
	// DefaultMaxEmptyReads
	// Count of available empty reads without errors
	// If stream got this numbers empty reads Stream.Run
	// returns io.ErrNoProgress to read error
	DefaultMaxEmptyReads = 300
)

var (
	// ErrStreamStopped
	// Returns if Stream.Run is stopped
	ErrStreamStopped = fmt.Errorf("stream was stopped")
	// ErrStopped
	// Returns on double call Stream.Run
	ErrStreamAlreadyStarted = fmt.Errorf("stream already started")
	// ErrClosed
	// spetial error for pass to stream that consumer
	// does not need more data
	ErrClosed = fmt.Errorf("already closed")
)

type (
	ConsumersErrors = map[string]error
	BeforeStop      func()
)

// Consumer
// Base interface to implement consumer
// Should implements io.WriteCloser interface
// Consumer should follow next rulles
//   - Close method should safe to call multiple times
//   - Close should not freeze, because Close calls not in gorutine
//   - If consumer should not receive more data
//     Write method should return ErrClosed error
//     in this case Results will not have error for consumer
//   - Write metod should copy received slice
//     because it can be part of slice of another buffer
//     or pulled from internal Pool
//   - Write method should check that Consumer is closed
//     if is closed should return 0, ErrClosed
type Consumer interface {
	io.WriteCloser
	// Name
	// returns unique name for consumer
	// this string using for set error
	// in Results for consumers
	Name() string
}

// Stream
// Base interface to implement read bytes from io.Reader
// and pass potion of readed data to all passed Consumer
type Stream interface {
	// Run
	// Start consume bytes from reader passed to stream
	// If one of consumer or stream get read error
	// returns not nil Results
	// if all consumes were not return errors
	// and was not got read error returns nil Results.
	// All consumers run in its own gorutine.
	// Also run operation send data potion over
	// chan with buffer len returned from WritesBufferedCount.
	// Run wait when all data read from Reader or all consumers stopped
	// or in error.
	// Run can returns io.ErrNoProgress in Results.ReadErr
	// Warning! run is block operation!
	// Warning! You should close reader for prevent leak internal read gourutine!
	// You can pass close of reader to WithBeforeStop
	// for bytes.Buffer we have ClosableReaderBuffer wrapper
	// ClosableReaderBuffer can used in the next way:
	// buf := &bytes.Buffer{}
	// w := NewClosableReaderBuffer(buf)
	// s := NewTeeStream(w, ...)
	// s.WithBeforeStop(CloserBeforeStop(w))
	Run(context.Context) *Results

	// WithBeforeStop
	// All passed  BeforeStop functions will call
	// before stop operations
	// WARNING! each function run syncroniosly
	// and can blok Stop operation
	WithBeforeStop(...BeforeStop)

	// Stop
	// Can call if need stop consume operation
	// during read
	// Safe for multiple calls
	Stop()

	// WaitReadEnd
	// Util function for wait end internal read cycle
	// In most cases should not used. For dubug and test purposes
	// Safe for multiple calls
	WaitReadEnd(context.Context) error
}

// Results
// Struct for return errors got from consumers
// or read operation
// implemens error interface
type Results struct {
	ReadErr error
	// ConsumersErrs
	// errors got from consumers
	// consumer name -> error
	ConsumersErrs ConsumersErrors
}

// HasReadError
// returns true if Run operation got read error
// safe for call if Results is nil
func (r *Results) HasReadError() bool {
	if r == nil {
		return false
	}

	return !internal.IsNil(r.ReadErr)
}

// HasConsumersErrors
// returns true if Run operation got error from least one consumer
// safe for call if Results is nil
func (r *Results) HasConsumersErrors() bool {
	if r == nil {
		return false
	}

	return len(r.ConsumersErrs) > 0
}

// HasLeastOneError
// returns true if Run operation got error from least one consumer
// or read error
// safe for call if Results is nil
func (r *Results) HasLeastOneError() bool {
	if r == nil {
		return false
	}

	if r.HasReadError() {
		return true
	}

	if r.HasConsumersErrors() {
		return true
	}

	return false
}

// GetError
// returns combine error for read and consumers errors
// if Results is nil or not has any errors returns nil
// safe for call if Results is nil
func (r *Results) GetError() error {
	if r == nil {
		return nil
	}

	if !r.HasLeastOneError() {
		return nil
	}

	var res error

	if r.HasReadError() {
		res = fmt.Errorf("read error: %w", r.ReadErr)
	}

	if !r.HasConsumersErrors() {
		return res
	}

	for c, err := range r.ConsumersErrs {
		err = fmt.Errorf("consumer '%s' error: %w", c, err)
		res = internal.AppendErr(res, err)
	}

	return res
}

// Error
// implements error interface
// safe for call if Results is nil
func (r *Results) Error() string {
	if r == nil {
		return ""
	}

	if err := r.GetError(); err != nil {
		return err.Error()
	}

	return ""
}

// CloserBeforeStop
// Wrap Closer.Close call to use pass to use WithBeforeStop
// CloserBeforeStop handle panic with recover
func CloserBeforeStop(c io.Closer) BeforeStop {
	return func() {
		defer func() {
			_ = recover()
		}()

		_ = c.Close()
	}
}

func newStoppedResults() *Results {
	return &Results{
		ReadErr:       ErrStreamStopped,
		ConsumersErrs: make(ConsumersErrors),
	}
}

func newAlreadyStartedResults() *Results {
	return &Results{
		ReadErr:       ErrStreamAlreadyStarted,
		ConsumersErrs: make(ConsumersErrors),
	}
}

type (
	noValT   = struct{}
	stopChan = chan noValT
	outChan  = chan []byte
	errChan  = chan error
)

var (
	noVal = struct{}{}
)
