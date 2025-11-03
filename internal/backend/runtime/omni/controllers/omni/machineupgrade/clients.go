// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineupgrade

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

type TalosClientFactory interface {
	New(ctx context.Context, managementAddress string) (TalosClient, error)
}

// TalosClient is a minimal interface for a Talos client used in maintenance update process.
type TalosClient interface {
	io.Closer
	UpgradeWithOptions(ctx context.Context, opts ...client.UpgradeOption) (*machineapi.UpgradeResponse, error)
}

type talosCliFactory struct{}

func (t *talosCliFactory) New(ctx context.Context, managementAddress string) (TalosClient, error) {
	opts := talos.GetSocketOptions(managementAddress)
	opts = append(opts, client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), client.WithEndpoints(managementAddress))

	talosClient, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create talos client: %w", err)
	}

	return talosClient, nil
}
