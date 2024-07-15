// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ctxstore_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/siderolabs/gen/pair"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestWithValue(t *testing.T) {
	ctx := ctxstore.WithValue(context.Background(), "value1")
	ctx = ctxstore.WithValue(ctx, 42)
	ctx = ctxstore.WithValue(ctx, true)

	type (
		customString string
		stringAlias  = string
	)

	var cs customString

	assert.Equal(t, pair.MakePair("value1", true), pair.MakePair(ctxstore.Value[string](ctx)))
	assert.Equal(t, pair.MakePair(42, true), pair.MakePair(ctxstore.Value[int](ctx)))
	assert.Equal(t, pair.MakePair(true, true), pair.MakePair(ctxstore.Value[bool](ctx)))
	assert.Equal(t, pair.MakePair(0.0, false), pair.MakePair(ctxstore.Value[float64](ctx)))
	assert.Equal(t, pair.MakePair(cs, false), pair.MakePair(ctxstore.Value[customString](ctx)))
	assert.Equal(t, pair.MakePair("value1", true), pair.MakePair(ctxstore.Value[stringAlias](ctx)))
}

func BenchmarkWithValue(b *testing.B) {
	b.ReportAllocs()

	type (
		emtpyStruct     struct{}
		myStruct[T any] struct{ Val T }
	)

	b.Run("empty struct", func(b *testing.B) { benchmarkFor(b, emtpyStruct{}) })
	b.Run("small int", func(b *testing.B) { benchmarkFor(b, 42) })
	b.Run("small int inside struct", func(b *testing.B) { benchmarkFor(b, myStruct[int]{Val: 42}) })
	b.Run("normal int", func(b *testing.B) { benchmarkFor(b, 424242) })
	b.Run("normal int inside struct", func(b *testing.B) { benchmarkFor(b, myStruct[int]{Val: 424242}) })
	b.Run("bool", func(b *testing.B) { benchmarkFor(b, true) })
	b.Run("bool inside struct", func(b *testing.B) { benchmarkFor(b, myStruct[bool]{Val: true}) })
	b.Run("string", func(b *testing.B) { benchmarkFor(b, "value") })
	b.Run("string inside struct", func(b *testing.B) { benchmarkFor(b, myStruct[string]{Val: "value"}) })
}

func benchmarkFor[T any](b *testing.B, value T) {
	b.ReportAllocs()

	var (
		ok     bool
		result T
	)

	for range b.N {
		ctx := ctxstore.WithValue(context.Background(), value)

		result, ok = ctxstore.Value[T](ctx)
		if !ok {
			b.Fatal("unexpected")
		}
	}

	runtime.KeepAlive(result)
}
