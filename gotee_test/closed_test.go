// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"testing"

	"github.com/name212/gotee/pkg/tee"
	"github.com/stretchr/testify/require"
)

func TestClosedFlag(t *testing.T) {
	flag := tee.NewClosedFlag()
	closedCalls := 0

	close := func() {
		if flag.SetClosed() {
			return
		}
		closedCalls++
	}

	assertClosed(t, flag, close, func() int {
		return closedCalls
	})
}

func assertClosed(t *testing.T, c closed, close func(), closedCalls func() int) {
	assertOneCloseCall := func(t *testing.T, msg string) {
		require.Equal(t, 1, closedCalls(), "should one Close call: %s", msg)
	}

	beforeClose := c.IsClosed()
	require.False(t, beforeClose, "should not closed before first SetClose")

	close()
	assertOneCloseCall(t, "first SetClosed should return that should close")

	afterClose := c.IsClosed()
	require.True(t, afterClose, "should return closed after first SetClose")

	close()
	assertOneCloseCall(t, "second SetClosed should return that should not close")

	afterSecondClose := c.IsClosed()
	require.True(t, afterSecondClose, "should return closed after second SetClose")
}

type closed interface {
	IsClosed() bool
}
