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
	readEndCh   stopChan

	maxEmptyReads int
}

func NewTeeStream(input io.Reader, consumers ...Consumer) (*TeeStream, error) {
	if len(consumers) == 0 {
		return nil, fmt.Errorf("empty consumers list")
	}

	return &TeeStream{
		baseStream:    newBaseStream(),
		input:         input,
		consumers:     append([]Consumer{}, consumers...),
		innerStopCh:   make(stopChan, 1),
		readEndCh:     make(stopChan),
		maxEmptyReads: DefaultMaxEmptyReads,
	}, nil
}

// WithMaxEmptyReads
// Set maximum available empty reads
// No action after start Run
func (s *TeeStream) WithMaxEmptyReads(n int) *TeeStream {
	if s.started.IsClosed() {
		return s
	}

	if n > 0 {
		s.maxEmptyReads = n
	}

	return s
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
// if max empty reads reached Run returns io.ErrNoProgress in Results.ReadErr
func (s *TeeStream) Run(ctx context.Context) *Results {
	if s.isStopped() {
		return newStoppedResults()
	}

	if s.started.SetClosed() {
		return newAlreadyStartedResults()
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
			logger.Log("Got stop from read")
			break OuterLoop
		// we handle innerStopCh in read cycle and here
		// for next reason.
		// we can have slow reader and we can block on io.Reader
		// in this situation we do not exit from this cycle
		// and Run blocks for unknown time after call Stop.
		// Also we can have sitation in reader cycle.
		// We have block channel for send to consumer
		// if we receive innerStopCh we should return from
		// read cycle as fast as possible for avoid leak
		// goroutine
		case <-s.innerStopCh:
			logger.Log("Got stop signal in run handler")
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
		logger.Log("Close pipe for '%s'...", consumerName)

		if err := p.Stop(); err != nil {
			consumersErrs[consumerName] = err
			logger.Log("Consumer '%s' has error: '%v'. Save to results", consumerName, err)
		}
	}

	logger.Log("All pipes were closed. Send stop to reader cycle...")

	// call stop here to prevent leak read gourutine
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

	close(s.innerStopCh)
}

// WaitReadEnd
// waiting wen read cycle was closed
func (s *TeeStream) WaitReadEnd(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.readEndCh:
		return nil
	}
}

func (s *TeeStream) startRead(outCh outChan, stopCh stopChan, errCh errChan) {
	defer func() {
		// close channels for prevent leak
		close(outCh)
		close(stopCh)
		close(errCh)
		close(s.readEndCh)
	}()

	logger := s.createLogger("READ_CYCLE")

	sendStop := func() {
		stopCh <- noVal
	}

	buf := make([]byte, s.bufSize)
	emptyReads := 0

	sendPart := func(n int) (bool, error) {
		if n <= 0 {
			emptyReads++
			if emptyReads >= s.maxEmptyReads {
				logger.Log("Reached max empty read %d", s.maxEmptyReads)
				return true, io.ErrNoProgress
			}
		}

		emptyReads = 0

		logger.LogBuf(buf, n, "Receive buf, send to Run")

		toSend := make([]byte, n)
		copy(toSend, buf[:n])

		select {
		case outCh <- toSend:
			return false, nil
		case <-s.innerStopCh:
			logger.Log("Receive stop signal during send to outCh")
			return true, nil
		}
	}

	sendErr := func(err error) {
		logger.Log("End read. Got error: %v", err)
		errCh <- err
	}

	for {
		n, err := s.input.Read(buf)
		exit, gotSendErr := sendPart(n)
		if gotSendErr != nil {
			sendErr(gotSendErr)
			return
		}

		if exit {
			sendStop()
			return
		}

		if internal.IsNil(err) {
			if !s.isReceiveStop() {
				logger.Log("Continue read...")
				continue
			}

			logger.Log("Got stop signal. Send stop to Run")
			sendStop()
			return
		}

		if s.isEndRead(err) {
			logger.Log("End read. Send stop to Run")
			sendStop()
		} else {
			sendErr(err)
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

func (s *TeeStream) createLogger(target string) internal.Logger {
	return internal.GetDebugLogger("TEE_STREAM", s.GetName(), target)
}
