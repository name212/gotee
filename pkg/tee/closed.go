// Copyright 2026
// license that can be found in the LICENSE file.

package tee

import (
	"sync/atomic"
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
