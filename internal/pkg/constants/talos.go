// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package constants keeps all constants which are used project wide.
package constants

// TODO: we should consider defining that in Talos machinery package.
const (
	// APIDService Talos APID service ID.
	APIDService = "apid"
)

const (
	// TaskResetSystemDiskSpec the disk was reset by using a reset spec
	// In Omni we only reset and STATE and EPHEMERAL partitions.
	TaskResetSystemDiskSpec = "resetSystemDiskSpec"
)
