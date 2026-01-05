// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewLabelsCompletion creates new LabelsCompletion resource.
func NewLabelsCompletion(id resource.ID) *LabelsCompletion {
	return typed.NewResource[LabelsCompletionSpec, LabelsCompletionExtension](
		resource.NewMetadata(resources.VirtualNamespace, LabelsCompletionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.LabelsCompletionSpec{}),
	)
}

const (
	// LabelsCompletionType is the type of the LabelsCompletion resource.
	// tsgen:LabelsCompletionType
	LabelsCompletionType = resource.Type("LabelsCompletions.omni.sidero.dev")
)

// LabelsCompletion contains the dictionary of all existing labels for any resource.
type LabelsCompletion = typed.Resource[LabelsCompletionSpec, LabelsCompletionExtension]

// LabelsCompletionSpec wraps specs.LabelsCompletionSpec.
type LabelsCompletionSpec = protobuf.ResourceSpec[specs.LabelsCompletionSpec, *specs.LabelsCompletionSpec]

// LabelsCompletionExtension provides auxiliary methods for LabelsCompletion resource.
type LabelsCompletionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LabelsCompletionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LabelsCompletionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
