// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpcutil provides utilities for gRPC.
package grpcutil

import (
	"context"
	"errors"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/siderolabs/omni/internal/pkg/errgroup"
)

// RunServer starts gRPC server on top of the provided listener, stops it when the context is done.
func RunServer(ctx context.Context, server *grpc.Server, lis net.Listener, eg *errgroup.Group) {
	eg.Go(func() error {
		err := server.Serve(lis)
		if !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}

		return nil
	})

	eg.Go(func() error {
		serverGracefulStop(server, ctx)

		return nil
	})
}

func serverGracefulStop(server *grpc.Server, ctx context.Context) { //nolint:revive
	<-ctx.Done()

	stopped := make(chan struct{})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-shutdownCtx.Done():
		server.Stop()
	case <-stopped:
		server.Stop()
	}
}
