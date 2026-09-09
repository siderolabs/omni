// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omnictl/security"
)

func TestFetchConcurrentlyPreservesOrder(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	results, err := security.FetchConcurrently(t.Context(), items, func(_ context.Context, i int) (int, error) {
		// Items that sort later finish first, so a naive "append on completion" implementation
		// would produce a different order than the input.
		time.Sleep(time.Duration(len(items)-i) * time.Millisecond)

		return i * 10, nil
	})
	require.NoError(t, err)

	require.Equal(t, []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90}, results)
}

func TestFetchConcurrentlyPropagatesError(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	errBoom := errors.New("boom")

	_, err := security.FetchConcurrently(t.Context(), items, func(_ context.Context, i int) (int, error) {
		if i == 2 {
			return 0, errBoom
		}

		return i, nil
	})
	require.ErrorIs(t, err, errBoom)
}

func TestFetchConcurrentlyRespectsLimit(t *testing.T) {
	items := make([]int, security.FetchConcurrency*3)

	var inFlight, maxInFlight atomic.Int64

	_, err := security.FetchConcurrently(t.Context(), items, func(_ context.Context, _ int) (struct{}, error) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)

		for {
			observedMax := maxInFlight.Load()
			if n <= observedMax || maxInFlight.CompareAndSwap(observedMax, n) {
				break
			}
		}

		time.Sleep(time.Millisecond)

		return struct{}{}, nil
	})
	require.NoError(t, err)

	require.LessOrEqual(t, maxInFlight.Load(), int64(security.FetchConcurrency))
}

func TestFetchConcurrentlyEmpty(t *testing.T) {
	results, err := security.FetchConcurrently(t.Context(), []int{}, func(_ context.Context, i int) (int, error) {
		t.Fatal("fetch must not be called for an empty input")

		return i, nil
	})
	require.NoError(t, err)
	require.Empty(t, results)
}

// notFound is the error the server reports for an artifact the factory has not produced, wrapped
// the way every fetch callback wraps it - the wrapping is what carries the schematic/version/arch
// into the message, and it must not hide the code.
func notFound() error {
	return fmt.Errorf("failed to fetch SBOM for abc/1.9.0/amd64: %w", status.Error(codes.NotFound, "SBOM not found"))
}

// TestSkipMissingToleratesNotFound covers the one error a cluster-wide fetch has to survive: an
// artifact the factory has not produced is an answer rather than a fault, so the artifacts that did
// come back must still be rendered. The skipped item is left as the zero value for the caller to
// drop.
func TestSkipMissingToleratesNotFound(t *testing.T) {
	fetch := security.SkipMissing(func(_ context.Context, i int) (int, error) {
		if i == 2 {
			return 0, notFound()
		}

		return i + 1, nil
	})

	results, err := security.FetchConcurrently(t.Context(), []int{0, 1, 2, 3, 4}, fetch)
	require.NoError(t, err)

	require.Equal(t, []int{1, 2, 0, 4, 5}, results)
}

// TestSkipMissingPropagatesOtherCodes guards the boundary of that tolerance: a factory that is
// unreachable or rejecting Omni's credentials says nothing about whether the artifact exists, so it
// must not be rendered as absent.
func TestSkipMissingPropagatesOtherCodes(t *testing.T) {
	fetch := security.SkipMissing(func(_ context.Context, i int) (int, error) {
		if i == 2 {
			return 0, status.Error(codes.Unavailable, "image factory rejected Omni's credentials")
		}

		return i, nil
	})

	_, err := security.FetchConcurrently(t.Context(), []int{0, 1, 2, 3, 4}, fetch)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

// TestUnwrappedFetchPropagatesNotFound covers the other half of the rule: a command handed one
// exact artifact does not wrap its fetch, so NotFound stays an error and keeps the message saying
// what was missing.
func TestUnwrappedFetchPropagatesNotFound(t *testing.T) {
	_, err := security.FetchConcurrently(t.Context(), []int{0}, func(_ context.Context, _ int) (int, error) {
		return 0, notFound()
	})
	require.Equal(t, codes.NotFound, status.Code(err))
	require.ErrorContains(t, err, "failed to fetch SBOM for abc/1.9.0/amd64")
}

func TestRequireArtifacts(t *testing.T) {
	missing := func(i int) bool { return i == 0 }

	present, err := security.RequireArtifacts([]int{1, 0, 3}, missing)
	require.NoError(t, err)
	require.Equal(t, []int{1, 3}, present)

	_, err = security.RequireArtifacts([]int{0, 0}, missing)
	require.ErrorIs(t, err, security.ErrNoArtifacts)
}
