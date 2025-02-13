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
	aliasToPort actionMap[alias, pair.Pair[*clusterData, port]]
	slc         uniqueSlice[*clusterData]
}

// ReplaceCluster replaces the cluster data in the aliasToCluster. If the ReconcileData is nil or doesn't contain any
// aliases, the cluster will be removed from the structure.
func (a *aliasToCluster) ReplaceCluster(clusterID resource.ID, rd *ReconcileData) error {
	if rd == nil || len(rd.AliasPort) == 0 {
		got, ok := a.slc.FindBy(func(e *clusterData) bool { return clusterID == e.clusterID })
		if !ok {
			return nil
		}

		for _, als := range got.aliases {
			a.aliasToPort.Delete(als)
		}

		ok = a.slc.Remove(got)
		if !ok {
			return fmt.Errorf("cluster %q not found", clusterID)
		}

		return nil
	}

	got, ok := a.slc.FindBy(func(e *clusterData) bool { return clusterID == e.clusterID })
	if !ok {
		for als := range rd.AliasesData() {
			if res := a.aliasToPort.Get(alias(als)); res.F1 != nil {
				return fmt.Errorf("alias %q already exists and used by cluster %q", als, res.F1.clusterID)
			}
		}

		got = &clusterData{
			clusterID: clusterID,
			hosts:     rd.Hosts,
			aliases:   toSortedSlice(rd),
		}

		a.slc.Replace(got)
	} else {
		newAliases := toSortedSlice(rd)
		if !slices.Equal(got.aliases, newAliases) {
			for _, als := range got.aliases {
				a.aliasToPort.Delete(als)
			}

			got.inUsePort = ""
		}

		got.hosts = rd.Hosts
	}

	for als, p := range rd.AliasPort {
		a.aliasToPort.Replace(alias(als), pair.MakePair(got, port(p)))
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
	got := a.aliasToPort.Get(als)
	if ptr := got.F1; ptr != nil {
		return ptr.clusterID, got.F2, true
	}

	return "", "", false
}

// ClusterData returns the cluster data for the given cluster ID. If the cluster doesn't exist, the function returns nil.
func (a *aliasToCluster) ClusterData(id resource.ID) *clusterData {
	if val, ok := a.slc.FindBy(func(d *clusterData) bool { return d.clusterID == id }); ok {
		return val
	}

	return nil
}

// SetActivePort sets the in-use port for the given cluster ID. If the cluster doesn't exist, the function returns an error.
func (a *aliasToCluster) SetActivePort(id resource.ID, p port) error {
	if val, ok := a.slc.FindBy(func(d *clusterData) bool { return d.clusterID == id }); ok {
		val.inUsePort = p

		return nil
	}

	return fmt.Errorf("cluster %q not found", id)
}

// ActiveHostsPort returns the active hosts and port for the given cluster ID. If the cluster doesn't exist, the function
// finds the first cluster with the given ID and returns its hosts and in-use port, also setting the in-use port for the
// cluster.
func (a *aliasToCluster) ActiveHostsPort(clusterID resource.ID) ([]string, port) {
	val, ok := a.slc.FindBy(func(d *clusterData) bool { return d.clusterID == clusterID })
	if !ok {
		return nil, ""
	}

	for _, v := range a.aliasToPort.All() {
		if v.F1 == val {
			val.inUsePort = v.F2

			return val.hosts, v.F2
		}
	}

	return nil, ""
}

func (a *aliasToCluster) DropAlias(als alias) *clusterData {
	removed, ok := a.aliasToPort.Delete(als)
	if !ok {
		return nil
	}

	clusterPtr := removed.F1

	if clusterPtr.inUsePort == removed.F2 {
		clusterPtr.inUsePort = ""
	}

	return clusterPtr
}

// All returns all the alias data. The function returns a triple: alias, it's
// port and the cluster data.
func (a *aliasToCluster) All() iter.Seq2[pair.Pair[alias, port], *clusterData] {
	return func(yield func(pair.Pair[alias, port], *clusterData) bool) {
		for als, v := range a.aliasToPort.All() {
			ptr := v.F1
			if ptr == nil {
				continue
			}

			if !yield(pair.MakePair(als, v.F2), ptr) {
				return
			}
		}
	}
}

type clusterData struct {
	clusterID resource.ID
	inUsePort port
	hosts     []string
	aliases   []alias
}

func (a *clusterData) Equal(other *clusterData) bool { return a.clusterID == other.clusterID }

// uniqueSlice is a data structure that ensures that the elements are unique
// (using Equal(T) contract). It also allows for a callback to be called when the
// element is removed.
type uniqueSlice[E elem[E]] []E

// Replace replaces the element in the slice. If the element doesn't exist, it's
// added to the slice. The function returns the old element and a boolean
// indicating if the element was replaced.
func (s *uniqueSlice[E]) Replace(val E) (old E, replaced bool) {
	idx := slices.IndexFunc(*s, func(e E) bool { return e.Equal(val) })
	if idx != -1 {
		prev := (*s)[idx]
		(*s)[idx] = val

		return prev, true
	}

	*s = append(*s, val)

	return *new(E), false
}

// FindBy finds the element in the slice. If the element doesn't exist, the
// function returns the zero value of the element and false.
func (s *uniqueSlice[E]) FindBy(pred func(E) bool) (E, bool) {
	if idx := slices.IndexFunc(*s, func(e E) bool { return pred(e) }); idx != -1 {
		return (*s)[idx], true
	}

	return *new(E), false
}

// Remove removes the element from the slice.
func (s *uniqueSlice[E]) Remove(val E) bool {
	if idx := slices.IndexFunc(*s, func(e E) bool { return e == val }); idx != -1 {
		*s = slices.Delete(*s, idx, idx+1)

		return true
	}

	return false
}

type elem[E any] interface {
	comparable
	Equal(E) bool
}

// actionMap is a data structure that maps keys to values.
type actionMap[K comparable, V any] struct {
	m map[K]V
}

// Replace replaces the value in the map. If the value doesn't exist, it's added
// to the map. The function returns the old value and a boolean indicating if the
// value was replaced.
func (a *actionMap[K, V]) Replace(k K, v V) (V, bool) {
	old, ok := a.Delete(k)

	if a.m == nil {
		a.m = map[K]V{}
	}

	a.m[k] = v

	return old, ok
}

// Delete deletes the value from the map. If the value doesn't exist, the
// function returns the zero value of the value and false.
func (a *actionMap[K, V]) Delete(k K) (V, bool) {
	if v, ok := a.m[k]; ok {
		delete(a.m, k)

		return v, true
	}

	return *new(V), false
}

// Get returns the value for the given key. If the value doesn't exist, the
// function returns the zero value of the value.
func (a *actionMap[K, V]) Get(k K) V {
	if v, ok := a.m[k]; ok {
		return v
	}

	return *new(V)
}

// All returns all the values in the map. The function returns a pair of key and value.
func (a *actionMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if a.m == nil {
			return
		}

		for k, v := range a.m {
			if !yield(k, v) {
				return
			}
		}
	}
}
