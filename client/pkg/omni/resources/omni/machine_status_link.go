// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineStatusLink creates new MachineStatusLink resource.
func NewMachineStatusLink(id resource.ID) *MachineStatusLink {
	return typed.NewResource[MachineStatusLinkSpec, MachineStatusLinkExtension](
		resource.NewMetadata(resources.MetricsNamespace, MachineStatusLinkType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineStatusLinkSpec{}),
	)
}

// MachineStatusLink resource contains current information about the [MachineStatus] and [siderolink.LinkCounter] resources.
type MachineStatusLink = typed.Resource[MachineStatusLinkSpec, MachineStatusLinkExtension]

// MachineStatusLinkSpec wraps specs.MachineStatusLinkSpec.
type MachineStatusLinkSpec = protobuf.ResourceSpec[specs.MachineStatusLinkSpec, *specs.MachineStatusLinkSpec]

// MachineStatusLinkType is the type of MachineStatusLink resource.
//
// tsgen:MachineStatusLinkType
const MachineStatusLinkType = resource.Type("MachineStatusLinks.omni.sidero.dev")

// MachineStatusLinkExtension providers auxiliary methods for MachineStatusLink resource.
type MachineStatusLinkExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStatusLinkExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineStatusLinkType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.MetricsNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Bytes Received",
				JSONPath: "{.siderolinkcounter.bytesreceived}",
			},
			{
				Name:     "Bytes Sent",
				JSONPath: "{.siderolinkcounter.bytessent}",
			},
			{
				Name:     "Last Alive",
				JSONPath: "{.siderolinkcounter.lastalive}",
			},
		},
	}
}

// Make implements [typed.Maker] interface.
func (MachineStatusLinkExtension) Make(_ *resource.Metadata, spec *MachineStatusLinkSpec) any {
	return (*machineStatusLinkAux)(spec)
}

type machineStatusLinkAux MachineStatusLinkSpec

func (m *machineStatusLinkAux) Match(searchFor string) bool {
	val := m.Value

	if strings.Contains(val.GetMessageStatus().GetNetwork().GetHostname(), searchFor) ||
		strings.Contains(val.GetMessageStatus().GetPlatformMetadata().GetHostname(), searchFor) {
		return true
	}

	for _, link := range val.GetMessageStatus().GetNetwork().GetNetworkLinks() {
		if strings.Contains(link.GetHardwareAddress(), searchFor) {
			return true
		}
	}

	return strings.Contains(val.GetMessageStatus().GetCluster(), searchFor)
}

func (m *machineStatusLinkAux) Field(fieldName string) (string, bool) {
	val := m.Value

	switch fieldName {
	case "cluster":
		return val.GetMessageStatus().GetCluster(), true
	case "hostname":
		return val.GetMessageStatus().GetNetwork().GetHostname(), true
	case "platform":
		return val.GetMessageStatus().GetPlatformMetadata().GetPlatform(), true
	case "arch":
		return val.GetMessageStatus().GetHardware().GetArch(), true
	case "last_alive":
		return fmt.Sprintf("%020d", val.GetSiderolinkCounter().GetLastAlive().GetSeconds()), true
	case "machine_created_at":
		return fmt.Sprintf("%020d", val.GetMachineCreatedAt()), true
	default:
		return "", false
	}
}
