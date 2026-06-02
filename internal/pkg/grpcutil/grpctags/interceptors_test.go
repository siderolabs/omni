// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpctags_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpctags"
)

func TestUnaryServerInterceptorStoresPeerHost(t *testing.T) {
	addr := &net.TCPAddr{
		IP:   net.ParseIP("10.10.10.10"),
		Port: 12345,
	}

	interceptor := grpctags.UnaryServerInterceptor()
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		require.Equal(t, "10.10.10.10", grpctags.Extract(ctx).Values()["peer.address"])

		return nil, nil //nolint:nilnil
	})
	require.NoError(t, err)
}
