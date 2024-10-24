// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"errors"
	"fmt"
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

func (r resolvedNodeInfo) getNode() (dns.Info, error) {
	if r.nodeOk {
		return r.node, nil
	}

	if len(r.nodes) > 0 {
		var clusterName string

		for _, n := range r.nodes {
			if n.Ambiguous {
				return n, nil
			}

			if clusterName != "" && clusterName != n.Cluster {
				return dns.Info{}, fmt.Errorf("all nodes should be in the same cluster, found clusters %q and %q", clusterName, n.Cluster)
			}

			clusterName = n.Cluster
		}

		return r.nodes[0], nil
	}

	return dns.Info{}, errors.New("node not found")
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

		if val != "" {
			resolved = dnsService.Resolve(cluster, val)
		}

		if resolved.GetAddress() == "" && !resolved.Ambiguous {
			return dns.NewInfo(
				cluster,
				val,
				val,
				val,
			)
		}

		return resolved
	}

	return resolvedNodeInfo{
		node:   resolveNode(node),
		nodes:  xslices.Map(nodes, resolveNode),
		nodeOk: nodeOK,
	}
}
