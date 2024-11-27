// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package baremetal provides a client to interact with the bare-metal infra provider.
package baremetal

import (
	"github.com/jhump/grpctunnel"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

const (
	// ProviderIDMetadataKey is the key used to identify the infra provider ID in the GRPC metadata (header).
	ProviderIDMetadataKey = "provider-id"

	unknownProviderID = "unknown"
)

// GetAffinityKey extracts the AffinityKey from the given GRPC tunnel context.
func GetAffinityKey(channel grpctunnel.TunnelChannel, logger *zap.Logger) string {
	md, ok := metadata.FromIncomingContext(channel.Context())
	if !ok {
		logger.Warn("no metadata found in channel context")

		return unknownProviderID
	}

	vals := md.Get(ProviderIDMetadataKey)
	if len(vals) == 0 {
		logger.Warn("no provider ID found in metadata")

		return unknownProviderID
	}

	if len(vals) > 1 {
		logger.Warn("multiple provider IDs found in metadata, using the first one", zap.Strings("provider_ids", vals))
	}

	return vals[0]
}
