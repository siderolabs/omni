// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration_test

import (
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
)

type null struct{}

func (d null) DeepCopy() null {
	return d
}

type machineClassStatusExtension struct{}

func (machineClassStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             migration.MachineClassStatusType,
		DefaultNamespace: resources.DefaultNamespace,
	}
}

type machineSetRequiredMachinesExtension struct{}

func (machineSetRequiredMachinesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             migration.MachineSetRequiredMachinesType,
		DefaultNamespace: resources.DefaultNamespace,
	}
}
