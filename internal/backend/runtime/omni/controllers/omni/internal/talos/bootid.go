// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// GetBootID reads the machine's kernel boot ID from the BootID resource, which Talos 1.14 and later serve.
func GetBootID(ctx context.Context, st state.State) (string, error) {
	bootID, err := safe.ReaderGetByID[*runtime.BootID](ctx, st, runtime.BootIDID)
	if err != nil {
		return "", err
	}

	return bootID.TypedSpec().BootID, nil
}

// InMaintenance reports whether the machine runs in maintenance, read from the machine itself.
func InMaintenance(ctx context.Context, st state.State) (bool, error) {
	status, err := safe.ReaderGetByID[*runtime.MachineStatus](ctx, st, runtime.MachineStatusID)
	if err != nil {
		return false, err
	}

	return status.TypedSpec().Stage == runtime.MachineStageMaintenance, nil
}

// ConfigIsActive reports whether data is the machine's active config. A config applied in try mode is active until it is rolled back,
// and never becomes active when the try apply did not reach the machine.
func ConfigIsActive(ctx context.Context, st state.State, data []byte) (bool, error) {
	active, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, st, configres.ActiveID)
	if err != nil {
		return false, fmt.Errorf("failed to get the active config: %w", err)
	}

	return SameConfig(active.Provider(), data)
}

// SameConfig reports whether data holds the same config as provider, ignoring formatting and comments.
func SameConfig(provider config.Provider, data []byte) (bool, error) {
	other, err := configloader.NewFromBytes(data)
	if err != nil {
		return false, fmt.Errorf("failed to load the config: %w", err)
	}

	providerBytes, err := provider.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return false, err
	}

	otherBytes, err := other.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return false, err
	}

	return bytes.Equal(providerBytes, otherBytes), nil
}
