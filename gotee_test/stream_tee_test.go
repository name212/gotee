// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

func TestTeeStreamInterrupt(t *testing.T) {
	enableDebugLogs(t)
	bufString := `First string
Second String
Third string`
	createBuf := func() *bytes.Buffer {
		return bytes.NewBufferString(bufString)
	}

	createErrConsumer := func(name string, cut string, err error) *testWriteCloserConsumer {
		ct := []byte(cut)
		c := newTestWriteCloserConsumer(name)
		c.setWriteErrChecker(func(b []byte) ([]byte, error) {
			if bytes.Contains(b, ct) {
				return ct, err
			}

			return ct, nil
		})

		return c
	}

	createClosedConsumer := func(name string, cut string) *testWriteCloserConsumer {
		return createErrConsumer("closed_"+name, cut, gotee.ErrClosed)
	}

	ctx := context.TODO()

	assertConsumers := func(t *testing.T, consumers map[*testWriteCloserConsumer]string) {
		for c, expected := range consumers {
			require.Equal(t, expected, c.content(), "consumer %s should have correct content", c.Name())
			require.True(t, c.IsClosed(), "consumer %s should be closed", c.Name())
		}
	}

	assertNoErrResults := func(t *testing.T, res *gotee.Results) {
		require.Nil(t, res, "results should be nil")
		require.NoError(t, res.GetError(), "results should not get error")
		require.False(t, res.HasLeastOneError(), "results should not returns least one error")
		require.False(t, res.HasConsumersErrors(), "results should not have consumers errors")
		require.False(t, res.HasReadError(), "results should not have read error")
	}

	assertErrResults := func(t *testing.T, res *gotee.Results, consumersErrs map[string]error) {
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

	type streamConstructor func(t *testing.T, r io.Reader, c ...gotee.Consumer) *gotee.TeeStream

	createAssertionsErrConsumers := func(consumerParams map[string][]string) ([]gotee.Consumer, map[*testWriteCloserConsumer]string, map[string]error) {
		consumers := make([]gotee.Consumer, 0, len(consumerParams))

		consumersToAsserts := make(map[*testWriteCloserConsumer]string)
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
			assert func(t *testing.T, constructor streamConstructor)
		}{
			{
				name: "all consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) {
					first := createClosedConsumer("mul_all_close_first", "st str")
					second := createClosedConsumer("mul_all_close_second", "nd S")
					third := createClosedConsumer("mul_all_close_third", "Third ")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*testWriteCloserConsumer]string{
						first:  "Fir",
						second: "First string\nSeco",
						third:  "First string\nSecond String\n",
					})
				},
			},

			{
				name: "one consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) {
					first := newTestWriteCloserConsumer("mul_one_close_first")
					second := createClosedConsumer("mul_one_close_closed", "nd S")
					third := newTestWriteCloserConsumer("mul_one_close_third")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*testWriteCloserConsumer]string{
						first:  bufString,
						second: "First string\nSeco",
						third:  bufString,
					})
				},
			},

			{
				name: "multiple consumers closed",
				assert: func(t *testing.T, constructor streamConstructor) {
					first := newTestWriteCloserConsumer("mul_one_close_first")
					second := createClosedConsumer("mul_one_close_closed", "nd S")
					third := createClosedConsumer("mul_one_close_closed", "First")

					buf := createBuf()

					stream := constructor(t, buf, first, second, third)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*testWriteCloserConsumer]string{
						first:  bufString,
						second: "First string\nSeco",
						third:  "",
					})
				},
			},

			{
				name: "single consumer closed",
				assert: func(t *testing.T, constructor streamConstructor) {
					c := createClosedConsumer("one_close_closed", "nd S")

					buf := createBuf()

					stream := constructor(t, buf, c)
					res := stream.Run(ctx)

					assertNoErrResults(t, res)

					assertConsumers(t, map[*testWriteCloserConsumer]string{
						c: "First string\nSeco",
					})
				},
			},

			{
				name: "all consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},

			{
				name: "one consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},

			{
				name: "multiple consumers in error",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},

			{
				name: "single consumer in error",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},

			{
				name: "one consumer in error anothers closed",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},

			{
				name: "one consumer closed anothers in error",
				assert: func(t *testing.T, constructor streamConstructor) {
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
				},
			},
		}

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				tst.assert(t, constructor)
			})
		}
	}

	constructors := []struct {
		name        string
		constructor streamConstructor
	}{
		{
			name: "Small read buffer",
			constructor: func(t *testing.T, r io.Reader, c ...gotee.Consumer) *gotee.TeeStream {
				stream, err := gotee.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("small")
				stream.WithReadBufSize(1)
				return stream
			},
		},

		{
			name: "Small read buffer and unbuffered write",
			constructor: func(t *testing.T, r io.Reader, c ...gotee.Consumer) *gotee.TeeStream {
				stream, err := gotee.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("small unbuf")
				stream.WithReadBufSize(1)
				stream.WithWritesBufferedCount(0)
				return stream
			},
		},

		{
			name: "Big read buffer",
			constructor: func(t *testing.T, r io.Reader, c ...gotee.Consumer) *gotee.TeeStream {
				stream, err := gotee.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("big")
				stream.WithReadBufSize(64 * 1024)
				return stream
			},
		},

		{
			name: "Big read buffer and unbuffered write",
			constructor: func(t *testing.T, r io.Reader, c ...gotee.Consumer) *gotee.TeeStream {
				stream, err := gotee.NewTeeStream(r, c...)
				require.NoError(t, err, "stream should created")
				stream.WithName("big unbuf")
				stream.WithReadBufSize(64 * 1024)
				stream.WithWritesBufferedCount(0)
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
