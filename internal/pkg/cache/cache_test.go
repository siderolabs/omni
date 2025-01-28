// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cache_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/cache"
)

func TestResultCacher_GetOrUpdate(t *testing.T) {
	cacher := cache.Value[int]{Duration: time.Second}

	result, err := cacher.GetOrUpdate(func() (int, error) { return 42, errors.New("test") })

	require.EqualError(t, err, "test")
	require.Zero(t, result)

	result, err = cacher.GetOrUpdate(func() (int, error) { return 42, nil })

	require.NoError(t, err)
	require.Equal(t, 42, result)

	result, err = cacher.GetOrUpdate(func() (int, error) { return 43, nil })

	require.NoError(t, err)
	require.Equal(t, 42, result)

	time.Sleep(time.Second)

	result, err = cacher.GetOrUpdate(func() (int, error) { return 43, nil })

	require.NoError(t, err)
	require.Equal(t, 43, result)
}
