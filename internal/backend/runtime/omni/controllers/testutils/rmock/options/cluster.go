// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package options

import "github.com/siderolabs/omni/client/pkg/omni/resources/omni"

// WithTalosVersion creates a cluster with a version defined.
func WithTalosVersion(v string) MockOption {
	return Modify(func(res *omni.Cluster) error {
		res.TypedSpec().Value.TalosVersion = v

		return nil
	})
}
