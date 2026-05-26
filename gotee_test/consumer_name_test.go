// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	tee "github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

func TestConsumersDefaultName(t *testing.T) {
	buf := bytes.Buffer{}

	bufConsumer := tee.NewBufferConsumer(&buf)

	funcConsumer := tee.NewFuncConsumer(
		func(b []byte) error {
			return nil
		},
	)

	funcNoErrConsumer := tee.NewFuncNoErrConsumer(func(b []byte) {})

	lineHandlerConsumer := tee.NewLineConsumer(tee.NewStringsSliceLineHandler())

	lineFuncConsumer := tee.NewFuncLineConsumer(
		func(s string) error {
			return nil
		},
	)

	lineFuncNoErrConsumer := tee.NewFuncNoErrLineConsumer(func(s string) {})

	wc := newTestWriteCloser()

	writeCloserConsumer := tee.NewWriteCloserConsumer(wc)

	writerConsumer := tee.NewWriterConsumer(wc)

	customLinesConsumer := tee.NewCustomLineConsumer(&testNameDummyPartsHandler{})

	consumers := []struct {
		consumer tee.Consumer
		name     string
		line     int
	}{
		{
			name:     "BufferConsumer",
			consumer: bufConsumer,
			line:     19,
		},

		{
			name:     "FuncConsumer",
			consumer: funcConsumer,
			line:     21,
		},

		{
			name:     "FuncNoErrConsumer",
			consumer: funcNoErrConsumer,
			line:     27,
		},

		{
			name:     "LineConsumer",
			consumer: lineHandlerConsumer,
			line:     29,
		},

		{
			name:     "LineFuncConsumer",
			consumer: lineFuncConsumer,
			line:     31,
		},

		{
			name:     "LineFuncNoErrConsumer",
			consumer: lineFuncNoErrConsumer,
			line:     37,
		},

		{
			name:     "WriteCloserConsumer",
			consumer: writeCloserConsumer,
			line:     41,
		},

		{
			name:     "WriterConsumer",
			consumer: writerConsumer,
			line:     43,
		},

		{
			name:     "CustomLinesConsumer",
			consumer: customLinesConsumer,
			line:     45,
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
