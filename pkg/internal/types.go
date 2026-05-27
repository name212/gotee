// Copyright 2026
// license that can be found in the LICENSE file.

package internal

type (
	NoValT   = struct{}
	StopChan = chan NoValT
	OutChan  = chan []byte
	ErrChan  = chan error
)

var (
	NoVal = struct{}{}
)
