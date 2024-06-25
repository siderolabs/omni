// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"context"

	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/dns"
)

// ResolvedNodesHeaderKey is used to propagate the node IP information from the node/nodes headers to the backend.
const ResolvedNodesHeaderKey = "resolved-nodes"

// OmniBackend implements a backend (proxying one2one to a Talos node).
type OmniBackend struct {
	conn         *grpc.ClientConn
	nodeResolver NodeResolver
	name         string
}

// NewOmniBackend builds new backend.
func NewOmniBackend(name string, nodeResolver NodeResolver, conn *grpc.ClientConn) *OmniBackend {
	backend := &OmniBackend{
		name:         name,
		nodeResolver: nodeResolver,
		conn:         conn,
	}

	return backend
}

func (l *OmniBackend) String() string {
	return l.name
}

// GetConnection returns a grpc connection to the backend.
func (l *OmniBackend) GetConnection(ctx context.Context, _ string) (context.Context, *grpc.ClientConn, error) {
	md, _ := metadata.FromIncomingContext(ctx)

	// Set resolved nodes as a header to be used by the ResourceServer.
	// Use a new header to avoid signature mismatch.
	resolved := resolveNodes(l.nodeResolver, md)

	var allNodes []dns.Info

	if resolved.nodeOk {
		allNodes = append(allNodes, resolved.node)
	}

	allNodes = append(allNodes, resolved.nodes...)

	if len(allNodes) > 0 {
		md.Set(ResolvedNodesHeaderKey, xslices.Map(allNodes, func(info dns.Info) string {
			return info.GetAddress()
		})...)
	}

	outCtx := metadata.NewOutgoingContext(ctx, md)

	return outCtx, l.conn, nil
}

// AppendInfo is called to enhance response from the backend with additional data.
func (l *OmniBackend) AppendInfo(_ bool, resp []byte) ([]byte, error) {
	return resp, nil
}

// BuildError is called to convert error from upstream into response field.
func (l *OmniBackend) BuildError(bool, error) ([]byte, error) {
	return nil, nil
}
