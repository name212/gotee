// Copyright 2026
// license that can be found in the LICENSE file.

package stream

import (
	"github.com/name212/gotee/pkg/internal"
	tee "github.com/name212/gotee/pkg/tee"
)

type baseStream struct {
	stopped     *tee.ClosedFlag
	name        string
	beforeStop  []tee.BeforeStop
	writesCount int
	bufSize     int
	started     *tee.ClosedFlag
}

func newBaseStream() *baseStream {
	return &baseStream{
		stopped:     tee.NewClosedFlag(),
		started:     tee.NewClosedFlag(),
		writesCount: tee.DefaultConsumerBufferedWrites,
		bufSize:     tee.DefaultReadBufSize,
	}
}

func (s *baseStream) isStopped() bool {
	return s.stopped.IsClosed()
}

// WithName
// Set Stream name for debug purposes
// No action after start Run
func (s *baseStream) WithName(n string) {
	if s.started.IsClosed() {
		return
	}

	s.name = n
}

func (s *baseStream) GetName() string {
	return s.name
}

// WithReadBufSize
// Set internal read buffer size.
// By default DefaultReadBufSize
// No action after start Run
func (s *baseStream) WithReadBufSize(size int) {
	if s.started.IsClosed() {
		return
	}

	if size > 0 {
		s.bufSize = size
	}
}

// WithBeforeStop
// Append all paseed befor stop functions
// to call on Stop
// No action after start Run
func (s *baseStream) WithBeforeStop(bs ...tee.BeforeStop) {
	if s.started.IsClosed() {
		return
	}
	s.beforeStop = append(s.beforeStop, bs...)
}

// WritesBufferedCount
// set chan buffer len for internal pipe
// apply for all consumers pipes
// if passed 0 value Stream will block in
// every Write operation
// No action after start Run
func (s *baseStream) WithWritesBufferedCount(n int) {
	if s.started.IsClosed() {
		return
	}

	if s.writesCount >= 0 {
		s.writesCount = n
	}
}

func (s *baseStream) setStopped() bool {
	return s.stopped.SetClosed()
}

func (s *baseStream) runBeforeStop(logger internal.Logger) {
	for indx, bs := range s.beforeStop {
		if internal.IsNil(bs) {
			continue
		}

		logger.Log("Run before stop %d", indx)
		bs()
	}
}
