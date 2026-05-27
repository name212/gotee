// Copyright 2026
// license that can be found in the LICENSE file.

package consumer

import (
	"bufio"
	"sync"

	tee "github.com/name212/gotee/pkg/tee"
)

var (
	_ PartsHandler = &linePartsWrapper{}
	_ LineHandler  = &FuncLineHandler{}
	_ LineHandler  = &StringsSliceLineHandler{}
)

type (
	FuncStr      func(string) error
	FuncStrNoErr func(string)
)

// LineHandler
// interface for handle lines
type LineHandler interface {
	Handle(l string) error
}

// NewLineConsumer
// Returns SplitConsumer with bufio.ScanLines split function.
// This uses wrap handler to skip all flags from got from PartsHandler
// and always pass input to Linehandler.
// Warning! Do not use WithCopyInput(true) to avoid unnecessary allocations!
func NewLineConsumer(handler LineHandler, name ...string) *SplitConsumer {
	nameForSet := []string{tee.ConsumerName(1, name...)}
	return newLineConsumer(handler, nameForSet...)
}

// NewCustomLineConsumer
// Returns SplitConsumer with bufio.ScanLines split function
// uses raw PartsHandler
// Can be used for better handle Unhandled bytes last token or scan error
// You can use WithCopyInput(true) to avoid use internal buffer slices
func NewCustomLineConsumer(partsHandler PartsHandler, name ...string) *SplitConsumer {
	nameForSet := []string{tee.ConsumerName(1, name...)}
	return NewSplitConsumer(bufio.ScanLines, partsHandler, nameForSet...)
}

// NewLineConsumer
// Like as NewConsumer but get function instead interface.
// Warning! Do not use WithCopyInput(true) to avoid unnecessary allocations!
func NewFuncLineConsumer(handler FuncStr, name ...string) *SplitConsumer {
	nameForSet := []string{tee.ConsumerName(1, name...)}
	return newLineConsumer(NewFuncLineHandler(handler), nameForSet...)
}

// NewLineConsumer
// Like as NewConsumer but get function without returnning error instead interface.
// Warning! Do not use WithCopyInput(true) to avoid unnecessary allocations!
func NewFuncNoErrLineConsumer(handler FuncStrNoErr, name ...string) *SplitConsumer {
	nameForSet := []string{tee.ConsumerName(1, name...)}
	return newLineConsumer(NewFuncNoErrLineHandler(handler), nameForSet...)
}

// StringsSliceLineHandler
// Spetial LineHandler that save all lines to slice.
// After consume all you can use Lines method.
type StringsSliceLineHandler struct {
	mu    sync.Mutex
	lines []string
}

// NewStringsSliceLineHandler
// Creates StringsSliceLineHandler with internal slice.
// you can pass capacity of internal slice by first argument (anoter arguments ignored).
// Default capacity is 16.
func NewStringsSliceLineHandler(capacity ...int) *StringsSliceLineHandler {
	resCap := 16
	if len(capacity) > 0 {
		resCap = capacity[0]
	}

	return &StringsSliceLineHandler{
		lines: make([]string, 0, resCap),
	}
}

// Handle
// Implements LineHandler
func (h *StringsSliceLineHandler) Handle(l string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lines = append(h.lines, l)

	return nil
}

// Lines
// get all lines handler
// each call copy all lines from internal slice to result!
func (h *StringsSliceLineHandler) Lines() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	res := make([]string, len(h.lines))
	copy(res, h.lines)

	return res
}

// FuncLineHandler
// LineHandler function wrapper
type FuncLineHandler struct {
	handler FuncStr
}

// NewFuncLineHandler
// Create FuncLineHandler with passed function
func NewFuncLineHandler(handler FuncStr) *FuncLineHandler {
	return &FuncLineHandler{
		handler: handler,
	}
}

// NewFuncNoErrLineHandler
// Create FuncLineHandler with passed function
func NewFuncNoErrLineHandler(handler FuncStrNoErr) *FuncLineHandler {
	return &FuncLineHandler{
		handler: func(s string) error {
			handler(s)
			return nil
		},
	}
}

// Handle
// Implements LineHandler
func (l *FuncLineHandler) Handle(s string) error {
	return l.handler(s)
}

func newLineConsumer(handler LineHandler, name ...string) *SplitConsumer {
	wrapper := newLinePartsWrapper(handler)
	return NewSplitConsumer(bufio.ScanLines, wrapper, name...)
}

type linePartsWrapper struct {
	handler LineHandler
}

func newLinePartsWrapper(handler LineHandler) *linePartsWrapper {
	return &linePartsWrapper{
		handler: handler,
	}
}

func (h *linePartsWrapper) Handle(part []byte, _ bool, _ bool, _ bool) error {
	return h.handler.Handle(string(part))
}
