// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/dns"
)

// NodeResolver resolves a given cluster and a node name to an IP address.
type NodeResolver interface {
	Resolve(cluster, node string) dns.Info
}

type resolvedNodeInfo struct {
	node  dns.Info
	nodes []dns.Info
}

func resolveNodes(dnsService NodeResolver, md metadata.MD) resolvedNodeInfo {
	nodesVal := md.Get(message.NodesHeaderKey)

	cluster := getClusterName(md)

	nodes := make([]string, 0, len(nodesVal)*2)
	for _, node := range nodesVal {
		nodes = append(nodes, strings.Split(node, ",")...)
	}

	node := ""
	if nodeVal := md.Get("node"); len(nodeVal) > 0 {
		node = nodeVal[0]
	}

	if cluster == "" {
		return resolvedNodeInfo{
			nodes: xslices.Map(nodes, func(n string) dns.Info {
				return dns.Info{Address: n}
			}),
			node: dns.Info{Address: node},
		}
	}

	resolveNode := func(val string) dns.Info {
		if val == "" {
			return dns.Info{}
		}

		return dnsService.Resolve(cluster, val)
	}

	resolvedNode := resolveNode(node)

	resolvedNodes := make([]dns.Info, 0, len(nodes))
	for _, n := range nodesVal {
		resolvedNodes = append(resolvedNodes, resolveNode(n))
	}

	return resolvedNodeInfo{
		nodes: resolvedNodes,
		node:  resolvedNode,
	}
}
