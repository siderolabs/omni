// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil

import (
	"context"
	"errors"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
)

// RunServer starts gRPC server on top of the provided listener, stops it when the context is done.
func RunServer(ctx context.Context, server *grpc.Server, lis net.Listener, eg *errgroup.Group, logger *zap.Logger) {
	eg.Go(func() error {
		err := server.Serve(lis)
		if !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}

		return nil
	})

	eg.Go(func() error {
		serverGracefulStop(server, ctx, logger)

		return nil
	})
}

func serverGracefulStop(server *grpc.Server, ctx context.Context, logger *zap.Logger) { //nolint:revive
	<-ctx.Done()

	stopped := make(chan struct{})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	panichandler.Go(func() {
		server.GracefulStop()
		close(stopped)
	}, logger)

	select {
	case <-shutdownCtx.Done():
		server.Stop()
	case <-stopped:
		server.Stop()
	}
}
