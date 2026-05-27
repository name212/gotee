// Copyright 2026
// license that can be found in the LICENSE file.

package tee

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

// ConsumerName
// Returns joined strings of name
// or unique name for Consumer
// If name is not passed use runtime.Caller
// for extract filename and line and add rand number prefix
func ConsumerName(deep int, name ...string) string {
	if len(name) > 0 {
		return strings.Join(name, " ")
	}

	if deep < 0 {
		deep = 1
	}

	deep++

	randPrefix := rand.NewSource(time.Now().UnixNano()).Int63()

	_, f, line, ok := runtime.Caller(deep)
	if !ok {
		return fmt.Sprintf("%d: unknown", randPrefix)
	}

	return fmt.Sprintf("%d: %s:%d", randPrefix, f, line)
}

// CopyBytes
// copy input to new byte slice
func CopyBytes(input []byte) []byte {
	res := make([]byte, len(input))
	copy(res, input)
	return res
}
