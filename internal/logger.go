// Copyright 2026
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"os"
	"strings"
)

type Logger interface {
	Log(string, ...any)
	LogBuf([]byte, int, string, ...any)
}

func GetDebugLogger(targets ...string) Logger {
	if e := os.Getenv("GO_TEE_ENABLE_DEBUG_LOG"); e == "" {
		return emptyLoggerInstance
	}

	logBuf := os.Getenv("GO_TEE_DEBUG_LOG_FULL_BUFF") != ""

	return newFmtLogger(logBuf, targets...)
}

type emptyLogger struct{}

var emptyLoggerInstance = &emptyLogger{}

func (l *emptyLogger) Log(f string, args ...any) {}

func (l *emptyLogger) LogBuf(buf []byte, consume int, f string, args ...any) {}

type fmtLogger struct {
	pref        string
	logFullBuff bool
}

func (l *fmtLogger) Log(f string, args ...any) {
	if l == nil {
		return
	}

	l.printF(f, args...)
}

func (l *fmtLogger) LogBuf(buf []byte, consume int, f string, args ...any) {
	if l == nil {
		return
	}

	if consume < 0 {
		consume = len(buf)
	}

	res := fmt.Sprintf(f, args...)

	if l.logFullBuff {
		bufStr := string(buf)
		bufStr = strings.ReplaceAll(bufStr, "\n", "\\n")
		resF := "%s; Consume %d; buf len: %d; buf: %q"
		l.printF(resF, res, consume, len(buf), buf[:consume])
		return
	}

	l.printF("%s; Consume %d; buf len: %d", res, consume, len(buf))
}

func (l *fmtLogger) printF(f string, args ...any) {
	f = l.pref + f + "\n"
	_, _ = fmt.Fprintf(os.Stderr, f, args...)
}

func newFmtLogger(fullBuf bool, targets ...string) *fmtLogger {
	return &fmtLogger{
		pref:        createPrefixForLogger(targets...),
		logFullBuff: fullBuf,
	}
}

func createPrefixForLogger(targets ...string) string {
	pref := "[github.com/name212/gotee][DEBUG]"

	if len(targets) > 0 {
		resTargets := make([]string, 0, len(targets))
		for _, t := range targets {
			if t == "" {
				continue
			}
			resTargets = append(resTargets, fmt.Sprintf("[%s]", t))
		}
		pref = pref + strings.Join(resTargets, "")
	}

	return pref + ": "
}
