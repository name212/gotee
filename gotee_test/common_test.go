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

	tee "github.com/name212/gotee/pkg/tee"
	"github.com/name212/gotee/pkg/tee/consumer"
	"github.com/stretchr/testify/require"
)

var (
	_ tee.Consumer   = &TestWriteCloserConsumer{}
	_ io.WriteCloser = &TestWriteCloser{}
	_ io.Writer      = &TestWriteCloser{}
)

var (
	testsBaseDir = filepath.Join(os.TempDir(), "tests-go-tee")
)

type TestWriteCloserConsumer struct {
	*consumer.BaseConsumer

	mu                sync.Mutex
	buf               *bytes.Buffer
	writeErrorChecker func([]byte) ([]byte, error)
	writeErr          error
	closeErr          error
}

func newTestWriteCloserConsumer(name string) *TestWriteCloserConsumer {
	c := &TestWriteCloserConsumer{
		buf: &bytes.Buffer{},
	}

	c.BaseConsumer = consumer.NewBaseConsumer(name, c.write, c.close)

	return c
}

func (c *TestWriteCloserConsumer) write(p []byte) (int, error) {
	c.checkErr(p)

	if err := c.getWriteErr(); err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.Write(p)
}

func (c *TestWriteCloserConsumer) close() error {
	return c.getCloseErr()
}

func (c *TestWriteCloserConsumer) Content() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.buf.String()
}

func (c *TestWriteCloserConsumer) getWriteErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.writeErr
}

func (c *TestWriteCloserConsumer) setWriteErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.writeErr = err
}

func (c *TestWriteCloserConsumer) checkErr(input []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeErrorChecker != nil {
		bt := c.buf.Bytes()
		bt = tee.CopyBytes(bt)
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

func (c *TestWriteCloserConsumer) SetWriteErrChecker(ch func([]byte) ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.writeErrorChecker = ch
}

func (c *TestWriteCloserConsumer) getCloseErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closeErr
}

func (c *TestWriteCloserConsumer) setCloseErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeErr = err
}

type TestWriteCloser struct {
	base *TestWriteCloserConsumer
}

func NewTestWriteCloser() *TestWriteCloser {
	return &TestWriteCloser{
		base: newTestWriteCloserConsumer(""),
	}
}

func (c *TestWriteCloser) Write(p []byte) (int, error) {
	return c.base.Write(p)
}

func (c *TestWriteCloser) Close() error {
	return c.base.Close()
}

type TestReader struct {
	buf            *tee.ClosableReaderBuffer
	sleepTime      time.Duration
	maxReadSymbols int
	failAfter      int
	failErr        error
	mu             sync.Mutex
	readed         int
}

func NewTestReader(content string) *TestReader {
	buf := bytes.NewBufferString(content)
	return &TestReader{
		buf:            tee.NewClosableReaderBuffer(buf),
		maxReadSymbols: -1,
		failAfter:      -1,
	}
}

func (r *TestReader) WithSleep(t time.Duration) *TestReader {
	r.sleepTime = t
	return r
}

func (r *TestReader) WithMaxSymbols(s int) *TestReader {
	r.maxReadSymbols = s
	return r
}

func (r *TestReader) WithFailAfter(a int, err error) *TestReader {
	r.failAfter = a
	r.failErr = err
	return r
}

func (r *TestReader) Read(p []byte) (int, error) {
	if r.sleepTime > 0 {
		time.Sleep(r.sleepTime)
	}

	toRead := p

	bufferChanged := false
	if r.maxReadSymbols > 0 && len(p) < r.maxReadSymbols {
		bufferChanged = true
		toRead = make([]byte, r.maxReadSymbols)
	}

	n, err := r.buf.Read(toRead)
	r.readed += n

	if bufferChanged {
		copy(p, toRead)
	}

	if err != nil {
		return n, err
	}

	if r.failAfter >= 0 && r.readed >= r.failAfter {
		return n, r.failErr
	}

	return n, nil
}

func (b *TestReader) Close() error {
	return b.buf.Close()
}

func RandString(seed string) string {
	n := rand.NewSource(time.Now().UnixNano()).Int63()

	all := fmt.Sprintf("%s%d", seed, n)

	hash := md5.Sum([]byte(all))

	res := fmt.Sprintf("%x", hash)

	return fmt.Sprintf("%.10s", res)
}

func WriteScript(t *testing.T, name, content string) string {
	err := os.MkdirAll(testsBaseDir, 0o777)
	require.NoError(t, err, "base tests dir %s should create", testsBaseDir)

	randStr := RandString(content)

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

func EnableDebugLogs(t *testing.T) {
	t.Setenv("GO_TEE_ENABLE_DEBUG_LOG", "true")
	t.Setenv("GO_TEE_DEBUG_LOG_FULL_BUFF", "true")
}

func AssertNoTeeGorutines(t *testing.T, additionals map[string]string) {
	t.Log("wait 100 ms before assert call stack...")
	time.Sleep(100 * time.Millisecond)

	runtime.Gosched()

	buf := make([]byte, 128*1024)
	n := runtime.Stack(buf, true)

	bufStr := string(buf[:n])

	contains := map[string]string{
		"io wait":                "[IO wait]",
		"wait group":             "[sync.WaitGroup.Wait]",
		"pipe reader":            "io.(*PipeReader)",
		"created combine stream": "created by github.com/name212/gotee.(*CombineStream).Run",
		"created by tee stream":  "created by github.com/name212/gotee.(*TeeStream).Run",
		"internal pipe":          "github.com/name212/gotee.(*pipe)",
		"not gotee":              "github.com/name212/gotee.",
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
