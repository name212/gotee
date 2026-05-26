// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"errors"
	"fmt"
	"sync"

	"github.com/name212/gotee/internal"
)

type pipe struct {
	consumer Consumer

	writeCh outChan
	// end channel needs for
	// waiting that all data will write
	endCh stopChan

	stopped *ClosedFlag

	writeErrMu sync.RWMutex
	writeErr   error

	// we can call stop from WriteToBuf
	// and from Stream
	stopMu    sync.Mutex
	resultErr error
}

func newPipe(consumer Consumer, writesCount int) *pipe {
	var writeCh outChan
	if writesCount > 0 {
		writeCh = make(outChan, writesCount)
	} else {
		writeCh = make(outChan)
	}

	return &pipe{
		endCh:    make(stopChan, 2),
		writeCh:  writeCh,
		consumer: consumer,
		stopped:  NewClosedFlag(),
	}
}

func (p *pipe) WriteToPipe(buf []byte) (bool, error) {
	const loggerTarget = "WRITE_TO_PIPE"

	writeErr := p.getWriteErr()
	if p.stopped.IsClosed() {
		p.createLogger(loggerTarget).Log("Is closed. Write error %v", writeErr)
		return true, writeErr
	}

	if !internal.IsNil(writeErr) {
		p.createLogger(loggerTarget).Log("Got write error. Stop pipe")
		_ = p.Stop()
		return true, writeErr
	}

	select {
	case p.writeCh <- buf:
		return false, nil
	case <-p.endCh:
		wErrAfterClose := p.getWriteErr()
		p.createLogger(loggerTarget).Log("Got end write. Write err %v. Stop pipe", wErrAfterClose)
		_ = p.Stop()
		return true, wErrAfterClose
	}
}

