// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra_test

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/infra/internal/resources"
)

const providerID = "test"

// NewTestResource creates new test resource.
func NewTestResource(ns, id string) *TestResource {
	return typed.NewResource[TestResourceSpec, TestResourceExtension](
		resource.NewMetadata(ns, TestResourceType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSpec{}),
	)
}

const TestResourceType = resource.Type("Resource.test.sidero.dev")

// TestResource describes virtual machine configuration.
type TestResource = typed.Resource[TestResourceSpec, TestResourceExtension]

// TestResourceSpec wraps specs.TestResourceSpec.
type TestResourceSpec = protobuf.ResourceSpec[specs.MachineSpec, *specs.MachineSpec]

// TestResourceExtension providers auxiliary methods for TestResource resource.
type TestResourceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (TestResourceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TestResourceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.ResourceNamespace(providerID),
		PrintColumns:     []meta.PrintColumn{},
	}
}
