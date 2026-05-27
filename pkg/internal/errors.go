// Copyright 2026
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
)

func AppendErr(resErr error, toAppend ...error) error {
	toAppendLen := len(toAppend)
	if toAppendLen == 0 {
		return resErr
	}

	i := 0
	result := resErr
	if IsNil(result) {
		result = toAppend[0]
		i++
	}

	for ; i < toAppendLen; i++ {
		a := toAppend[i]

		if !IsNil(a) {
			result = errors.Join(result, a)
		}
	}

	if IsNil(result) {
		return nil
	}

	return result
}

func ConcatErrs(first, second error) error {
	return errors.Join(first, second)
}
