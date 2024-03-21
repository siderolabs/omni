// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineIDList is a list of Machine UUIDs.
type MachineIDList []MachineID

// MachineID is a Machine UUID.
type MachineID string

// KindMachine is a Machine model kind.
const KindMachine = "Machine"

// Machine provides customization for a specific machine.
type Machine struct { //nolint:govet
	Meta             `yaml:",inline"`
	SystemExtensions `yaml:",inline"`

	// Machine name (ID).
	Name MachineID `yaml:"name"`

	// Descriptors are the user descriptors to apply to the cluster.
	Descriptors Descriptors `yaml:",inline"`

	// Locked locks the machine, so no config updates, upgrades and downgrades will be performed on the machine.
	Locked bool `yaml:"locked,omitempty"`

	// Install specification.
	Install MachineInstall `yaml:"install,omitempty"`

	// ClusterMachine patches.
	Patches PatchList `yaml:"patches,omitempty"`
}

// MachineInstall provides machine install configuration.
type MachineInstall struct {
	// Disk device name.
	Disk string `yaml:"disk"`
}

// Validate the list of machines.
func (l MachineIDList) Validate() error {
	var multiErr error

	for _, id := range l {
		multiErr = joinErrors(multiErr, id.Validate())
	}

	return multiErr
}

// Validate the model.
func (id MachineID) Validate() error {
	if _, err := uuid.Parse(string(id)); err != nil {
		return fmt.Errorf("invalid machine ID %q: %w", id, err)
	}

	return nil
}

// Validate the model.
func (install *MachineInstall) Validate() error {
	return nil
}

// Validate the model.
func (machine *Machine) Validate() error {
	var multiErr error

	if machine.Name == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("name is required for machine"))
	}

	if err := machine.Descriptors.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	multiErr = joinErrors(multiErr, machine.Name.Validate(), machine.Install.Validate(), machine.Patches.Validate())

	if multiErr != nil {
		return fmt.Errorf("machine %q is invalid: %w", machine.Name, multiErr)
	}

	return nil
}

// Translate the model.
func (machine *Machine) Translate(ctx TranslateContext) ([]resource.Resource, error) {
	var resourceList []resource.Resource

	if machine.Install.Disk != "" {
		patch := Patch{
			Name: "install-disk",
			Inline: map[string]any{
				"machine": map[string]any{
					"install": map[string]any{
						"disk": machine.Install.Disk,
					},
				},
			},
		}

		patchResource, err := patch.Translate(
			fmt.Sprintf("cm-%s", machine.Name),
			constants.PatchWeightInstallDisk,
			pair.MakePair(omni.LabelCluster, ctx.ClusterName),
			pair.MakePair(omni.LabelClusterMachine, string(machine.Name)),
			pair.MakePair(omni.LabelSystemPatch, ""),
		)
		if err != nil {
			return nil, err
		}

		resourceList = append(resourceList, patchResource)
	}

	patches, err := machine.Patches.Translate(
		fmt.Sprintf("cm-%s", machine.Name),
		constants.PatchBaseWeightClusterMachine,
		pair.MakePair(omni.LabelCluster, ctx.ClusterName),
		pair.MakePair(omni.LabelClusterMachine, string(machine.Name)),
	)
	if err != nil {
		return nil, err
	}

	resourceList = append(resourceList, patches...)

	schematicConfigurations := machine.SystemExtensions.translate(
		ctx,
		string(machine.Name),
		pair.MakePair(omni.LabelClusterMachine, string(machine.Name)),
	)

	return append(resourceList, schematicConfigurations...), nil
}

func init() {
	register[Machine](KindMachine)
}
