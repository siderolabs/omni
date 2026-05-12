// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func schematicConfigurationValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(
			func(_ context.Context, res *omni.SchematicConfiguration, _ ...state.CreateOption) error {
				return validateSchematicConfiguration(res)
			},
		)),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(_ context.Context, _, res *omni.SchematicConfiguration, _ ...state.UpdateOption) error {
				return validateSchematicConfiguration(res)
			},
		)),
	}
}

func validateSchematicConfiguration(schematicConfiguration *omni.SchematicConfiguration) error {
	var targetValid bool

	labels := []string{
		omni.LabelClusterMachine,
		omni.LabelMachineSet,
		omni.LabelCluster,
	}

	for _, label := range labels {
		_, targetValid = schematicConfiguration.Metadata().Labels().Get(label)
		if targetValid {
			break
		}
	}

	if !targetValid {
		return fmt.Errorf("schematic configuration should have one of %q labels", strings.Join(labels, ", "))
	}

	if schematicConfiguration.TypedSpec().Value.SchematicId == "" {
		return fmt.Errorf("schematic ID can not be empty")
	}

	return nil
}
