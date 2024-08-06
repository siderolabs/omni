// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineStatus creates new MachineStatus state.
func NewMachineStatus(ns, id string) *MachineStatus {
	return typed.NewResource[MachineStatusSpec, MachineStatusExtension](
		resource.NewMetadata(ns, MachineStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineStatusSpec{}),
	)
}

// MachineStatusType is the type of MachineStatus resource.
//
// tsgen:MachineStatusType
const MachineStatusType = resource.Type("MachineStatuses.omni.sidero.dev")

// MachineStatus resource contains current information about the Machine.
//
// MachineStatus contains node information like hostname,
//
// MachineStatus resource ID is a Machine UUID.
type MachineStatus = typed.Resource[MachineStatusSpec, MachineStatusExtension]

// MachineStatusSpec wraps specs.MachineStatusSpec.
type MachineStatusSpec = protobuf.ResourceSpec[specs.MachineStatusSpec, *specs.MachineStatusSpec]

// MachineStatusExtension providers auxiliary methods for MachineStatus resource.
type MachineStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// Make implements [typed.Maker] interface.
func (MachineStatusExtension) Make(_ *resource.Metadata, spec *MachineStatusSpec) any {
	return (*machineStatusAux)(spec)
}

type machineStatusAux MachineStatusSpec

func (m *machineStatusAux) Match(searchFor string) bool {
	val := m.Value

	if strings.Contains(val.GetNetwork().GetHostname(), searchFor) ||
		strings.Contains(val.GetPlatformMetadata().GetHostname(), searchFor) {
		return true
	}

	for _, link := range val.GetNetwork().GetNetworkLinks() {
		if strings.Contains(link.GetHardwareAddress(), searchFor) {
			return true
		}
	}

	return strings.Contains(val.GetCluster(), searchFor)
}

func (m *machineStatusAux) Field(fieldName string) (string, bool) {
	val := m.Value

	switch fieldName {
	case LabelSuffixCluster:
		return val.GetCluster(), true
	case LabelSuffixHostname:
		return val.GetNetwork().GetHostname(), true
	case LabelSuffixPlatform:
		return val.GetPlatformMetadata().GetPlatform(), true
	case LabelSuffixArch:
		return val.GetHardware().GetArch(), true
	default:
		return "", false
	}
}

func setLabel(labels *resource.Labels, key string, valueFunc func() string) {
	if value := valueFunc(); value != "" {
		labels.Set(key, value)
	} else {
		labels.Delete(key)
	}
}

func setLabelOptional(labels *resource.Labels, key string, valueFunc func() optional.Optional[string]) {
	if value := valueFunc(); value.IsPresent() {
		labels.Set(key, value.ValueOr(""))
	} else {
		labels.Delete(key)
	}
}

// MachineStatusReconcileLabels builds a set of labels based on hardware/meta information.
//
//nolint:gocognit
func MachineStatusReconcileLabels(machineStatus *MachineStatus) {
	labels := machineStatus.Metadata().Labels()

	setLabel(labels, MachineStatusLabelCores, func() string {
		numCores := 0

		for _, cpu := range machineStatus.TypedSpec().Value.GetHardware().GetProcessors() {
			numCores += int(cpu.GetCoreCount())
		}

		if numCores > 0 {
			return strconv.Itoa(numCores)
		}

		return ""
	})

	setLabel(labels, MachineStatusLabelCPU, func() string {
		cpuManufacturer := ""

		for _, cpu := range machineStatus.TypedSpec().Value.GetHardware().GetProcessors() {
			if cpu.GetManufacturer() != "" {
				cpuManufacturer = cpu.GetManufacturer()
			}
		}

		cpuManufacturer = strings.ToLower(strings.TrimSpace(cpuManufacturer))

		switch {
		case strings.HasPrefix(cpuManufacturer, "intel"):
			return "intel"
		case strings.HasPrefix(cpuManufacturer, "amd"):
			return "amd"
		default:
			return cpuManufacturer
		}
	})

	setLabel(labels, MachineStatusLabelArch, func() string {
		return strings.ToLower(machineStatus.TypedSpec().Value.GetHardware().GetArch())
	})

	setLabel(labels, MachineStatusLabelMem, func() string {
		memMB := 0

		for _, mem := range machineStatus.TypedSpec().Value.GetHardware().GetMemoryModules() {
			memMB += int(mem.GetSizeMb())
		}

		if memMB >= 1024 {
			return fmt.Sprintf("%dGiB", memMB/1024)
		}

		return ""
	})

	setLabel(labels, MachineStatusLabelStorage, func() string {
		storageSize := uint64(0)

		for _, blockDevice := range machineStatus.TypedSpec().Value.GetHardware().GetBlockdevices() {
			storageSize += blockDevice.GetSize()
		}

		if storageSize >= 1000*1000*1000 {
			return fmt.Sprintf("%dGB", storageSize/(1000*1000*1000))
		}

		return ""
	})

	setLabel(labels, MachineStatusLabelNet, func() string {
		netMbps := uint64(0)

		for _, net := range machineStatus.TypedSpec().Value.GetNetwork().GetNetworkLinks() {
			if net.GetSpeedMbps() >= 10 && net.GetSpeedMbps() <= 100000 {
				netMbps += uint64(net.GetSpeedMbps())
			}
		}

		switch {
		case netMbps >= 1000:
			return fmt.Sprintf("%dGbps", netMbps/1000)
		case netMbps > 0:
			return fmt.Sprintf("%dMbps", netMbps)
		default:
			return ""
		}
	})

	setLabel(labels, MachineStatusLabelPlatform, func() string {
		return machineStatus.TypedSpec().Value.PlatformMetadata.GetPlatform()
	})

	setLabel(labels, MachineStatusLabelRegion, func() string {
		return machineStatus.TypedSpec().Value.PlatformMetadata.GetRegion()
	})

	setLabel(labels, MachineStatusLabelZone, func() string {
		return machineStatus.TypedSpec().Value.PlatformMetadata.GetZone()
	})

	setLabel(labels, MachineStatusLabelInstance, func() string {
		return machineStatus.TypedSpec().Value.PlatformMetadata.GetInstanceType()
	})

	setLabel(labels, MachineStatusLabelTalosVersion, func() string {
		return machineStatus.TypedSpec().Value.TalosVersion
	})

	setLabelOptional(labels, MachineStatusLabelInstalled, func() optional.Optional[string] {
		if machineStatus.TypedSpec().Value.Hardware == nil {
			return optional.None[string]()
		}

		installed := slices.IndexFunc(machineStatus.TypedSpec().Value.Hardware.Blockdevices,
			func(dev *specs.MachineStatusSpec_HardwareStatus_BlockDevice) bool {
				return dev.SystemDisk
			},
		) != -1

		if installed {
			return optional.Some("")
		}

		return optional.None[string]()
	})
}

// GetMachineStatusSystemDisk looks up a system disk for the Talos machine.
func GetMachineStatusSystemDisk(res *MachineStatus) string {
	if res == nil || res.TypedSpec().Value.Hardware == nil {
		return ""
	}

	for _, disk := range res.TypedSpec().Value.Hardware.Blockdevices {
		if disk.SystemDisk {
			return disk.LinuxName
		}
	}

	return ""
}
