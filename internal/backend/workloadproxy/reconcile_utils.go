// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"fmt"
	"iter"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/pair"
)

// aliasToCluster is a data structure that maps aliases to clusters and ports. It doesn't keep tarck of active probes,
// but it keeps track of the in-use port for the said probe for each cluster.
type aliasToCluster struct {
	aliases  map[alias]aliasData
	clusters map[resource.ID]*clusterData
}

type aliasData struct {
	clusterData *clusterData
	port        port
}

type clusterData struct {
	clusterID resource.ID
	inUsePort port
	hosts     []string
	aliases   []alias
}

// ReplaceCluster replaces the cluster data in the aliasToCluster. If the ReconcileData is nil or doesn't contain any
// aliases, the cluster will be removed from the structure.
func (a *aliasToCluster) ReplaceCluster(clusterID resource.ID, rd *ReconcileData) error {
	if rd == nil || len(rd.AliasPort) == 0 {
		got, ok := a.clusters[clusterID]
		if !ok {
			return nil
		}

		delete(a.clusters, clusterID)

		for _, als := range got.aliases {
			delete(a.aliases, als)
		}

		return nil
	}

	got, ok := a.clusters[clusterID]
	if !ok {
		for als := range rd.AliasesData() {
			if res := a.aliases[alias(als)]; res.clusterData != nil {
				return fmt.Errorf("alias %q already exists and used by cluster %q", als, res.clusterData.clusterID)
			}
		}

		got = &clusterData{
			clusterID: clusterID,
			hosts:     rd.Hosts,
			aliases:   toSortedSlice(rd),
		}

		a.clusters[clusterID] = got
	} else {
		if !slices.Equal(got.aliases, toSortedSlice(rd)) {
			for _, als := range got.aliases {
				delete(a.aliases, als)
			}

			got.inUsePort = ""
		}

		got.hosts = rd.Hosts
	}

	for als, p := range rd.AliasPort {
		a.aliases[alias(als)] = aliasData{clusterData: got, port: port(p)}
	}

	return nil
}

func toSortedSlice(rd *ReconcileData) []alias {
	slc := xmaps.ToSlice(rd.AliasPort, func(k, _ string) alias { return alias(k) })

	slices.Sort(slc)

	return slc
}

// ClusterPort returns the cluster ID and port for the given alias. If the alias doesn't exist, the function returns false.
func (a *aliasToCluster) ClusterPort(als alias) (resource.ID, port, bool) {
	if got, ok := a.aliases[als]; ok && got.clusterData != nil {
		return got.clusterData.clusterID, got.port, true
	}

	return "", "", false
}

// ClusterData returns the cluster data for the given cluster ID. If the cluster doesn't exist, the function returns nil.
func (a *aliasToCluster) ClusterData(clusterID resource.ID) *clusterData {
	if val, ok := a.clusters[clusterID]; ok {
		return val
	}

	return nil
}

// SetActivePort sets the in-use port for the given cluster ID. If the cluster doesn't exist, the function returns an error.
func (a *aliasToCluster) SetActivePort(clusterID resource.ID, p port) error {
	if val, ok := a.clusters[clusterID]; ok {
		val.inUsePort = p

		return nil
	}

	return fmt.Errorf("cluster %q not found", clusterID)
}

// ActiveHostsPort returns the active hosts and port for the given cluster ID. If the cluster doesn't exist, the function
// finds the first cluster with the given ID and returns its hosts and in-use port, also setting the in-use port for the
// cluster.
func (a *aliasToCluster) ActiveHostsPort(clusterID resource.ID) ([]string, port) {
	existingCluster, ok := a.clusters[clusterID]
	if !ok {
		return nil, ""
	}

	for _, als := range existingCluster.aliases {
		if alsData, found := a.aliases[als]; found {
			existingCluster.inUsePort = alsData.port

			return existingCluster.hosts, alsData.port
		}
	}

	return nil, ""
}

func (a *aliasToCluster) DropAlias(als alias) *clusterData {
	removed, ok := a.aliases[als]
	if !ok {
		return nil
	}

	delete(a.aliases, als)

	clusterPtr := removed.clusterData

	if clusterPtr.inUsePort == removed.port {
		clusterPtr.inUsePort = ""
	}

	return clusterPtr
}

// All returns all the alias data. The function returns a triple: alias, it's
// port and the cluster data.
func (a *aliasToCluster) All() iter.Seq2[pair.Pair[alias, port], *clusterData] {
	return func(yield func(pair.Pair[alias, port], *clusterData) bool) {
		for als, v := range a.aliases {
			ptr := v.clusterData
			if ptr == nil {
				continue
			}

			if !yield(pair.MakePair(als, v.port), ptr) {
				return
			}
		}
	}
}
