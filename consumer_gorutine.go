// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

// import (
// 	"bufio"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"sync"
// 	"sync/atomic"
// )

// var (
// 	_ Consumer    = &SplitConsumer{}
// 	_ PartsHandler = &FuncLineHandler{}
// 	_ PartsHandler = &StringsSliceLineHandler{}
// )

// type PartsHandler interface {
// 	Handle(string) error
// }

// type (
// 	FuncStr      func(string) error
// 	FuncStrNoErr func(string)
// )

// type SplitConsumer struct {
// 	*privateBaseConsumer
// 	handler PartsHandler

// 	started atomic.Bool

// 	pipeWriter *io.PipeWriter
// 	pipeReader *io.PipeReader

// 	errMu sync.RWMutex
// 	err   error

// 	allConsumedCh stopChan
// }

// func NewLineConsumer(handler PartsHandler, name ...string) *SplitConsumer {
// 	r, w := io.Pipe()

// 	return &SplitConsumer{
// 		privateBaseConsumer: newPrivateBaseConsumer(name...),
// 		handler:             handler,
// 		pipeReader:          r,
// 		pipeWriter:          w,
// 		allConsumedCh:       make(stopChan, 1),
// 	}
// }

// func NewFuncLineConsumer(handler FuncStr, name ...string) *SplitConsumer {
// 	nameForSet := []string{CalculateStreamName(1, name...)}
// 	return NewLineConsumer(NewFuncLineHandler(handler), nameForSet...)
// }

// func NewFuncNoErrLineConsumer(handler FuncStrNoErr, name ...string) *SplitConsumer {
// 	nameForSet := []string{CalculateStreamName(1, name...)}
// 	return NewLineConsumer(NewFuncNoErrLineHandler(handler), nameForSet...)
// }

// func (c *SplitConsumer) Write(p []byte) (int, error) {
// 	if err := c.GetErr(); err != nil {
// 		return 0, err
// 	}

// 	if err := c.start(); err != nil {
// 		return 0, err
// 	}

// 	return c.pipeWriter.Write(p)
// }

// func (c *SplitConsumer) start() error {
// 	if c.isClosed() {
// 		return ErrClosed
// 	}

// 	notStarted := c.started.CompareAndSwap(false, true)

// 	if !notStarted {
// 		return nil
// 	}

// 	logger := getDebugLogger("LINE_CONSUMER_START", c.Name())

// 	logger.Log("Start consume")

// 	go func() {
// 		defer func(){
// 			c.allConsumedCh <- noVal
// 			logger.Log("End consume")
// 		}()

// 		scan := bufio.NewScanner(c.pipeReader)
// 		for scan.Scan() {
// 			line := scan.Text()
// 			err := c.handler.Handle(line)
// 			if err != nil {
// 				if !errors.Is(err, io.EOF) {
// 					c.setErr(err)
// 				}
// 				return
// 			}
// 		}
// 	}()

// 	return nil
// }

// func (c *SplitConsumer) setErr(err error) {
// 	c.errMu.Lock()
// 	defer c.errMu.Unlock()

// 	c.err = err
// }

// func (c *SplitConsumer) GetErr() error {
// 	c.errMu.RLock()
// 	defer c.errMu.RUnlock()

// 	return c.err
// }

// func (c *SplitConsumer) Close() error {
// 	if c.setClosed() {
// 		return nil
// 	}

// 	writerErr := c.pipeWriter.Close()
// 	readerErr := c.pipeReader.Close()

// 	<- c.allConsumedCh

// 	var resErr error

// 	if writerErr != nil {
// 		resErr = fmt.Errorf("LineConsumer writer close error: %w", writerErr)
// 	}

// 	if readerErr != nil {
// 		readerErr = fmt.Errorf("LineConsumer reader close error: %w", readerErr)
// 		resErr = appendErr(resErr, readerErr)
// 	}

// 	return resErr
// }

// type StringsSliceLineHandler struct {
// 	mu    sync.Mutex
// 	lines []string
// }

// func NewStringsSliceLineHandler(capacity ...int) *StringsSliceLineHandler {
// 	resCap := 16
// 	if len(capacity) > 0 {
// 		resCap = capacity[0]
// 	}

// 	return &StringsSliceLineHandler{
// 		lines: make([]string, 0, resCap),
// 	}
// }

// func (h *StringsSliceLineHandler) Handle(l string) error {
// 	h.mu.Lock()
// 	defer h.mu.Unlock()

// 	h.lines = append(h.lines, l)

// 	return nil
// }

// func (h *StringsSliceLineHandler) Lines() []string {
// 	h.mu.Lock()
// 	defer h.mu.Unlock()

// 	res := make([]string, len(h.lines))
// 	copy(res, h.lines)

// 	return res
// }

// type FuncLineHandler struct {
// 	handler FuncStr
// }

// func NewFuncLineHandler(handler FuncStr) *FuncLineHandler {
// 	return &FuncLineHandler{
// 		handler: handler,
// 	}
// }

// func NewFuncNoErrLineHandler(handler FuncStrNoErr) *FuncLineHandler {
// 	return &FuncLineHandler{
// 		handler: func(s string) error {
// 			handler(s)
// 			return nil
// 		},
// 	}
// }

// func (l *FuncLineHandler) Handle(s string) error {
// 	return l.handler(s)
// }
