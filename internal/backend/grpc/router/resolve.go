// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/internal/backend/dns"
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
