// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package errgroup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/errgroup"
)

func TestGroup(t *testing.T) {
	t.Run("should not return an error if it was canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		eg, innerCtx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			// We can't wait on parent `Done` here because `context` package closes `Done` channel before
			// it cancels the children contexts.
			<-innerCtx.Done()

			// Or we could wait for parent `Err` method to unblock to achieve the same of effect.
			// _ = ctx.Err()

			return nil
		})
		cancel()

		err := eg.Wait()

		require.NoError(t, err)
	})

	t.Run("should return an error if it was canceled", func(t *testing.T) {
		eg, _ := errgroup.WithContext(t.Context())
		eg.Go(func() error { return nil })
		err := eg.Wait()
		require.Contains(t, err.Error(), "sentinel error: function returned with nil error")
		require.Contains(t, err.Error(), "errgroup.(*Group).Go")
		require.Contains(t, err.Error(), "internal/pkg/errgroup/errgroup.go:50", "unexpected stack trace - did the line numbers change in the source file? error:\n%s", err.Error())
	})

	t.Run("should return an error if it got an error", func(t *testing.T) {
		eg, _ := errgroup.WithContext(t.Context())
		eg.Go(func() error { return errors.New("error") })
		err := eg.Wait()
		require.Contains(t, err.Error(), "error")
	})

	t.Run("should not hit ReturnError path on second Go call", func(t *testing.T) {
		eg, _ := errgroup.WithContext(t.Context())
		eg.Go(func() error { return errors.New("error") })
		err := eg.Wait()
		eg.Go(func() error { return nil })
		require.Contains(t, err.Error(), "error")
	})
}
