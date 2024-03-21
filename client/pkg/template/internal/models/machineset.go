// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineSet is a base model for controlplane and workers.
type MachineSet struct {
	Meta             `yaml:",inline"`
	SystemExtensions `yaml:",inline"`

	// Name is the name of the machine set. When empty, the default name will be used.
	Name string `yaml:"name,omitempty"`

	// Descriptors are the user descriptors to apply to the cluster.
	Descriptors Descriptors `yaml:",inline"`

	BootstrapSpec *BootstrapSpec `yaml:"bootstrapSpec,omitempty"`

	// MachineSet machines.
	Machines MachineIDList `yaml:"machines,omitempty"`

	MachineClass *MachineClassConfig `yaml:"machineClass,omitempty"`

	// UpdateStrategy defines the update strategy for the machine set.
	UpdateStrategy *UpdateStrategyConfig `yaml:"updateStrategy,omitempty"`

	// DeleteStrategy defines the delete strategy for the machine set.
	DeleteStrategy *UpdateStrategyConfig `yaml:"deleteStrategy,omitempty"`

	// MachineSet patches.
	Patches PatchList `yaml:"patches,omitempty"`
}

// BootstrapSpec defines the model for setting the bootstrap specification, i.e. restoring from a backup, in the machine set.
// Only valid for the control plane machine set.
type BootstrapSpec struct {
	// ClusterUUID defines the UUID of the cluster to restore from.
	ClusterUUID string `yaml:"clusterUUID"`

	// Snapshot defines the snapshot file name to restore from.
	Snapshot string `yaml:"snapshot"`
}

// MachineClassConfig defines the model for setting the machine class based machine selector in the machine set.
type MachineClassConfig struct {
	// Name defines used machine class name.
	Name string `yaml:"name"`

	// Size sets the number of machines to be pulled from the machine class.
	Size Size `yaml:"size"`
}

// Size extends protobuf generated allocation type enum to parse string constants.
type Size struct {
	Value          uint32
	AllocationType specs.MachineSetSpec_MachineClass_AllocationType
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Size) UnmarshalYAML(unmarshal func(any) error) error {
	var value string

	if err := unmarshal(&value); err != nil {
		return err
	}

	switch value {
	case "unlimited", "∞", "infinity":
		value = "Unlimited"
	}

	v, ok := specs.MachineSetSpec_MachineClass_AllocationType_value[value]

	if !ok {
		c.AllocationType = specs.MachineSetSpec_MachineClass_Static

		count, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid machine count %s: %w", value, err)
		}

		c.Value = uint32(count)
	}

	c.AllocationType = specs.MachineSetSpec_MachineClass_AllocationType(v)

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (c Size) MarshalYAML() (any, error) {
	if c.AllocationType != specs.MachineSetSpec_MachineClass_Static {
		return specs.MachineSetSpec_MachineClass_AllocationType_name[int32(c.AllocationType)], nil
	}

	return c.Value, nil
}

// UpdateStrategyType extends protobuf generated update strategy enum to parse string constants.
type UpdateStrategyType uint32

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *UpdateStrategyType) UnmarshalYAML(unmarshal func(any) error) error {
	var value string

	if err := unmarshal(&value); err != nil {
		return err
	}

	v, ok := specs.MachineSetSpec_UpdateStrategy_value[value]

	if !ok {
		return fmt.Errorf("invalid update strategy type %s", value)
	}

	*t = UpdateStrategyType(v)

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (t UpdateStrategyType) MarshalYAML() (any, error) {
	return specs.MachineSetSpec_UpdateStrategy_name[int32(t)], nil
}

// RollingUpdateStrategyConfig defines the model for setting the rolling update strategy in the machine set.
type RollingUpdateStrategyConfig struct {
	MaxParallelism uint32 `yaml:"maxParallelism,omitempty"`
}

// UpdateStrategyConfig defines the model for setting the update strategy in the machine set.
type UpdateStrategyConfig struct {
	Type    *UpdateStrategyType          `yaml:"type,omitempty"`
	Rolling *RollingUpdateStrategyConfig `yaml:"rolling,omitempty"`
}

// Validate checks the machine set fields correctness.
func (machineset *MachineSet) Validate() error {
	var multiErr error

	if err := machineset.Descriptors.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if len(machineset.Machines) > 0 && machineset.MachineClass != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("machine set can not have both machines and machine class defined"))
	}

	return multiErr
}

// Translate the model.
func (machineset *MachineSet) translate(ctx TranslateContext, nameSuffix, roleLabel string) ([]resource.Resource, error) {
	id := omni.AdditionalWorkersResourceID(ctx.ClusterName, nameSuffix)

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, id)
	machineSet.Metadata().Labels().Set(omni.LabelCluster, ctx.ClusterName)
	machineSet.Metadata().Labels().Set(roleLabel, "")

	machineset.Descriptors.Apply(machineSet)

	machineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling // Update strategy is Rolling when not specified.

	if machineset.UpdateStrategy != nil {
		if machineset.UpdateStrategy.Type != nil {
			machineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_UpdateStrategy(*machineset.UpdateStrategy.Type)
		}

		if machineset.UpdateStrategy.Rolling != nil {
			machineSet.TypedSpec().Value.UpdateStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: machineset.UpdateStrategy.Rolling.MaxParallelism,
				},
			}
		}
	}

	if machineset.DeleteStrategy != nil {
		if machineset.DeleteStrategy.Type != nil {
			machineSet.TypedSpec().Value.DeleteStrategy = specs.MachineSetSpec_UpdateStrategy(*machineset.DeleteStrategy.Type)
		}

		if machineset.DeleteStrategy.Rolling != nil {
			machineSet.TypedSpec().Value.DeleteStrategyConfig = &specs.MachineSetSpec_UpdateStrategyConfig{
				Rolling: &specs.MachineSetSpec_RollingUpdateStrategyConfig{
					MaxParallelism: machineset.DeleteStrategy.Rolling.MaxParallelism,
				},
			}
		}
	}

	resourceList := []resource.Resource{machineSet}

	if machineset.BootstrapSpec != nil {
		machineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
			ClusterUuid: machineset.BootstrapSpec.ClusterUUID,
			Snapshot:    machineset.BootstrapSpec.Snapshot,
		}
	}

	if machineset.MachineClass != nil {
		machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
			Name:           machineset.MachineClass.Name,
			MachineCount:   machineset.MachineClass.Size.Value,
			AllocationType: machineset.MachineClass.Size.AllocationType,
		}
	} else {
		for _, machineID := range machineset.Machines {
			machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, string(machineID), machineSet)
			descriptors := ctx.MachineDescriptors[machineID]

			descriptors.Apply(machineSetNode)

			_, locked := ctx.LockedMachines[machineID]
			if locked {
				machineSetNode.Metadata().Annotations().Set(omni.MachineLocked, "")
			}

			resourceList = append(resourceList, machineSetNode)
		}
	}

	patches, err := machineset.Patches.Translate(
		id,
		constants.PatchBaseWeightMachineSet,
		pair.MakePair(omni.LabelCluster, ctx.ClusterName),
		pair.MakePair(omni.LabelMachineSet, id),
	)
	if err != nil {
		return nil, err
	}

	resourceList = append(resourceList, patches...)

	schematicConfigurations := machineset.SystemExtensions.translate(
		ctx,
		id,
		pair.MakePair(omni.LabelMachineSet, id),
	)

	return append(resourceList, schematicConfigurations...), nil
}
