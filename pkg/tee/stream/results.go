// Copyright 2026
// license that can be found in the LICENSE file.

package stream

import tee "github.com/name212/gotee/pkg/tee"

func newStoppedResults() *tee.Results {
	return &tee.Results{
		ReadErr:       tee.ErrStreamStopped,
		ConsumersErrs: make(tee.ConsumersErrors),
	}
}

func newAlreadyStartedResults() *tee.Results {
	return &tee.Results{
		ReadErr:       tee.ErrStreamAlreadyStarted,
		ConsumersErrs: make(tee.ConsumersErrors),
	}
}
