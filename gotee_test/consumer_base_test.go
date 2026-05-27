// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"fmt"
	"testing"

	"github.com/name212/gotee/pkg/tee/consumer"
	"github.com/stretchr/testify/require"
)

func TestBaseConsumerClose(t *testing.T) {
	writeOp := func(b []byte) (int, error) {
		return 0, nil
	}

	closedCalls := 0

	c := consumer.NewBaseConsumer("name", writeOp, func() error {
		closedCalls++
		return nil
	})

	retClosedCalls := func() int {
		return closedCalls
	}

	close := func() {
		_ = c.Close()
	}

	assertClosed(t, c, close, retClosedCalls)
}

func TestBaseConsumerCopyInput(t *testing.T) {
	assertCopy := func(t *testing.T, shouldCopy bool) {
		input := []byte("a")
		var got *[]byte
		c := consumer.NewBaseConsumer("copy test", func(b []byte) (int, error) {
			got = &b
			return len(b), nil
		})

		c.WithCopyInput(shouldCopy)

		_, _ = c.Write(input)

		require.NotNil(t, got, "should got bytes")

		require.Equal(t, input, *got, "should same input")

		inputAddress := fmt.Sprintf("%p", input)
		gotAddress := fmt.Sprintf("%p", *got)

		assert := require.Equal
		msg := "should same address"

		if shouldCopy {
			assert = require.NotEqual
			msg = "should different address"
		}

		assert(t, inputAddress, gotAddress, msg)
	}

	t.Run("no copy", func(t *testing.T) {
		assertCopy(t, false)
	})

	t.Run("should copy", func(t *testing.T) {
		assertCopy(t, true)
	})
}

func TestBaseConsumerWrite(t *testing.T) {
	type test struct {
		name   string
		called bool
		n      int
		err    error
	}

	createWriteOp := func(tst *test) consumer.ImplWriteFunc {
		return func(b []byte) (int, error) {
			tst.called = true
			return tst.n, tst.err
		}
	}

	tests := []*test{
		{
			name: "Call Write and return no error",
			n:    1,
			err:  nil,
		},

		{
			name: "Call Write and return error",
			n:    0,
			err:  fmt.Errorf("err"),
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			c := consumer.NewBaseConsumer("write test", createWriteOp(tst))
			n, err := c.Write([]byte("b"))

			require.True(t, tst.called, "should call Write")
			require.Equal(t, tst.n, n, "should return correct write bytes")
			require.Equal(t, tst.err, err, "should return correct error")
		})
	}
}

func TestBaseConsumerCloseCall(t *testing.T) {
	type test struct {
		name   string
		err    error
		called bool
	}

	createCloseOp := func(tst *test) consumer.ImplCloseFunc {
		return func() error {
			tst.called = true
			return tst.err
		}
	}

	tests := []*test{
		{
			name: "Call Close and return no error",
			err:  nil,
		},

		{
			name: "Call Close and return error",
			err:  fmt.Errorf("err"),
		},
	}

	writeOp := func(b []byte) (int, error) {
		return len(b), nil
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			c := consumer.NewBaseConsumer("close test", writeOp, createCloseOp(tst))
			_, _ = c.Write([]byte("b"))

			err := c.Close()

			require.True(t, tst.called, "should call Close")
			require.True(t, c.IsClosed(), "consumer should closed")
			require.Equal(t, tst.err, err, "should return correct error")
		})
	}

	t.Run("Close op not passsed not panics", func(t *testing.T) {
		c := consumer.NewBaseConsumer("close test", writeOp)
		_, _ = c.Write([]byte("b"))

		var err error
		closeFun := func() {
			err = c.Close()
		}

		require.NotPanics(t, closeFun, "should not panics without closeOp")
		require.True(t, c.IsClosed(), "consumer should closed")
		require.NoError(t, err, "should return no error without closeOp")
	})
}
