// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/name212/gotee"
	tee "github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

var (
	_ tee.Consumer   = &testWriteCloserConsumer{}
	_ io.WriteCloser = &testWriteCloser{}
	_ io.Writer      = &testWriteCloser{}
)

var (
	testsBaseDir = filepath.Join(os.TempDir(), "tests-go-tee")
)

type testWriteCloserConsumer struct {
	*tee.BaseConsumer

	mu                sync.Mutex
	buf               *bytes.Buffer
	writeErrorChecker func([]byte) ([]byte, error)
	writeErr          error
	closeErr          error
}

func newTestWriteCloserConsumer(name string) *testWriteCloserConsumer {
	return &testWriteCloserConsumer{
		BaseConsumer: tee.NewBaseConsumer(name),
		buf:          &bytes.Buffer{},
	}
}

func (c *testWriteCloserConsumer) Write(p []byte) (int, error) {
	if c.IsClosed() {
		return 0, tee.ErrClosed
	}

	c.checkErr(p)

	if err := c.getWriteErr(); err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.Write(p)
}

func (c *testWriteCloserConsumer) Close() error {
	if c.SetClosed() {
		return nil
	}

	return c.getCloseErr()
}

func (c *testWriteCloserConsumer) content() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.String()
}

func (c *testWriteCloserConsumer) getWriteErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.writeErr
}

func (c *testWriteCloserConsumer) setWriteErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.writeErr = err
}

func (c *testWriteCloserConsumer) checkErr(input []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeErrorChecker != nil {
		bt := c.buf.Bytes()
		bt = gotee.CopyBytes(bt)
		bt = append(bt, input...)
		if cut, err := c.writeErrorChecker(bt); err != nil {
			c.buf.Write(input)
			all := c.buf.Bytes()
			indx := bytes.Index(all, cut)
			all = all[0:indx]
			c.buf.Reset()
			c.buf.Write(all)
			c.writeErr = err
		}
	}
}

func (c *testWriteCloserConsumer) setWriteErrChecker(ch func([]byte) ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.writeErrorChecker = ch
}

func (c *testWriteCloserConsumer) getCloseErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closeErr
}

func (c *testWriteCloserConsumer) setCloseErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeErr = err
}

type testWriteCloser struct {
	base *testWriteCloserConsumer
}

func newTestWriteCloser() *testWriteCloser {
	return &testWriteCloser{
		base: newTestWriteCloserConsumer(""),
	}
}

func (c *testWriteCloser) Write(p []byte) (int, error) {
	return c.base.Write(p)
}

func (c *testWriteCloser) Close() error {
	return c.base.Close()
}

func randString(seed string) string {
	n := rand.NewSource(time.Now().UnixNano()).Int63()

	all := fmt.Sprintf("%s%d", seed, n)

	hash := md5.Sum([]byte(all))

	res := fmt.Sprintf("%x", hash)

	return fmt.Sprintf("%.10s", res)
}

func writeScript(t *testing.T, name, content string) string {
	err := os.MkdirAll(testsBaseDir, 0o777)
	require.NoError(t, err, "base tests dir %s should create", testsBaseDir)

	randStr := randString(content)

	fullName := fmt.Sprintf(
		"%s.%s.sh",
		name,
		randStr,
	)

	fullPath := filepath.Join(testsBaseDir, fullName)

	err = os.WriteFile(fullPath, []byte(content), 0o777)
	require.NoError(t, err, "script %s should write to %s", name, fullPath)

	return fullPath
}

func enableDebugLogs(t *testing.T) {
	t.Setenv("GO_TEE_ENABLE_DEBUG_LOG", "true")
	t.Setenv("GO_TEE_DEBUG_LOG_FULL_BUFF", "true")
}


func assertNoTeeGorutines(t *testing.T, additionals map[string]string) {
	t.Log("wait 100 ms before assert call stack...")
	time.Sleep(100 * time.Millisecond)

	runtime.Gosched()

	buf := make([]byte, 128 * 1024)
	n := runtime.Stack(buf, true)

	bufStr := string(buf[:n])

	contains := map[string]string{
		"io wait": "[IO wait]",
		"wait group": "[sync.WaitGroup.Wait]",
		"pipe reader": "io.(*PipeReader)",
		"created combine stream": "created by github.com/name212/gotee.(*CombineStream).Run",
		"created by tee stream": "created by github.com/name212/gotee.(*TeeStream).Run",
		"internal pipe": "github.com/name212/gotee.(*pipe)",
		"not gotee": "github.com/name212/gotee.",
	}

	if len(additionals) > 0 {
		for k, v := range additionals {
			contains[k] = v
		}
	}

	for msg, c := range contains {
		require.NotContains(t, bufStr, c, "should not contans %s in call stack\n---\n%s---\n", msg, bufStr)
	}
}