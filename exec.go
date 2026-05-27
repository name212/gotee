// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/name212/gotee/internal"
)

// CmdCleaner
// interface that close internal buffers
// for RunCommand and provide close buffers errors
// CmdCleaner try to handle all Closed errors
// for avoid retun error on Run call
type CmdCleaner interface {
	// GetError
	// Get errors got during close interanl buffers
	// if noWait passed as first true no wait and
	// immediately returns error
	// Otherwise, wait until all buffers close and return errors
	GetError(noWait ...bool) error
}

var (
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

// RunCmdWithStdout
// Create Option that provide consumers for handle stdout from command
func RunCmdWithStdout(consumers ...Consumer) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if len(consumers) == 0 {
			return
		}

		o.stdoutConsumers = append(o.stdoutConsumers, consumers...)
	}
}

// RunCmdWithStderr
// Create Option that provide consumers for handle stderr from command
func RunCmdWithStderr(consumers ...Consumer) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if len(consumers) == 0 {
			return
		}

		o.stderrConsumers = append(o.stderrConsumers, consumers...)
	}
}

// RunCmdWithStderr
// Create Option that provide read buffer size internal streams
func RunCmdWithReadBufSize(size int) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if size > 0 {
			o.bufSize = size
		}
	}
}

// RunCmdWithStderr
// Create Option that provide writes chan buffer size
func RunCmdWithBufferedWritesCount(n int) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if n >= 0 {
			o.bufferedWritesCount = &n
		}
	}
}

// RunCmdWithStderr
// Create Option that provide name for result stream
func RunCmdWithName(name string) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if o.name != "" {
			return
		}

		o.name = name
	}
}

// RunCmdWithStderr
// Create Option that provide wait duration before close
// read pipe affter close write pipe
// After programm was exit, Stream run CmdCleaner for close
// internal pipes for stop Stream (avoid forever block on read)
// that can be used if you use slow consumers for avoid
// lost output
func RunCmdWithCloseWait(w time.Duration) RunCmdOpt {
	return func(o *RunCmdOpts) {
		if o.closeWait != nil {
			return
		}

		o.closeWait = &w
	}
}

// NewStreamForCmd
// Prepare CombineStream for consume stdout and/or stderr for passed command
// without running.
// You can pass consumers for stderr and stdout both or not with
// RunCmdWithStdout and RunCmdWithStderr
// CombineStream use 1 or two TeeStream for consume output.
// Each stream use for stdout or stderr.
// For comunicate with cmd NewStreamForCmd creates io.Pipe for stout and/or stderr
// This not use cmd.Stdou/errPipe because it can returns ClosePipe error.
// Also NewStreamForCmd creates internal CmdCleaner that added to 
// OnBeforeStop functions for avoid forever block on read after stop programm
// (remember, we are using io.Pipe for communicating with programm, see reason above).
// You can add your own before cleanup functions.
// NewStreamForCmd returns CmdCleaner for getting close errors from close communicating pipes.
// For consume you should CombineStream.Run in gouritine and call cmd.Start and cmd.Wait 
// Do not forget call CombineStream.Stop after cmd.Wait for prevent block
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
		return nil, &noWriteReaderCleaner{}, fmt.Errorf("stdout and/or sterr consumers not passed")
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
			st.WithReadBufSize(optsToSet.bufSize)
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

// RunCmd
// Run programm for consume stdout and/or stderr from it.
// Uses NewStreamForCmd for create stream
// RunCmd:
// - calls CombineStream.Run in gourutine
// - Start and Wait command run
// - If Start returns error run CmdCleaner and return 
// - If Wait returns error, save error
// - calls CombineStream.Stop and wait returns result from CombineStream.Run gourutine
// - run CmdCleaner with combine errors from cmd.Wait and run CmdCleaner
// - wait with block in CmdCleaner.GetError
// You can check the next spetial errors for handling the next situations:
// - ErrCreateStreamBeforeRun - if Stream cannot created. Cmd is not running in this case
// - ErrRunCmd - cmd.Start or cmd.Wait returns error
// - ErrCleanAfterRun - CmdCleaner returned from NewStreamForCmd has error
// RunCmd func covered with tests in ./gotee_test/exec_test.go
func RunCmd(ctx context.Context, cmd *exec.Cmd, opts ...RunCmdOpt) (*Results, error) {
	runCmdAdditionalOptions := []RunCmdOpt{
		RunCmdWithName(cmd.String()),
		RunCmdWithCloseWait(200 * time.Millisecond),
	}

	cloneOpts := make([]RunCmdOpt, 0, len(opts) + len(runCmdAdditionalOptions))
	cloneOpts = append(cloneOpts, opts...)

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

