// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package dns

// NewInfo creates an Info with the given fields, including the unexported address.
func NewInfo(cluster, id, name, address string) Info {
	return Info{
		Cluster: cluster,
		ID:      id,
		Name:    name,
		address: address,
	}
}
