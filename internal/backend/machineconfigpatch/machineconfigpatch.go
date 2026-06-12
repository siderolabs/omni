// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machineconfigpatch extracts the configuration a machine arrives with into a user-managed ConfigPatch.
package machineconfigpatch

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	talossiderolink "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// configPatchIDPrefix is a low-priority prefix, so that user-created machine config patches (default priority) override the preserved one.
const configPatchIDPrefix = "000-preserved-machine-config-"

// Extractor extracts the config a machine arrives with into a user-managed ConfigPatch on first connection.
type Extractor struct {
	state  state.State
	logger *zap.Logger
}

// NewExtractor creates a new Extractor.
func NewExtractor(state state.State, logger *zap.Logger) (*Extractor, error) {
	if state == nil {
		return nil, fmt.Errorf("state is nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Extractor{
		state:  state,
		logger: logger,
	}, nil
}

// Extract extracts the preservable documents from the observed machine config into a machine-level ConfigPatch.
//
// The Omni-managed connection documents (SideroLink, event sink, kmsg log) are dropped, as Omni regenerates them.
// The rest is preserved so it survives the machine lifecycle.
//
// It returns a non-empty reason (and creates no patch) when the extracted documents do not form a valid config patch.
// We create the patch by bypassing the regular config patch validation, so we validate here to never inject a patch the
// user could not have created or could not edit afterwards.
//
// It is a no-op (empty reason, no error) if there is nothing worth preserving, or if the patch already exists.
func (extractor *Extractor) Extract(ctx context.Context, id resource.ID, observedConfig []byte) (string, error) {
	if len(observedConfig) == 0 {
		return "", nil
	}

	provider, err := configloader.NewFromBytes(observedConfig)
	if err != nil {
		return "", fmt.Errorf("failed to load observed machine config: %w", err)
	}

	documents := provider.Documents()
	preserved := make([]documentconfig.Document, 0, len(documents))

	for _, document := range documents {
		if isConnectionDocument(document) {
			continue
		}

		preserved = append(preserved, document)
	}

	if len(preserved) == 0 {
		return "", nil
	}

	preservedContainer, err := container.New(preserved...)
	if err != nil {
		return "", fmt.Errorf("failed to build preserved config container: %w", err)
	}

	data, err := preservedContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return "", fmt.Errorf("failed to encode preserved config: %w", err)
	}

	// we create the patch bypassing the regular config patch validation, so validate here against the same rules the user
	// faces, to never surface a patch the user could never have created or could not edit afterwards.
	if validationErr := omni.ValidateConfigPatch(data); validationErr != nil {
		return fmt.Sprintf("incoming machine config cannot be preserved as a config patch: %s", validationErr), nil
	}

	configPatch := omni.NewConfigPatch(configPatchIDPrefix + id)

	if err = configPatch.TypedSpec().Value.SetUncompressedData(data); err != nil {
		return "", fmt.Errorf("failed to set preserved config patch data: %w", err)
	}

	configPatch.Metadata().Labels().Set(omni.LabelMachine, id)
	configPatch.Metadata().Annotations().Set(omni.ConfigPatchName, "preserved-machine-config")
	configPatch.Metadata().Annotations().Set(omni.ConfigPatchDescription, "Machine configuration preserved from the machine on its first connection to Omni.")

	if err = extractor.state.Create(ctx, configPatch); err != nil && !state.IsConflictError(err) {
		return "", fmt.Errorf("failed to create preserved machine config patch: %w", err)
	}

	return "", nil
}

// isConnectionDocument reports whether the document is one of the Omni-managed connection documents that Omni regenerates itself.
func isConnectionDocument(document documentconfig.Document) bool {
	switch document.Kind() {
	case talossiderolink.Kind, runtimecfg.EventSinkKind, runtimecfg.KmsgLogKind:
		return true
	default:
		return false
	}
}
