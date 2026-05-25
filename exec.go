// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"time"

	"github.com/name212/gotee/internal"
)

type CmdCleaner interface {
	GetError(noWait ...bool) error
}

var (
	ErrCmdCleanerNotFinished = fmt.Errorf("CmdCleaner not finished")
	ErrCleanAfterRun         = fmt.Errorf("cannot clean after run cmd")
	ErrRunCmd                = fmt.Errorf("cannot run cmd")
	ErrCreateStreamBeforeRun = fmt.Errorf("cannot create stream before run cmd")
)

type (
	RunCmdOpts struct {
		stdoutConsumers []Consumer
		stderrConsumers []Consumer
		bufSize         int
		name            string
		closeWait       *time.Duration

		bufferedWritesCount *int
	}

	RunCmdOpt func(*RunCmdOpts)
)

func RunCmdWithStdout(consumers ...Consumer) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if len(consumers) == 0 {
			return
		}

		o.stdoutConsumers = append(o.stdoutConsumers, consumers...)
	}
}

func RunCmdWithStderr(consumers ...Consumer) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if len(consumers) == 0 {
			return
		}

		o.stderrConsumers = append(o.stderrConsumers, consumers...)
	}
}

func RunCmdWithBufSize(size int) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if size > 0 {
			o.bufSize = size
		}
	}
}

func RunCmdWithBufferedWritesCount(n int) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if n >= 0 {
			o.bufferedWritesCount = &n
		}
	}
}

func RunCmdWithName(name string) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if o.name != "" {
			return
		}

		o.name = name
	}
}

func RunCmdWithCloseWait(w time.Duration) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if o.closeWait != nil {
			return
		}

		o.closeWait = &w
	}
}

func RunCmd(ctx context.Context, cmd *exec.Cmd, opts ...RunCmdOpt) (*Results, error) {
	cloneOpts := make([]RunCmdOpt, len(opts))
	copy(cloneOpts, opts)

	runCmdAdditionalOptions := []RunCmdOpt{
		RunCmdWithName(cmd.String()),
		RunCmdWithCloseWait(200 * time.Millisecond),
	}

	cloneOpts = append(cloneOpts, runCmdAdditionalOptions...)

	stream, cleaner, err := NewStreamForCmd(cmd, cloneOpts...)
	if err != nil {
		return nil, internal.ConcatErrs(ErrCreateStreamBeforeRun, err)
	}

	resCh := make(chan *Results, 1)

	go func() {
		res := stream.Run(ctx)
		resCh <- res
		close(resCh)
	}()

	cleanupAndReturnErr := func(err error) (*Results, error) {
		stream.Stop()

		if cleanerErr := cleaner.GetError(); cleanerErr != nil {
			err = internal.AppendErr(err, cleanerErr)
		}

		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return cleanupAndReturnErr(fmt.Errorf("%w cannot start: %w", ErrRunCmd, err))
	}

	var resErr error

	if err := cmd.Wait(); err != nil {
		resErr = internal.AppendErr(resErr, fmt.Errorf("%w cannot wait: %w", ErrRunCmd, err))
	}

	stream.Stop()
	results := <-resCh

	if err := cleaner.GetError(); err != nil {
		resErr = internal.AppendErr(resErr, internal.ConcatErrs(ErrCleanAfterRun, err))
	}

	return results, resErr
}

