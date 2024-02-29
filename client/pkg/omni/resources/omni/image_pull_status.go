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

// NewImagePullStatus creates a new ImagePullStatus state.
func NewImagePullStatus(ns, id string) *ImagePullStatus {
	return typed.NewResource[ImagePullStatusSpec, ImagePullStatusExtension](
		resource.NewMetadata(ns, ImagePullStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ImagePullStatusSpec{}),
	)
}

// ImagePullStatusType is the type of the ImagePullStatus resource.
//
// tsgen:ImagePullStatusType
const ImagePullStatusType = resource.Type("ImagePullStatuses.omni.sidero.dev")

// ImagePullStatus resource contains a request to pull a set of container images to a set of nodes.
type ImagePullStatus = typed.Resource[ImagePullStatusSpec, ImagePullStatusExtension]

// ImagePullStatusSpec wraps specs.ImagePullStatusSpec.
type ImagePullStatusSpec = protobuf.ResourceSpec[specs.ImagePullStatusSpec, *specs.ImagePullStatusSpec]

// ImagePullStatusExtension providers auxiliary methods for ImagePullStatus resource.
type ImagePullStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ImagePullStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImagePullStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Processed",
				JSONPath: "{.processedcount}",
			},
			{
				Name:     "Total",
				JSONPath: "{.totalcount}",
			},
			{
				Name:     "Last Node",
				JSONPath: "{.lastprocessednode}",
			},
			{
				Name:     "Last Image",
				JSONPath: "{.lastprocessedimage}",
			},
			{
				Name:     "Last Error",
				JSONPath: "{.lastprocessederror}",
			},
			{
				Name:     "Request Version",
				JSONPath: "{.requestversion}",
			},
		},
	}
}
