// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

func TestLineConsumer(t *testing.T) {
	enableDebugLogs(t)

	t.Run("own handler", func(t *testing.T) {
		handler := gotee.NewStringsSliceLineHandler(5)
		c := gotee.NewLineConsumer(handler, "slice_handler")
		writeToLineConsumer(t, c)
		assertLineConsumer(t, handler.Lines())
	})

	t.Run("func handler", func(t *testing.T) {
		lines := make([]string, 0, 5)
		handler := func(l string) error {
			lines = append(lines, l)
			return nil
		}

		c := gotee.NewLineConsumer(gotee.NewFuncLineHandler(handler), "func_handler")
		writeToLineConsumer(t, c)
		assertLineConsumer(t, lines)
	})

	t.Run("func handler with err", func(t *testing.T) {
		lines := make([]string, 0, 3)

		calls := 0

		const errStep = 3

		handler := func(l string) error {
			if calls == errStep {
				return fmt.Errorf("my error")
			}

			lines = append(lines, l)
			calls++
			return nil
		}

		c := gotee.NewLineConsumer(gotee.NewFuncLineHandler(handler), "func_err_handler")

		linesToWrite := []string{
			"First\n",
			"Second\n",
			"Third\n",
			"Four\n",
			"Five\n",
		}

		for i := range linesToWrite {
			w := linesToWrite[i]
			n, err := c.Write([]byte(w))
			// next after error should write
			// because line consumer use gorutine
			if i < errStep {
				require.NoError(t, err, "should write '%s'", w)
				require.Equal(t, len(w), n, "should write in one part")
				continue
			}

			require.Error(t, err, "should not write '%s'", w)
			require.Equal(t, 0, n, "should no write any bytes")
		}

		err := c.Close()
		require.NoError(t, err, "should closed without error")
		require.True(t, c.IsClosed(), "should closed")

		require.Len(t, lines, errStep, "should consume all before error")

		for i, e := range linesToWrite[:errStep] {
			require.Equal(t, strings.TrimSpace(e), lines[i], "should write correct: %d", i)
		}
	})

	t.Run("func no err handler", func(t *testing.T) {
		lines := make([]string, 0, 5)
		handler := func(l string) {
			lines = append(lines, l)
		}

		c := gotee.NewLineConsumer(gotee.NewFuncNoErrLineHandler(handler), "func_no_err_handler")
		writeToLineConsumer(t, c)
		assertLineConsumer(t, lines)
	})
}

func writeToLineConsumer(t *testing.T, c *gotee.SplitConsumer) {
	toWrite := []string{
		"Fir",
		"st",
		" line\n",
		"Second line\n",
		"\n",
		"Four line",
		"\n",
		"No line at end",
	}

	for _, w := range toWrite {
		n, err := c.Write([]byte(w))
		require.NoError(t, err, "should write '%s'", w)
		require.Equal(t, len(w), n, "should write in one part")
	}

	err := c.Close()
	require.NoError(t, err, "consumer should be closed without error")
	require.True(t, c.IsClosed(), "consumer should be closed")
}

func assertLineConsumer(t *testing.T, lines []string) {
	toAssert := []string{
		"First line",
		"Second line",
		"",
		"Four line",
		"No line at end",
	}

	require.Len(t, lines, len(toAssert), "should have all lines")

	for i, l := range toAssert {
		require.Equal(t, l, lines[i], "line %d should be equal", i)
	}
}