func (p *pipe) Start() {
	var sendError error

	logger := p.createLogger("WRITE_CYCLE")

	logger.Log("Start write")

	defer func() {
		defer func() {
			// Close end channel on end only!
			// Stream can receive end read, but
			// because we use non-blocking channel
			// for prevent block all consumers when some
			// consumers are slow, we can situation
			// when stream call Stop and wait end writing
			// but we receive write error from consumer
			// if we first close end channel
			// Stop receive end writing and get nil error
			// but after it we set write error for pipe
			//
			// Will we get deadlock or panic?
			// We have next rules:
			// - We use stopped CloseFlag.
			//   In the Stop, we first set stopped flag to true atomically
			// - Also, we can call Stop only from Stream (it is provider)
			//   in WriteToPipe (call only from Stream) or call directly Stop
			//   from Stream after stop Stream Read cycle.
			//   This behavior cover rule "Close channel from writer only!"
			// - WriteToPipe call check that pipe stopped or/and have write error
			// - *** WriteToPipe have select for write to chan and endCh
			// - * Now we have one implementation TeeStream for using pipes
			//   When TeeStream got close flag or error, stream remove
			//   this pipe from list for send.
			// - ** TeeStream use channel for receive read operation
			//   It means that all reads receive and write to pipe in
			//   order and closed or pipe with write error removed
			//   from list for send before next write operation
			// We have next situations (in this we means that close consumer return no error):
			// First, when we call Stop from Stream and writeCh is empty.
			// In this situation:
			// - lock Stop and set pipe as closed
			// - we close write channel in Stop from Stream
			// - check write error in Stop
			// - err is nil. Waiting end write cycle
			// - write cycle finish immediately and close end channel
			//   without call Stop
			// - Stop continue and close consumer
			// - returns nil from Stop
			// - Stream get no error from Stop, no set error for consumer
			//   in Results
			// No deadlock because lock Stop only one and waiting one time
			// (using stopped flag)
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped (atomic set)
			//   or/and have write error no panic
			//   Also see * and ** rules above.
			// Second, when we call Stop from stream and writeCh is not empty
			// and all writes done without error.
			// In this situation:
			// - lock Stop and set pipe as closed
			// - we close write channel in Stop from stream
			// - check write error in Stop
			// - err is nil. Waiting end write cycle
			// - write cycle write all buffered data and close end channel
			//   without call Stop
			// - Stop continue and close consumer
			// - returns nil from Stop because write cycle not set error
			// - Stream get no error from Stop, no set error for consumer
			//   in Results
			// No deadlock because lock Stop only one and waiting one time
			// (using stopped flag)
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped or/and have write error
			//   no panic. Also see * and ** rules above.
			// Third, when we call Stop from stream and writeCh is not empty
			// and one write done with error.
			// In this situation:
			// - lock Stop and set pipe as closed
			// - we close write channel in Stop
			// - check pipe error in Stop
			// - err is nil. Waiting end write cycle
			// - write cycle write until error
			// - break write cycle and go to outer defer
			// - outer defer save error only, not call Stop
			// - close end channel
			// - Stop from stream continue and close consumer
			// - returns not nil from Stop, write cycle set error
			//   and we get error second time
			// - Stream get error from Stop and set error for consumer
			//   in Results
			// No deadlock because lock Stop only one. waiting one time
			// and does not call Stop in write cycle
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped (atomic set)
			//   or/and have write error no panic
			//   Also see * and ** rules above.
			// Four, when we receive error in one consumer
			// and after some times receive not data in reader
			// In this situation:
			// - write cycle write until error
			// - break write cycle and go to outer defer
			// - outer defer save error without Call stop
			// - if pipe receive WriteToPipe call:
			//   - we have write error in pipe
			//   - WriteToPipe see this error and call Stop
			//   - lock Stop and set pipe as closed
			//   - close write channel. for no leak channel
			//   - check pipe error in Stop
			//   - err is not nil. No wait end write cycle
			//     This guarantee that Stop call is not-blocking
			//   - Stop from WriteToPipe continue and close consumer
			//   - save errors to resultErr
			//   - no handle error in WriteToPipe
			//   - returns error and stopped flag to Stream
			//   - see * and ** rules above.
			// - Stream receive end read
			// - Stream call Stop
			// - lock Stop
			// - because stopped flag is true stream no wait end write cycle
			// - Stop from Stream returns saved result error
			// - Stream get error from Stop and set error for consumer
			//   in Results
			// No deadlock because lock Stop separately call. waiting one time
			// and does not call Stop in write cycle
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped (atomic set)
			//   or/and have write error no panic
			//   Also see * and ** rules above.
			// Five, when we receive error in one consumer
			// and receive not data in reader concurrent
			// In this situation we have two sub situation:
			// - Stop call from stream first:
			//   - we close write channel in Stop from Stream
			//   - check write error in Stop
			//   - err is nil. Waiting end write cycle
			//   - write cycle write until error
			//   - break write cycle and go to outer defer
			//   - outer defer save error without Call stop
			//   - in good situation we cannot receive WriteToPipe call
			//   - Stop from stream continue and close consumer
			//   - returns not nil from Stop, write cycle set error
			//     and we get error second time
			//   - Stop from Stream returns write error and save in result error
			//   - Stream get error from Stop and set error for consumer
			//     in Results
			// - Got error from consumer first:
			//   - write cycle write until error
			//   - break write cycle and go to outer defer
			//   - outer defer save error without Call stop
			//   - in parallel Stream call Stop
			//   - lock Stop and set pipe as closed
			//   - we close write channel in Stop from Stream
			//   - check write error in Stop
			//   - err is nil. Waiting end write cycle
			//     We if error is not nil write cycle
			//     already stopped and we continue without wait
			//   - returns not nil from Stop
			//     because we get error second time
			//     here we have write error any way
			//   - Stop from Stream returns write error and save in result error
			//   - Stream get error from Stop and set error for consumer
			//     in Results
			// No deadlock because lock Stop one time. waiting one time
			// or not wait if error because does not call Stop in write cycle
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped (atomic set)
			//   or/and have write error no panic
			//   Also see * and ** rules above.
			// Six, when we receive error in one consumer
			// and call WriteToPipe concurrent
			// In this situation we have two sub situation:
			// - WriteToPipe call from stream first:
			//   - WriteToPipe see that pipe nas not write error
			//   - write to write channel handled data
			//   - returns not stop no error
			//   - write cycle write until error
			//   - break write cycle and go to outer defer
			//   - outer defer save error without Call stop
			//   - next call WriteToPipe see write error
			//   - lock Stop and set pipe as closed
			//   - we close write channel in Stop from WriteToPipe
			//     no leak write channel
			//   - check write error in Stop
			//   - err is not nil. Continue without wait
			//   - set write error to result error
			//     because we get error second time
			//   - no handle Stop error for WriteToPipe
			//   - After all Stream call Stop
			//   - lock Stop
			//   - because stopped flag is true stream no wait end write cycle
			//   - Stop from Stream returns saved result error
			//   - Stream get error from Stop and set error for consumer
			//     in Results
			// - Got error from consumer first:
			//   - write cycle write until error
			//   - break write cycle and go to outer defer
			//   - outer defer save error without Call stop
			//   - in parallel Stream call WriteToPipe
			//   - if error set first, call Stop
			//   - lock Stop and set pipe as closed
			//   - we close write channel in Stop from WriteToPipe
			//     no leak write channel
			//   - check write error in Stop
			//   - err is not nil. Continue without wait
			//   - set write error to result error
			//     because we get error second time
			//   - no handle Stop error for WriteToPipe
			//   - After all Stream call Stop
			//   - lock Stop
			//   - because stopped flag is true stream no wait end write cycle
			//   - Stop from Stream returns saved result error
			//   - Stream get error from Stop and set error for consumer
			//     in Results
			// No deadlock because lock Stop separate calls. one waiting end
			// because error is set in Stop call from WriteToPipe
			// Stop call from Stream returns results because stopped flag
			// set in Stop call from WriteToPipe
			// If pipe receive WriteToPipe call, pipe not send data to closed
			//   channel because check channel is stopped (atomic set)
			//   or/and have write error no panic
			//   Also see * and ** rules above.
			// Seven, WriteToPipe call with Stop concurrently and return error
			// from consumer
			// In good situation we cannot have this, Also see * and ** rules above.
			// But if we got it, we get write error from Stop call from Stream
			// because it was already set in write cycle and we wait it in Stop
			// Eight. when we have unbuffered writeCh and consumer normal closed
			// with ErrClosed
			// In this sitation we can have potential deadlock. We do not set
			// write error in case when consumer returns ErrClosed and we exit 
			// from read from writeCh cycle. But stream can send new potion.
			// In this sitation WriteToPipe forever block on writeCh because
			// we ended gourutine. In end of read from writeCh cycle we close 
			// endCh. With rule *** we get or correct write or signal that
			// read cycle ended. If cycle ended we Stop pipe and returns 
			// that write ended. If we have parallel call Stop from Stream
			// and WriteToPipe we got correct result error because Stop
			// have mutex
			logger.Log("Close endCh")
			close(p.endCh)

			logger.Log("Write stopped")
		}()

		if internal.IsNil(sendError) {
			logger.Log("Send err is nil")
			return
		}

		logger.Log("Send err is: %v", sendError)

		resultSendError := fmt.Errorf(
			"send to consumer %s: %w",
			p.consumer.Name(),
			sendError,
		)

		p.setWriteErr(resultSendError)
	}()

	writeAllToConsumer := func(data []byte) error {
		logger.LogBuf(data, -1, "Write buf...")
		for len(data) > 0 {
			n, err := p.consumer.Write(data)
			if err != nil {
				logger.Log("Write buf got err: %v", err)
				return err
			}

			logger.Log("Written bytes %d", n)

			data = data[n:]
		}

		return nil
	}

	isNormalClosed := func(err error) bool {
		if errors.Is(err, ErrClosed) {
			return true
		}

		return false
	}

	for data := range p.writeCh {
		err := writeAllToConsumer(data)

		if internal.IsNil(err) {
			continue
		}

		if !isNormalClosed(err) {
			sendError = err
		}

		return
	}
}

