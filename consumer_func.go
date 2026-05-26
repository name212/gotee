// Copyright 2026
// license that can be found in the LICENSE file.

package gotee

var _ Consumer = &FuncConsumer{}

type (
	Func      func([]byte) error
	FuncNoErr func([]byte)
)

// FuncConsumer
// Consummer wrapper around function
// WARNING! By default consumer not copy input
// For copy input before call function
// use WithCopyInput(true) after create consumer
type FuncConsumer struct {
	*privateBaseConsumer
}

func NewFuncConsumer(h Func, name ...string) *FuncConsumer {
	c := &FuncConsumer{}

	handler := func(input []byte) (int, error) {
		if err := h(input); err != nil {
			return 0, err
		}

		return len(input), nil
	}

    c.privateBaseConsumer = newPrivateBaseConsumer(handler, name...)

	return c
}

func NewFuncNoErrConsumer(h FuncNoErr, name ...string) *FuncConsumer {
	nameForSet := ConsumerName(1, name...)
	return NewFuncConsumer(
		func(b []byte) error {
			h(b)
			return nil
		},
		nameForSet,
	)
}

