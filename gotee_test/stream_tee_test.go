// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/name212/gotee/pkg/tee"
	"github.com/name212/gotee/pkg/tee/stream"
	"github.com/stretchr/testify/require"
)

func TestTeeStreamInterruptFromConsumer(t *testing.T) {
	EnableDebugLogs(t)
	bufString := `First string
Second String
Third string`
	createBuf := func() *tee.ClosableReaderBuffer {
		b := bytes.NewBufferString(bufString)
		return tee.NewClosableReaderBuffer(b)
	}

	createErrConsumer := func(name string, cut string, err error) *TestWriteCloserConsumer {
		ct := []byte(cut)
		c := newTestWriteCloserConsumer(name)
		c.SetWriteErrChecker(func(b []byte) ([]byte, error) {
			if bytes.Contains(b, ct) {
				return ct, err
			}

			return ct, nil
		})

		return c
	}

	createClosedConsumer := func(name string, cut string) *TestWriteCloserConsumer {
		return createErrConsumer("closed_"+name, cut, tee.ErrClosed)
	}

	ctx := context.TODO()

	assertConsumers := func(t *testing.T, consumers map[*TestWriteCloserConsumer]string) {
		for c, expected := range consumers {
			require.Equal(t, expected, c.Content(), "consumer %s should have correct content", c.Name())
			require.True(t, c.IsClosed(), "consumer %s should be closed", c.Name())
		}
	}

	assertNoErrResults := func(t *testing.T, res *tee.Results) {
		require.Nil(t, res, "results should be nil")
		require.NoError(t, res.GetError(), "results should not get error")
		require.False(t, res.HasLeastOneError(), "results should not returns least one error")
		require.False(t, res.HasConsumersErrors(), "results should not have consumers errors")
		require.False(t, res.HasReadError(), "results should not have read error")
	}

	assertErrResults := func(t *testing.T, res *tee.Results, consumersErrs map[string]error) {
		require.NotNil(t, res, "results should presents")
		require.Error(t, res.GetError(), "results should get error")
		require.Error(t, res, "results should be error")
		require.True(t, res.HasLeastOneError(), "results should returns least one error")
		require.True(t, res.HasConsumersErrors(), "results should have consumers errors")
		require.False(t, res.HasReadError(), "results should not have read error")
		require.NoError(t, res.ReadErr, "results should not have read error val")
		require.Len(t, res.ConsumersErrs, len(consumersErrs), "results should have all consumers errors")

		for c, expected := range consumersErrs {
			require.Contains(t, res.ConsumersErrs, c, "results ConsumersErrs should contais consumer %s", c)
			require.ErrorIs(t, res.ConsumersErrs[c], expected, "consumer %s should have correct error", c)
		}
	}

	type streamConstructor func(t *testing.T, r io.Reader, c ...tee.Consumer) *stream.TeeStream

	createAssertionsErrConsumers := func(consumerParams map[string][]string) ([]tee.Consumer, map[*TestWriteCloserConsumer]string, map[string]error) {
		consumers := make([]tee.Consumer, 0, len(consumerParams))

		consumersToAsserts := make(map[*TestWriteCloserConsumer]string)
		consumersErrs := make(map[string]error)

		for name, params := range consumerParams {
			err := fmt.Errorf("%s", params[1])
			c := createErrConsumer(name, params[0], err)
			consumersToAsserts[c] = params[2]
			consumersErrs[name] = err
			consumers = append(consumers, c)
		}

		return consumers, consumersToAsserts, consumersErrs
	}

	doAllInterruptedConsumers := func(t *testing.T, constructor streamConstructor) {
		tests := []struct {
			name   string
			assert func(t *testing.T, constructor streamConstructor) *stream.TeeStream
		}{
			{
				name: "all consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					first := createClosedConsumer("mul_all_close_first", "st str")
					second := createClosedConsumer("mul_all_close_second", "nd S")
					third := createClosedConsumer("mul_all_close_third", "Third ")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*TestWriteCloserConsumer]string{
						first:  "Fir",
						second: "First string\nSeco",
						third:  "First string\nSecond String\n",
					})

					return stream
				},
			},

			{
				name: "one consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					first := newTestWriteCloserConsumer("mul_one_close_first")
					second := createClosedConsumer("mul_one_close_closed", "nd S")
					third := newTestWriteCloserConsumer("mul_one_close_third")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*TestWriteCloserConsumer]string{
						first:  bufString,
						second: "First string\nSeco",
						third:  bufString,
					})

					return stream
				},
			},

			{
				name: "multiple consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					first := newTestWriteCloserConsumer("mul_one_close_first")
					second := createClosedConsumer("mul_one_close_closed", "nd S")
					third := createClosedConsumer("mul_one_close_closed", "First")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*TestWriteCloserConsumer]string{
						first:  bufString,
						second: "First string\nSeco",
						third:  "",
					})

					return stream
				},
			},

			{
				name: "single consumer closed",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					c := createClosedConsumer("one_close_closed", "nd S")

					buf := createBuf()

					stream := constructor(t, buf, c)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*TestWriteCloserConsumer]string{
						c: "First string\nSeco",
					})

					return stream
				},
			},

			{
				name: "all consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"mul_all_err_first": {
							"st str",
							"first_err",
							"Fir",
						},
						"mul_all_err_second": {
							"nd S",
							"second_err",
							"First string\nSeco",
						},
						"mul_all_err_third": {
							"Third ",
							"third_err",
							"First string\nSecond String\n",
						},
					})

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},

			{
				name: "one consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"mul_one_err": {
							"t str",
							"first_err",
							"Firs",
						},
					})

					second := newTestWriteCloserConsumer("mul_one_error_second")
					consumersToAsserts[second] = bufString

					third := newTestWriteCloserConsumer("mul_one_error_third")
					consumersToAsserts[third] = bufString

					consumers = append(consumers, second, third)

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},

			{
				name: "multiple consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"mul_mul_err_first": {
							"t str",
							"first_err",
							"Firs",
						},
						"mul_mul_err_second": {
							"t str",
							"first_err",
							"Firs",
						},
					})

					third := newTestWriteCloserConsumer("mul_one_error_third")
					consumersToAsserts[third] = bufString

					consumers = append(consumers, third)

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},

			{
				name: "single consumer in error",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"one_err": {
							"rd",
							"first_err",
							"First string\nSecond String\nThi",
						},
					})

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},

			{
				name: "one consumer in error anothers closed",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"mixed_one_err_err": {
							"\nSecond",
							"first_err",
							"First string",
						},
					})

					second := createClosedConsumer("mixed_one_err_closed_second", "nd S")
					consumersToAsserts[second] = "First string\nSeco"

					third := createClosedConsumer("mixed_one_err_closed_third", "\n")
					consumersToAsserts[third] = "First string"

					consumers = append(consumers, second, third)

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},

			{
				name: "one consumer closed anothers in error",
				assert: func(t *testing.T, constructor streamConstructor) *stream.TeeStream {
					consumers, consumersToAsserts, consumersErrs := createAssertionsErrConsumers(map[string][]string{
						"mixed_one_closed_err_first": {
							"g\nSecond",
							"first_err",
							"First strin",
						},
						"mixed_one_closed_err_second": {
							"rst strin",
							"second_err",
							"Fi",
						},
					})

					closed := createClosedConsumer("mixed_one_closed_closed", "ing\n")
					consumersToAsserts[closed] = "First str"

					consumers = append(consumers, closed)

					buf := createBuf()

					stream := constructor(t, buf, consumers...)
					res := stream.Run(ctx)

					assertErrResults(t, res, consumersErrs)

					assertConsumers(t, consumersToAsserts)

					return stream
				},
			},
		}

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				stream := tst.assert(t, constructor)
				assertNoReadGourutine(t, stream)
				AssertNoTeeGorutines(t, nil)
			})
		}
	}

	constructors := []struct {
		name        string
		constructor streamConstructor
	}{
		{
			name: "Small read buffer",
			constructor: func(t *testing.T, r io.Reader, c ...tee.Consumer) *stream.TeeStream {
				stream, err := stream.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("small")
				stream.WithReadBufSize(1)
				addClosableBufferToStream(stream, r)
				return stream
			},
		},

		{
			name: "Small read buffer and unbuffered write",
			constructor: func(t *testing.T, r io.Reader, c ...tee.Consumer) *stream.TeeStream {
				stream, err := stream.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("small unbuf")
				stream.WithReadBufSize(1)
				stream.WithWritesBufferedCount(0)
				addClosableBufferToStream(stream, r)
				return stream
			},
		},

		{
			name: "Big read buffer",
			constructor: func(t *testing.T, r io.Reader, c ...tee.Consumer) *stream.TeeStream {
				stream, err := stream.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("big")
				stream.WithReadBufSize(64 * 1024)
				addClosableBufferToStream(stream, r)
				return stream
			},
		},

		{
			name: "Big read buffer and unbuffered write",
			constructor: func(t *testing.T, r io.Reader, c ...tee.Consumer) *stream.TeeStream {
				stream, err := stream.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("big unbuf")
				stream.WithReadBufSize(64 * 1024)
				stream.WithWritesBufferedCount(0)
				addClosableBufferToStream(stream, r)
				return stream
			},
		},
	}

	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			doAllInterruptedConsumers(t, c.constructor)
		})
	}
}