func (p *pipe) Stop() error {
	// use lock for prevent parallel
	// Stops, second stop can return incorrect
	// error because another gorutine
	// can hot set writer error correctly
	// and for prevent concurent ser resultErr
	p.stopMu.Lock()
	defer p.stopMu.Unlock()

	logger := p.createLogger("STOP")

	if p.stopped.SetClosed() {
		logger.Log("Already stopped")
		return p.resultErr
	}

	logger.Log("Close writeCh")
	close(p.writeCh)

	if err := p.getWriteErr(); internal.IsNil(err) {
		logger.Log("No error. Waiting ending write...")
		<-p.endCh
	} else {
		logger.Log("Error set from write channel. Skip waiting ending write to prevent deadlock")
	}

	logger.Log("Write ended. Close consumer...")

	closeErr := p.consumer.Close()
	if closeErr != nil {
		logger.Log("Close consumer err: %v", closeErr)
		closeErr = fmt.Errorf("consumer '%s' close error: %w", p.consumer.Name(), closeErr)
	}

	// need to second call for getWriteErr because we can have
	// write error after
	p.resultErr = internal.AppendErr(p.resultErr, p.getWriteErr(), closeErr)

	logger.Log("Stopped! Result err %v", p.resultErr)

	return p.resultErr
}

func (p *pipe) getWriteErr() error {
	p.writeErrMu.RLock()
	defer p.writeErrMu.RUnlock()

	return p.writeErr
}

func (p *pipe) setWriteErr(err error) {
	p.writeErrMu.Lock()
	defer p.writeErrMu.Unlock()

	p.writeErr = internal.AppendErr(p.writeErr, err)
}

func (p *pipe) createLogger(target string) internal.Logger {
	return internal.GetDebugLogger("INTERNAL_PIPE", target, p.consumer.Name())
}
