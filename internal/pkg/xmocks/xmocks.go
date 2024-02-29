// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package xmocks provides some helpers for testify mocks.
package xmocks

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/stretchr/testify/mock"
)

// Name returns the name of the given method.
func Name(method any) string {
	val := reflect.ValueOf(method)

	if val.Kind() != reflect.Func {
		panic("not a function")
	}

	name := runtime.FuncForPC(val.Pointer()).Name()

	name = name[strings.LastIndex(name, ".")+1:]

	return strings.TrimSuffix(name, "-fm")
}

// GetAs casts the given [mock.Arguments.Get] to the given type.
func GetAs[T any](m mock.Arguments, i int) T {
	res := m.Get(i)
	if res != nil {
		return res.(T) //nolint:forcetypeassert
	}

	switch typ := reflect.TypeOf((*T)(nil)).Elem(); typ.Kind() { //nolint:exhaustive
	case reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Ptr, reflect.Interface, reflect.UnsafePointer:
		var z T

		return z
	default:
		panic("non nil value expected for type " + typ.String())
	}
}

// Cast2 casts the given [mock.Arguments.Get] to the given types. Helper function for multiple return values.
func Cast2[T, T1 any](m mock.Arguments) (T, T1) {
	return GetAs[T](m, 0), GetAs[T1](m, 1)
}

// Cast3 casts the given [mock.Arguments.Get] to the given types. Helper function for multiple return values.
func Cast3[T, T1, T2 any](m mock.Arguments) (T, T1, T2) {
	return GetAs[T](m, 0), GetAs[T1](m, 1), GetAs[T2](m, 2)
}
