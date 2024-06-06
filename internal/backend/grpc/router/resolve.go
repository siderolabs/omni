// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"strings"

	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/dns"
)

const (
	nodeHeaderKey  = "node"
	nodesHeaderKey = "nodes"
)

// NodeResolver resolves a given cluster and a node name to an IP address.
type NodeResolver interface {
	Resolve(cluster, node string) dns.Info
}

type resolvedNodeInfo struct {
	node  dns.Info
	nodes []dns.Info

	nodeOk bool
}

func resolveNodes(dnsService NodeResolver, md metadata.MD) resolvedNodeInfo {
	var (
		node  string
		nodes []string

		nodeOK bool
	)

	if nodeVal := md.Get(nodeHeaderKey); len(nodeVal) > 0 {
		nodeOK = true

		node = nodeVal[0]
	}

	if nodesVal := md.Get(nodesHeaderKey); len(nodesVal) > 0 {
		nodes = make([]string, 0, len(nodesVal)*2)
		for _, n := range nodesVal {
			nodes = append(nodes, strings.Split(n, ",")...)
		}
	}

	cluster := getClusterName(md)

	resolveNode := func(val string) dns.Info {
		var resolved dns.Info

		if cluster != "" && val != "" {
			resolved = dnsService.Resolve(cluster, val)
		}

		if resolved.Address == "" {
			return dns.Info{
				Cluster: cluster,
				Name:    val,
				Address: val,
			}
		}

		return resolved
	}

	return resolvedNodeInfo{
		node:   resolveNode(node),
		nodes:  xslices.Map(nodes, resolveNode),
		nodeOk: nodeOK,
	}
}
