// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"github.com/cosi-project/runtime/pkg/state"
)

type ManagementServer = managementServer

//nolint:revive
func NewManagementServer(st state.State) *ManagementServer {
	return &ManagementServer{
		omniState: st,
	}
}

func GenerateDest(apiurl string) (string, error) {
	return generateDest(apiurl)
}
