// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime_test

import (
	"strings"
	"testing"

	"github.com/siderolabs/gen/pair"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime"
)

func TestSliceSet(t *testing.T) {
	set := runtime.NewSliceSet(strings.Compare)

	set.Add("")
	set.Add("")
	set.Add("b")
	set.Add("a")
	require.Equal(t, []string{"", "a", "b"}, toSlice(set))
	require.EqualValues(t, 3, set.Len())

	set.Add("a")
	require.Equal(t, []string{"", "a", "b"}, toSlice(set))

	set.Add("c")
	require.Equal(t, []string{"", "a", "b", "c"}, toSlice(set))

	set.Add("1")
	require.Equal(t, []string{"", "1", "a", "b", "c"}, toSlice(set))

	require.True(t, set.Contains("a"))
	require.False(t, set.Contains("d"))

	require.Equal(t, pair.MakePair("", true), pair.MakePair(set.Min()))
	require.Equal(t, pair.MakePair("c", true), pair.MakePair(set.Max()))

	set.Reset()
	require.Zero(t, toSlice(set))
	require.False(t, set.Contains("a"))

	require.Equal(t, pair.MakePair("", false), pair.MakePair(set.Min()))
	require.Equal(t, pair.MakePair("", false), pair.MakePair(set.Max()))
}

func toSlice(set runtime.SliceSet[string]) []string {
	var slice []string

	set.ForEach(func(item string) {
		slice = append(slice, item)
	})

	return slice
}

func TestStreamOffsetLimiter(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
		offset   int
		limit    int
	}{
		{
			name:     "offset 0, limit 0",
			offset:   0,
			limit:    0,
			args:     []string{"a", "c", "d", "e", "f", "g"},
			expected: []string{"a", "c", "d", "e", "f", "g"},
		},
		{
			name:     "offset 0, limit 2",
			offset:   0,
			limit:    2,
			args:     []string{"a", "c", "d", "e", "f", "g"},
			expected: []string{"a", "c"},
		},
		{
			name:     "offset 3, limit 0",
			offset:   3,
			limit:    0,
			args:     []string{"a", "b", "b", "a1", "c", "b", "d", "e", "f", "b", "a1"},
			expected: []string{"c", "d", "e", "f"},
		},
		{
			name:     "offset 1, limit 1",
			offset:   1,
			limit:    1,
			args:     []string{"a", "c", "d", "e", "c", "f", "g"},
			expected: []string{"c", "c"},
		},
		{
			name:     "offset 3, limit 3, duplicated",
			offset:   3,
			limit:    3,
			args:     []string{"a", "a", "b", "b", "a", "c", "c", "d", "d", "e", "d", "e", "d", "f", "f", "g", "g", "f", "d"},
			expected: []string{"d", "d", "e", "d", "e", "d", "f", "f", "f", "d"},
		},
		{
			name:     "offset 3, limit 2",
			offset:   3,
			limit:    3,
			args:     []string{"a", "c", "d", "e", "e", "f", "e", "g", "h", "e", "i", "e1", "e2", "e"},
			expected: []string{"e", "e", "f", "e", "g", "e", "e1", "e2", "e"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			offsetLimiter := runtime.MakeStreamOffsetLimiter(test.offset, test.limit, strings.Compare)

			var accum []string

			for _, arg := range test.args {
				if offsetLimiter.Check(arg) {
					accum = append(accum, arg)
				}
			}

			require.Equal(t, test.expected, accum)
		})
	}
}