func TestTeeStreamStop(t *testing.T) {
	EnableDebugLogs(t)
	bufString := `First string
Second String
Third string`

	t.Run("Stop stream before read all", func(t *testing.T) {
		reader := NewTestReader(bufString)
		reader.WithSleep(2 * time.Second)

		c := newTestWriteCloserConsumer("not_full")

		stream, err := stream.NewTeeStream(reader, c)
		require.NoError(t, err, "stream should created")
		stream.WithName("rstop_before_read")
		stream.WithReadBufSize(1)
		addClosableBufferToStream(stream, reader)

		go func() {
			time.Sleep(2 * time.Second)
			stream.Stop()
		}()

		res := stream.Run(context.TODO())
		require.Nil(t, res, "results should nil")
		assertNoReadGourutine(t, stream)
		AssertNoTeeGorutines(t, nil)
	})

}

func addClosableBufferToStream(s *stream.TeeStream, r io.Reader) {
	bc, ok := r.(*tee.ClosableReaderBuffer)
	if ok {
		s.WithBeforeStop(tee.CloserBeforeStop(bc))
	}
}

func assertNoReadGourutine(t *testing.T, s *stream.TeeStream) {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	start := time.Now().UnixNano()
	err := s.WaitReadEnd(ctx)
	end := time.Now().UnixNano()
	waitTime := end - start
	t.Logf("WaitReadEnd time for %s: %s", s.GetName(), time.Duration(waitTime).String())
	require.NoError(t, err, "read gourutine should stop")
}
