// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/name212/gotee/pkg/tee"
	"github.com/name212/gotee/pkg/tee/consumer"
	"github.com/stretchr/testify/require"
)

func TestConsumersDefaultName(t *testing.T) {
	buf := bytes.Buffer{}

	bufConsumer := consumer.NewBufferConsumer(&buf)

	funcConsumer := consumer.NewFuncConsumer(
		func(b []byte) error {
			return nil
		},
	)

	funcNoErrConsumer := consumer.NewFuncNoErrConsumer(func(b []byte) {})

	lineHandlerConsumer := consumer.NewLineConsumer(consumer.NewStringsSliceLineHandler())

	lineFuncConsumer := consumer.NewFuncLineConsumer(
		func(s string) error {
			return nil
		},
	)

	lineFuncNoErrConsumer := consumer.NewFuncNoErrLineConsumer(func(s string) {})

	wc := NewTestWriteCloser()

	writeCloserConsumer := consumer.NewWriteCloserConsumer(wc)

	writerConsumer := consumer.NewWriterConsumer(wc)

	customLinesConsumer := consumer.NewCustomLineConsumer(&testNameDummyPartsHandler{})

	consumers := []struct {
		consumer tee.Consumer
		name     string
		line     int
	}{
		{
			name:     "BufferConsumer",
			consumer: bufConsumer,
			line:     20,
		},

		{
			name:     "FuncConsumer",
			consumer: funcConsumer,
			line:     22,
		},

		{
			name:     "FuncNoErrConsumer",
			consumer: funcNoErrConsumer,
			line:     28,
		},

		{
			name:     "LineConsumer",
			consumer: lineHandlerConsumer,
			line:     30,
		},

		{
			name:     "LineFuncConsumer",
			consumer: lineFuncConsumer,
			line:     32,
		},

		{
			name:     "LineFuncNoErrConsumer",
			consumer: lineFuncNoErrConsumer,
			line:     38,
		},

		{
			name:     "WriteCloserConsumer",
			consumer: writeCloserConsumer,
			line:     42,
		},

		{
			name:     "WriterConsumer",
			consumer: writerConsumer,
			line:     44,
		},

		{
			name:     "CustomLinesConsumer",
			consumer: customLinesConsumer,
			line:     46,
		},
	}

	for _, c := range consumers {
		t.Run(c.name, func(t *testing.T) {
			assertName(t, c.consumer.Name(), c.line)
		})
	}
}

func assertName(t *testing.T, name string, line int) {
	require.NotEmpty(t, name, "name for consumer should not empty")

	re := regexp.MustCompile(
		fmt.Sprintf(`[\-0-9]{1,20}: .*consumer_name_test\.go:%d`, line),
	)

	matched := re.MatchString(name)
	require.True(
		t,
		matched,
		"'%s' for consumer should match to '%s'",
		name,
		re.String(),
	)
}

type testNameDummyPartsHandler struct{}

func (h *testNameDummyPartsHandler) Handle(part []byte, unhandled bool, last bool, scanErr bool) error {
	return nil
}
