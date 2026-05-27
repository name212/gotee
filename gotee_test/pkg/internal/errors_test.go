// Copyright 2026
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrors(t *testing.T) {
	type test struct {
		name     string
		resErr   error
		toAppend []error
		expected []error
	}

	var (
		errFirst  = fmt.Errorf("first")
		errSecond = fmt.Errorf("second")
		errThird  = fmt.Errorf("third")
	)

	var customNil error = &customErr{}
	customNil = nil

	var custom error = &customErr{}

	tests := []*test{
		{
			name:     "resErr nil no to append",
			resErr:   nil,
			toAppend: nil,
			expected: nil,
		},

		{
			name:     "resErr not nil no to append",
			resErr:   errFirst,
			toAppend: nil,
			expected: []error{errFirst},
		},

		{
			name:     "resErr not nil to append one nil",
			resErr:   errFirst,
			toAppend: []error{errSecond},
			expected: []error{errFirst, errSecond},
		},

		{
			name:     "resErr not nil to append one not nil",
			resErr:   errFirst,
			toAppend: []error{nil, errSecond},
			expected: []error{errFirst, errSecond},
		},

		{
			name:     "resErr not nil to append multiple one not nil",
			resErr:   errFirst,
			toAppend: []error{errSecond, nil, nil},
			expected: []error{errFirst, errSecond},
		},

		{
			name:     "resErr not nil to append all not nil",
			resErr:   errFirst,
			toAppend: []error{errSecond, nil, errThird},
			expected: []error{errFirst, errSecond, errThird},
		},

		{
			name:     "resErr nil to append multiple not nil",
			resErr:   nil,
			toAppend: []error{errSecond, nil, errThird},
			expected: []error{errSecond, errThird},
		},

		{
			name:     "resErr nil to append multiple all not nil",
			resErr:   nil,
			toAppend: []error{errSecond, errThird},
			expected: []error{errSecond, errThird},
		},

		{
			name:     "resErr nil to append multiple first nil",
			resErr:   nil,
			toAppend: []error{nil, errThird},
			expected: []error{errThird},
		},

		{
			name:     "resErr nil to append multiple all nil",
			resErr:   nil,
			toAppend: []error{nil, nil},
			expected: nil,
		},

		{
			name:     "resErr nil to append multiple one nil",
			resErr:   nil,
			toAppend: []error{nil},
			expected: nil,
		},

		{
			name:     "resErr custom nil to append multiple one nil",
			resErr:   customNil,
			toAppend: []error{nil},
			expected: nil,
		},

		{
			name:     "resErr custom not nil to append multiple one nil",
			resErr:   custom,
			toAppend: []error{nil},
			expected: []error{&customErr{}},
		},

		{
			name:     "resErr not nil to append multiple one custom nil",
			resErr:   errFirst,
			toAppend: []error{errSecond, customNil, errThird},
			expected: []error{errFirst, errSecond, errThird},
		},

		{
			name:     "resErr not nil to append one custom nil",
			resErr:   errFirst,
			toAppend: []error{customNil},
			expected: []error{errFirst},
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			res := AppendErr(tst.resErr, tst.toAppend...)
			if len(tst.expected) == 0 {
				require.Nil(t, res, "should nil")
				require.NoError(t, res, "should not error")
				return
			}

			require.NotNil(t, res, "should not nil")
			require.Error(t, res, "should error")

			for _, expected := range tst.expected {
				require.ErrorIs(t, res, expected, "should error is %s", expected.Error())
			}
		})
	}
}

type customErr struct{}

func (e *customErr) Error() string {
	return "custom"
}
