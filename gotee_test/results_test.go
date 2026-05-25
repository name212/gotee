// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"testing"

	"github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

func TestResultNilNotPanic(t *testing.T) {
	type test struct {
		name string
		call func(t *testing.T)
	}

	var nilResults *gotee.Results

	tests := []test{
		{
			name: "HasReadError",
			call: func(t *testing.T) {
				r := nilResults.HasReadError()
				require.False(t, r)
			},
		},

		{
			name: "HasConsumersErrors",
			call: func(t *testing.T) {
				r := nilResults.HasConsumersErrors()
				require.False(t, r)
			},
		},

		{
			name: "HasLeastOneError",
			call: func(t *testing.T) {
				r := nilResults.HasLeastOneError()
				require.False(t, r)
			},
		},

		{
			name: "GetError",
			call: func(t *testing.T) {
				err := nilResults.GetError()
				require.NoError(t, err)
			},
		},

		{
			name: "Error",
			call: func(t *testing.T) {
				e := nilResults.Error()
				require.Empty(t, e)
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			require.NotPanics(t, func(){
				tst.call(t)
			}, "should not panics")
		})
	}
}
