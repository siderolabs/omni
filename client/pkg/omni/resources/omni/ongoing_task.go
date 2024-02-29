// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewOngoingTask creates new OngoingTask state.
func NewOngoingTask(ns, id string) *OngoingTask {
	return typed.NewResource[OngoingTaskSpec, OngoingTaskExtension](
		resource.NewMetadata(ns, OngoingTaskType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.OngoingTaskSpec{}),
	)
}

// OngoingTaskType is the type of OngoingTask resource.
//
// tsgen:OngoingTaskType
const OngoingTaskType = resource.Type("OngoingTasks.omni.sidero.dev")

// OngoingTask resource describes an ongoing background task.
type OngoingTask = typed.Resource[OngoingTaskSpec, OngoingTaskExtension]

// OngoingTaskSpec wraps specs.OngoingTaskSpec.
type OngoingTaskSpec = protobuf.ResourceSpec[specs.OngoingTaskSpec, *specs.OngoingTaskSpec]

// OngoingTaskExtension providers auxiliary methods for OngoingTask resource.
type OngoingTaskExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (OngoingTaskExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OngoingTaskType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Title",
				JSONPath: "{.title}",
			},
		},
	}
}