func NewStreamForCmd(cmd *exec.Cmd, opts ...RunCmdOpt) (*CombineStream, CmdCleaner, error) {
	optsToSet := &RunCmdOpts{
		bufSize: DefaultReadBufSize,
	}

	for _, o := range opts {
		o(optsToSet)
	}

	stdoutConsumers := optsToSet.stdoutConsumers
	stderrConsumers := optsToSet.stderrConsumers

	if len(stdoutConsumers) == 0 && len(stderrConsumers) == 0 {
		return nil, &noCleaner{}, fmt.Errorf("stdout and/or sterr consumers not passed")
	}

	closeWaitTime := time.Duration(0)
	if optsToSet.closeWait != nil {
		closeWaitTime = *optsToSet.closeWait
	}

	cleaner := newReaderWriterCleaner(closeWaitTime)

	createErr := func(f string, args ...any) (*CombineStream, CmdCleaner, error) {
		cleaner.close()
		return nil, cleaner, fmt.Errorf(f, args...)
	}

	streams := make([]Stream, 0, 2)

	createPipe := func(name string) (io.ReadCloser, io.WriteCloser) {
		reader, writer := io.Pipe()
		cleaner.append(name, reader, writer)
		return reader, writer
	}

	createTeeStream := func(r io.Reader, consumers []Consumer, name string) (*TeeStream, error) {
		st, err := NewTeeStream(r, consumers...)
		if err != nil {
			return nil, err
		}

		if optsToSet.bufSize > 0 {
			st.WithBufSize(optsToSet.bufSize)
		}

		if optsToSet.bufferedWritesCount != nil {
			st.WithWritesBufferedCount(*optsToSet.bufferedWritesCount)
		}

		streamName := fmt.Sprintf("%s:%s", optsToSet.name, name)
		st.WithName(streamName)

		return st, nil
	}

	if len(stdoutConsumers) > 0 {
		const stdoutName = "stdout"

		reader, writer := createPipe(stdoutName)
		cmd.Stdout = writer

		st, err := createTeeStream(reader, stdoutConsumers, stdoutName)
		if err != nil {
			return createErr("cannot create TeeStream for stdout: %w", err)
		}

		streams = append(streams, st)
	}

	if len(stderrConsumers) > 0 {
		const stderrName = "stderr"

		reader, writer := createPipe(stderrName)
		cmd.Stderr = writer

		st, err := createTeeStream(reader, stderrConsumers, stderrName)
		if err != nil {
			return createErr("cannot create TeeStream for stderr: %w", err)
		}

		streams = append(streams, st)
	}

	combine, err := NewCombineStream(streams...)
	if err != nil {
		return createErr("cannot create combine stream: %w", err)
	}

	combine.WithBeforeStop(func() {
		cleaner.close()
	})

	return combine, cleaner, nil
}

func cmdPipeClosed(err error) bool {
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	if errors.Is(err, os.ErrClosed) {
		return true
	}

	if errors.Is(err, fs.ErrClosed) {
		return true
	}

	if fsError, ok := err.(*fs.PathError); ok {
		if errors.Is(fsError.Err, fs.ErrClosed) {
			return true
		}
	}

	if osError, ok := err.(*os.PathError); ok {
		if errors.Is(osError.Err, os.ErrClosed) {
			return true
		}
	}

	return false
}

type noCleaner struct{}

func (c *noCleaner) GetError(noWait ...bool) error {
	return nil
}

type readerWriterCleaner struct {
	errCh errChan

	*ClosedFlag
	err error

	readers map[string]io.Closer
	writers map[string]io.Closer

	closeReadersWait time.Duration
}

func newReaderWriterCleaner(closeReadersWait time.Duration) *readerWriterCleaner {
	return &readerWriterCleaner{
		closeReadersWait: closeReadersWait,
		ClosedFlag:       NewClosedFlag(),
		errCh:            make(errChan, 1),
		readers:          make(map[string]io.Closer),
		writers:          make(map[string]io.Closer),
	}
}

func (c *readerWriterCleaner) append(name string, reader, writer io.Closer) {
	c.readers[name] = reader
	c.writers[name] = writer
}

func (c *readerWriterCleaner) close() {
	if c.IsClosed() {
		return
	}

	// first close writers
	err := c.closeOnly("writer", c.writers)

	// wait some time for reads
	if c.closeReadersWait > 0 {
		time.Sleep(c.closeReadersWait)
	}

	err = internal.AppendErr(err, c.closeOnly("reader", c.readers))

	c.errCh <- err
	close(c.errCh)
}

func (c *readerWriterCleaner) closeOnly(tp string, closers map[string]io.Closer) error {
	var resErr error
	for name, closer := range closers {
		if err := closer.Close(); err != nil {
			if !cmdPipeClosed(err) {
				resErr = internal.AppendErr(resErr, fmt.Errorf("cannot close %s for %s: %w", tp, name, err))
			}
		}
	}

	return resErr
}

func (c *readerWriterCleaner) GetError(noWait ...bool) error {
	if c.IsClosed() {
		return c.err
	}

	if len(noWait) > 0 && noWait[0] {
		select {
		case c.err = <-c.errCh:
		default:
		}
	} else {
		c.err = <-c.errCh
	}

	c.SetClosed()

	return c.err
}
