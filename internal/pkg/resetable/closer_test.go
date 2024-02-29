// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package resetable_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/resetable"
)

func testResetableCloser(reset bool) (closerCalled bool) {
	closer := func() error {
		closerCalled = true

		return nil
	}

	rc := resetable.NewCloserErr(closer)
	defer rc.Close()

	if reset {
		rc.Reset()
	}

	return false
}

func TestResetableCloser(t *testing.T) {
	require.False(t, testResetableCloser(true))
	require.True(t, testResetableCloser(false))
}
