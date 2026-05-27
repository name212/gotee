// Copyright 2026
// license that can be found in the LICENSE file.

package consumer

import (
	"fmt"

	"github.com/name212/gotee/pkg/internal"
	tee "github.com/name212/gotee/pkg/tee"
)

type (
	ImplWriteFunc func([]byte) (int, error)
	ImplCloseFunc func() error
)

// BaseConsumer
// base struct for implement your own consumers
// This consumer implements Write and Close methods
// with additional checks.
// Write method cheks that consumer already closed first.
// If closed returns 0, ErrClosed for follow Consumer.Write contract
// Next, if needs copy input and pass call writeOp passed to
// constructor with result input
// Close method immediately returns without error if consumer already closed
// follow Consumer.Close contract.
// Otherwise if passed closeOp, call this operation
type BaseConsumer struct {
	name      string
	closed    *tee.ClosedFlag
	copyInput bool
	writeOp   ImplWriteFunc
	closeOp   ImplCloseFunc
}

// NewBaseConsumer
// Create BaseConsumer with name and write function implementation
// closeOp can do not passed. In this case Close method call empty func returns nil
// If passed, get first closeOp and if is not nil call first closeOp in Close method
// In yor own implementation if name is not passed you can use
// ConsumerName(2) call for getting uniq consumer name
// For example, you can next Consumer impl and consumer constructor:
//
//	type MyConsumer struct {
//	  *BaseConsumer
//	}
//
// func (c* MyConsumer) write([]byte) (int, error) {...}
// func (c* MyConsumer) close() error {...}
//
//	func NewMyConsumer(name string...) *MyConsumer {
//	   // 1 means that got called name from place call NewMyConsumer if name is empty
//	   nameForSet := ConsumerName(1, name...)
//	   m := &MyConsumer{}
//	   m.BaseConsumer = NewBaseConsumer(nameForSet, m.write, m.close)
//	   return m
//	}
func NewBaseConsumer(name string, writeOp ImplWriteFunc, closeOp ...ImplCloseFunc) *BaseConsumer {
	if internal.IsNil(writeOp) {
		panic(fmt.Errorf("writeOp did not passed for consumer %s", name))
	}

	var cl ImplCloseFunc = noCloseImp
	if len(closeOp) > 0 && !internal.IsNil(closeOp[0]) {
		cl = closeOp[0]
	}

	return &BaseConsumer{
		name:      name,
		closed:    tee.NewClosedFlag(),
		copyInput: false,
		writeOp:   writeOp,
		closeOp:   cl,
	}
}

// Name
// returns passed name to constructor
func (c *BaseConsumer) Name() string {
	return c.name
}

// WithCopyInput
// set setting that should copy input before handle
func (c *BaseConsumer) WithCopyInput(f bool) {
	c.copyInput = f
}

// Write
// Consumer Write implementation
func (c *BaseConsumer) Write(input []byte) (int, error) {
	if c.closed.IsClosed() {
		return 0, tee.ErrClosed
	}

	if c.copyInput {
		input = tee.CopyBytes(input)
	}

	return c.writeOp(input)
}

// Close
// Consumer Close implementation
func (c *BaseConsumer) Close() error {
	if c.closed.SetClosed() {
		return nil
	}

	return c.closeOp()
}

// IsClosed
// returns true is consumer is closed
func (c *BaseConsumer) IsClosed() bool {
	return c.closed.IsClosed()
}

type privateBaseConsumer struct {
	*BaseConsumer
}

func newPrivateBaseConsumer(writeOp ImplWriteFunc, name ...string) *privateBaseConsumer {
	nameForSet := tee.ConsumerName(2, name...)
	return &privateBaseConsumer{
		BaseConsumer: NewBaseConsumer(nameForSet, writeOp),
	}
}

func (c *privateBaseConsumer) withClose(op ImplCloseFunc) *privateBaseConsumer {
	if internal.IsNil(op) {
		op = noCloseImp
	}

	c.closeOp = op

	return c
}

func noCloseImp() error {
	return nil
}
