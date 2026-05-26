// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/name212/gotee/internal"
)

var _ Stream = &TeeStream{}

// TeeStream
// Base implementation of Stream interface
type TeeStream struct {
	*baseStream

	input     io.Reader
	consumers []Consumer

	innerStopCh stopChan
}

func NewTeeStream(input io.Reader, consumers ...Consumer) (*TeeStream, error) {
	if len(consumers) == 0 {
		return nil, fmt.Errorf("empty consumers list")
	}

	return &TeeStream{
		baseStream:  newBaseStream(),
		input:       input,
		consumers:   append([]Consumer{}, consumers...),
		innerStopCh: make(stopChan, 1),
	}, nil
}

// Run
// implements Stream interface
// Run run each consumer in different gourutines
// Read data from reader until io.EOF or io.ErrClosedPipe error get
// Also Run operation stops if all consumers stopped wit ErrClosed error
// or all consumers returns errors
// Results.ReadErr will have error from io.Reader exclude io.EOF or io.ErrClosedPipe
// Results.ConsumersErrs contains all errors from all consumes
// if not receive read error and all consumers not have errors returns nil Results
func (s *TeeStream) Run(ctx context.Context) *Results {
	if s.isStopped() {
		return newStoppedResults()
	}

	stopCh := make(stopChan, 2)
	errCh := make(errChan)
	outCh := make(outChan)

	allPipesLen := len(s.consumers)

	allPipes := make([]*pipe, 0, allPipesLen)
	// no need mutex because we use currentPipesForSend
	// only in sendToAll that called from select
	currentPipesForSend := make([]*pipe, 0, allPipesLen)

	pipeWritesBufferedCount := s.writesCount

	for _, c := range s.consumers {
		p := newPipe(c, pipeWritesBufferedCount)
		allPipes = append(allPipes, p)
		currentPipesForSend = append(currentPipesForSend, p)
		go p.Start()
	}

	logger := s.createLogger("RUN")
	loggerSendAll := s.createLogger("SEND_ALL")

	// to avoid allocation
	// no need mutex because we use pipesForRemove
	// only in sendToAll that called from select
	pipesForRemove := make(map[int]struct{}, allPipesLen)

	sendToAll := func(b []byte) bool {
		pipesCount := len(currentPipesForSend)

		loggerSendAll.LogBuf(b, -1, "Send buf to current pipes %d", pipesCount)

		errPipes := 0
		stoppedPipes := 0
		sended := 0

		clear(pipesForRemove)

		for indx, p := range currentPipesForSend {
			stoppedOrErr := false
			stopped, writeErr := p.WriteToPipe(b)

			if stopped {
				stoppedPipes++
				stoppedOrErr = true
			}

			if !internal.IsNil(writeErr) {
				errPipes++
				stoppedOrErr = true
			}

			if stoppedOrErr {
				loggerSendAll.Log("detect stopped or write error pipe for consumer '%s'", p.consumer.Name())
				pipesForRemove[indx] = struct{}{}
				continue
			}

			sended++
		}

		loggerSendAll.Log(
			"sends %d: done %d; closed %d, errors: %d",
			pipesCount,
			sended,
			stoppedPipes,
			errPipes,
		)

		if len(pipesForRemove) > 0 {
			toReplacePipes := make([]*pipe, 0)
			loggerSendAll.Log("Got pipes for remove %d", len(pipesForRemove))
			for indx, pipeToSave := range currentPipesForSend {
				if _, ok := pipesForRemove[indx]; ok {
					loggerSendAll.Log("remove pipe for consumer '%s' from pipes to send", pipeToSave.consumer.Name())
					continue
				}
				toReplacePipes = append(toReplacePipes, pipeToSave)
			}

			currentPipesForSend = toReplacePipes
		}

		if len(currentPipesForSend) == 0 {
			return true
		}

		return false
	}

	logger.Log("Start read")

	go s.startRead(outCh, stopCh, errCh)

	var readErr error
OuterLoop:
	for {
		select {
		case <-ctx.Done():
			logger.Log("Got ctx done")
			if err := ctx.Err(); err != nil {
				readErr = fmt.Errorf("handle context error: %w", err)
			}
			s.Stop()
			break OuterLoop
		case err, ok := <-errCh:
			if !ok {
				logger.Log("Err channel was closed")
				break OuterLoop
			}
			logger.Log("Got read err: %v", err)
			readErr = err
			break OuterLoop
		case <-stopCh:
			logger.Log("Got stop")
			break OuterLoop
		case buf, ok := <-outCh:
			if !ok {
				logger.Log("Out channel was closed")
				break OuterLoop
			}
			logger.LogBuf(buf, -1, "Got buf from outCh")
			if allClosed := sendToAll(buf); allClosed {
				logger.Log("All consumers are closed")
				break OuterLoop
			}
		}
	}

	logger.Log("End read. Stop pipes")

	consumersErrs := make(ConsumersErrors)

	for _, p := range allPipes {
		consumerName := p.consumer.Name()
		logger.Log("Close pipe pipe for '%s'...", consumerName)

		if err := p.Stop(); err != nil {
			consumersErrs[consumerName] = err
			logger.Log("Consumer '%s' has error: '%v'. Save to results", consumerName, err)
		}
	}

	logger.Log("All pipes were closed. Send stop to reader cycle...")

	s.Stop()

	r := &Results{
		ReadErr:       readErr,
		ConsumersErrs: consumersErrs,
	}

	if r.HasLeastOneError() {
		logger.Log("Has least one errors. Returns not nil results")
		return r
	}

	logger.Log("Done without errors. Returns nil results")

	return nil
}

// Stop
// Stop read operation
// Safe for multiple calls
// Runs all VeforeStop funtions
// And send stop signal to internal reader goroutine
// without blocking
func (s *TeeStream) Stop() {
	logger := s.createLogger("STOP")

	if s.setStopped() {
		logger.Log("Already stopped")
		return
	}

	s.runBeforeStop(logger)

	logger.Log("Send stop signal to reader cycle")

	s.innerStopCh <- noVal
}

func (s *TeeStream) startRead(outCh outChan, stopCh stopChan, errCh errChan) {
	defer func() {
		// close channels for prevent leak
		close(outCh)
		close(stopCh)
		close(errCh)
	}()

	buf := make([]byte, s.bufSize)

	sendStop := func() {
		stopCh <- noVal
	}

	logger := s.createLogger("READ_CYCLE")

	for {
		n, err := s.input.Read(buf)
		if n > 0 {
			logger.LogBuf(buf, n, "Receive buf, send to Run")
			toSend := make([]byte, n)
			copy(toSend, buf[:n])
			outCh <- toSend
		}

		if internal.IsNil(err) {
			if s.isReceiveStop() {
				logger.Log("Got stop signal. Send stop to Run")
				sendStop()
				return
			}

			logger.Log("Continue read...")

			continue
		}

		if s.isEndRead(err) {
			logger.Log("End read. Send stop to Run")
			sendStop()
		} else {
			logger.Log("End read. Got error: %v", err)
			errCh <- err
		}

		return
	}
}

func (s *TeeStream) isReceiveStop() bool {
	select {
	case <-s.innerStopCh:
		return true
	default:
		return false
	}
}

func (s *TeeStream) isEndRead(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}

	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	return false
}

func (p *TeeStream) createLogger(target string) internal.Logger {
	return internal.GetDebugLogger("TEE_STREAM", p.GetName(), target)
}
