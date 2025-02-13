// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"maps"
	"testing"

	"github.com/siderolabs/gen/xiter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func Test_ActionMap(t *testing.T) {
	var am workloadproxy.ActionMap

	for range am.All() {
		t.Fatal("unexpected range")
	}

	replaced, ok := am.Replace("key1", "value1")
	require.False(t, ok)
	require.Zero(t, replaced)

	replaced, ok = am.Replace("key2", "value2")
	require.False(t, ok)
	require.Zero(t, replaced)

	replaced, ok = am.Replace("key1", "value3")
	require.True(t, ok)
	require.Equal(t, "value1", replaced)

	require.Equal(t, map[string]string{
		"key1": "value3",
		"key2": "value2",
	}, maps.Collect(am.All()))

	for _, v := range xiter.Filter2(
		func(k, _ string) bool { return k == "key1" },
		am.All(),
	) {
		require.Equal(t, "value3", v)

		break
	}

	require.Equal(t, am.Get("key1"), "value3")
	require.Equal(t, am.Get("key2"), "value2")
	require.Zero(t, am.Get("key3"))

	am.Delete("key2")
}

func Test_UniqueSlice(t *testing.T) {
	var us workloadproxy.UniqueSlice

	got, ok := us.FindBy(func(e *workloadproxy.MyStruct) bool { return e.Val == "42" })
	require.False(t, ok)
	require.Nil(t, got)

	old, replaced := us.Replace(&workloadproxy.MyStruct{Val: "41", Data: "hello"})
	require.False(t, replaced)
	require.Nil(t, old)

	old, replaced = us.Replace(&workloadproxy.MyStruct{Val: "41", Data: "world"})
	require.True(t, replaced)
	require.Equal(t, "hello", old.Data)

	result, ok := us.FindBy(func(e *workloadproxy.MyStruct) bool { return e.Data == "world" && e.Val == "41" })
	require.True(t, ok)
	require.Equal(t, "world", result.Data)
	require.Equal(t, "41", result.Val)

	require.True(t, us.Remove(result))
	require.False(t, us.Remove(result))

	toInsert := &workloadproxy.MyStruct{Val: "42", Data: "something"}
	old, replaced = us.Replace(toInsert)
	require.False(t, replaced)
	require.Nil(t, old)

	result, ok = us.FindBy(func(e *workloadproxy.MyStruct) bool { return e.Data == "something" })
	require.True(t, ok)
	require.Equal(t, "something", result.Data)
	require.Equal(t, "42", result.Val)

	require.True(t, us.Remove(toInsert))

	result, ok = us.FindBy(func(e *workloadproxy.MyStruct) bool { return e.Data == "something" })
	require.False(t, ok)
	require.Nil(t, result)
}
