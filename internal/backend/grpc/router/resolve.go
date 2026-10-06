// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const (
	nodeHeaderKey  = "node"
	nodesHeaderKey = "nodes"
)

// NodeResolver resolves a given cluster and a node name to a dns.Info.
type NodeResolver interface {
	Resolve(cluster, node string) (dns.Info, error)
}

func resolveNodes(nodeResolver NodeResolver, md metadata.MD) ([]dns.Info, error) {
	var rawNodes []string

	if nodeVal := md.Get(nodeHeaderKey); len(nodeVal) > 0 {
		rawNodes = append(rawNodes, nodeVal[0])
	}

	if nodesVal := md.Get(nodesHeaderKey); len(nodesVal) > 0 {
		for _, n := range nodesVal {
			rawNodes = append(rawNodes, strings.Split(n, ",")...)
		}
	}

	cluster := getClusterName(md)

	nodes := make([]dns.Info, 0, len(rawNodes))

	for _, val := range rawNodes {
		if val == "" {
			return nil, fmt.Errorf("empty node value")
		}

		info, err := nodeResolver.Resolve(cluster, val)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, info)
	}

	// All resolved nodes must belong to the same cluster.
	// This is not technically required anymore, but targeting nodes of different clusters at once is most possibly unintentional.
	// Additionally, ensuring that all of them belong to the same cluster allows us to run an ACL check against a single cluster.
	if len(nodes) > 1 {
		clusterName := nodes[0].Cluster

		for _, n := range nodes[1:] {
			if n.Cluster != clusterName {
				return nil, fmt.Errorf("all nodes should be in the same cluster, found clusters %q and %q", clusterName, n.Cluster)
			}
		}
	}

	return nodes, nil
}

// authenticate verifies the signature of a proxied request, as the regular gRPC interceptors do not run for it.
func authenticate(ctx context.Context, verifier grpc.UnaryServerInterceptor, md metadata.MD, fullMethodName string) (context.Context, error) {
	ctx = ctxstore.WithValue(ctx, auth.GRPCMessageContextKey{Message: message.NewGRPC(md, fullMethodName)})

	_, err := verifier(ctx, nil, nil, func(innerCtx context.Context, _ any) (any, error) {
		ctx = innerCtx //nolint:fatcontext

		return nil, nil //nolint:nilnil
	})

	return ctx, err
}

// reportResolveError verifies the caller of a request whose nodes or cluster could not be resolved, and returns the
// error that caller receives.
func reportResolveError(ctx context.Context, verifier grpc.UnaryServerInterceptor, fullMethodName string, resolveErr error) error {
	md, _ := metadata.FromIncomingContext(ctx)

	ctx, err := authenticate(ctx, verifier, md, fullMethodName)
	if err != nil {
		return err
	}

	return resolveErrorFor(ctx, resolveErr)
}

// resolveErrorFor returns the resolution error to callers that can read every machine, and the regular access error to
// everyone else.
func resolveErrorFor(ctx context.Context, resolveErr error) error {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return err
	}

	return resolveErr
}

// ResolveMachines returns the IDs of the machines the node headers point to.
func ResolveMachines(ctx context.Context, nodeResolver NodeResolver, md metadata.MD) ([]string, error) {
	nodes, err := resolveNodes(nodeResolver, md)
	if err != nil {
		return nil, resolveErrorFor(ctx, err)
	}

	return xslices.Map(nodes, func(info dns.Info) string { return info.ID }), nil
}
