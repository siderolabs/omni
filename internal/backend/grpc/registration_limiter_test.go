// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/grpc"
)

func TestRegistrationLimiter(t *testing.T) {
	var limiter grpc.RegistrationLimiter

	now := time.Now()

	for range grpc.RegistrationBurst {
		require.True(t, limiter.Allow("10.0.0.1", now))
	}

	assert.False(t, limiter.Allow("10.0.0.1", now), "the burst is used up")
	assert.True(t, limiter.Allow("10.0.0.2", now), "another source has its own burst")
	assert.Equal(t, 2, limiter.Len())

	// the sweep drops the full bucket of 10.0.0.2 and keeps the used one of 10.0.0.1, which has 20 tokens by then
	afterSweep := now.Add(20 * grpc.RegistrationInterval)

	require.True(t, limiter.Allow("10.0.0.3", afterSweep))
	assert.Equal(t, 2, limiter.Len())

	for range 20 {
		require.True(t, limiter.Allow("10.0.0.1", afterSweep))
	}

	assert.False(t, limiter.Allow("10.0.0.1", afterSweep))
	assert.True(t, limiter.Allow("10.0.0.1", afterSweep.Add(grpc.RegistrationInterval)), "one more after the interval")
}

func TestSourceAddress(t *testing.T) {
	withForwardedFor := func(values ...string) context.Context {
		md := metadata.New(nil)

		for _, value := range values {
			md.Append("x-forwarded-for", value)
		}

		return metadata.NewIncomingContext(t.Context(), md)
	}

	assert.Equal(t, "203.0.113.9", grpc.SourceAddress(withForwardedFor("203.0.113.9")))
	assert.Equal(t, "203.0.113.9", grpc.SourceAddress(withForwardedFor("203.0.113.9, 192.168.3.120")), "the gateway appends the connection after the client")
	assert.Equal(t, "203.0.113.9", grpc.SourceAddress(withForwardedFor("203.0.113.9", "5.6.7.8")), "the first value, then its first entry")
	assert.Equal(t, "2001:db8:1:2::/64", grpc.SourceAddress(withForwardedFor("2001:db8:1:2::1")), "an IPv6 source is its /64")
	assert.Equal(t, "2001:db8:1:2::/64", grpc.SourceAddress(withForwardedFor("2001:db8:1:2:ffff::1")))
	assert.Equal(t, "203.0.113.9", grpc.SourceAddress(withForwardedFor("::ffff:203.0.113.9")), "a mapped IPv4 address is keyed as IPv4")
	assert.Equal(t, "", grpc.SourceAddress(t.Context()))
}
