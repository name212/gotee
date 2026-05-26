// Copyright 2026
// license that can be found in the LICENSE file.

package internal

import "reflect"

// IsNil
// copy from here
// https://github.com/name212/govalue/blob/c6f1c3ba706eed264adfcac58cfba34fbfd9fe12/is_nil.go#L13-L24
// for no deps
// need for checking user defined errors
func IsNil(value any) bool {
	iv := reflect.ValueOf(value)
	if !iv.IsValid() {
		return true
	}
	switch iv.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return iv.IsNil()
	default:
		return false
	}
}