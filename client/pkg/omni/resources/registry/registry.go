// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry specifies the list of resources to be registered in the state.
package registry

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

// Resources defines all resources added to the resource definitions.
var Resources []generic.ResourceWithRD

// MustRegisterResource adds resource to the registry, registers it's protobuf decoders/encoders.
func MustRegisterResource[T any, R interface {
	protobuf.Res[T]
	meta.ResourceDefinitionProvider
}](
	resourceType resource.Type,
	r R,
) {
	Resources = append(Resources, r)

	err := protobuf.RegisterResource(resourceType, r)
	if err != nil {
		panic(fmt.Errorf("failed to register resource %T: %w", r, err))
	}
}
