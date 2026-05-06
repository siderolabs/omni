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

// ImageFactoryAuthID is the default and the only allowed ID for ImageFactoryAuth resource.
//
// tsgen:ImageFactoryAuthID
const ImageFactoryAuthID = "image-factory-auth"

// NewImageFactoryAuth creates a new ImageFactoryAuth resource.
func NewImageFactoryAuth() *ImageFactoryAuth {
	return typed.NewResource[ImageFactoryAuthSpec, ImageFactoryAuthExtension](
		resource.NewMetadata(resources.VirtualNamespace, ImageFactoryAuthType, ImageFactoryAuthID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ImageFactoryAuthSpec{}),
	)
}

const (
	// ImageFactoryAuthType is the type of ImageFactoryAuth resource.
	//
	// tsgen:ImageFactoryAuthType
	ImageFactoryAuthType = resource.Type("ImageFactoryAuths.omni.sidero.dev")
)

// ImageFactoryAuth resource returns current auth credentials for the image factory.
type ImageFactoryAuth = typed.Resource[ImageFactoryAuthSpec, ImageFactoryAuthExtension]

// ImageFactoryAuthSpec wraps specs.ImageFactoryAuthSpec.
type ImageFactoryAuthSpec = protobuf.ResourceSpec[specs.ImageFactoryAuthSpec, *specs.ImageFactoryAuthSpec]

// ImageFactoryAuthExtension providers auxiliary methods for ImageFactoryAuth resource.
type ImageFactoryAuthExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ImageFactoryAuthExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImageFactoryAuthType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Username",
				JSONPath: "{.username}",
			},
		},
	}
}
