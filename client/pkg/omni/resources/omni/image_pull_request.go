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

// NewImagePullRequest creates a new ImagePullRequest state.
func NewImagePullRequest(ns, id string) *ImagePullRequest {
	return typed.NewResource[ImagePullRequestSpec, ImagePullRequestExtension](
		resource.NewMetadata(ns, ImagePullRequestType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ImagePullRequestSpec{}),
	)
}

// ImagePullRequestType is the type of the ImagePullRequest resource.
//
// tsgen:ImagePullRequestType
const ImagePullRequestType = resource.Type("ImagePullRequests.omni.sidero.dev")

// ImagePullRequest resource contains a request to pull a set of container images to a set of nodes.
type ImagePullRequest = typed.Resource[ImagePullRequestSpec, ImagePullRequestExtension]

// ImagePullRequestSpec wraps specs.ImagePullRequestSpec.
type ImagePullRequestSpec = protobuf.ResourceSpec[specs.ImagePullRequestSpec, *specs.ImagePullRequestSpec]

// ImagePullRequestExtension providers auxiliary methods for ImagePullRequest resource.
type ImagePullRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ImagePullRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImagePullRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
