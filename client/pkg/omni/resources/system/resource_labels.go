// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/meta/spec"
	"github.com/gertd/go-pluralize"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewResourceLabels creates new ResourceLabels state.
func NewResourceLabels[T generic.ResourceWithRD](id string) *ResourceLabels[T] {
	return &ResourceLabels[T]{
		resource.NewMetadata(resources.DefaultNamespace, ResourceLabelsType[T](), id, resource.VersionUndefined),
	}
}

const suffix = "Labels"

type null struct{}

// MarshalProto implements ProtoMarshaler.
func (null) MarshalProto() ([]byte, error) {
	return nil, nil
}

// ResourceLabels is a special resource type that has no spec but stores all extracted labels from the source resource.
type ResourceLabels[T generic.ResourceWithRD] struct {
	md resource.Metadata
}

// Metadata implements Resource.
func (t *ResourceLabels[T]) Metadata() *resource.Metadata {
	return &t.md
}

// Spec implements resource.Resource.
func (t *ResourceLabels[T]) Spec() any {
	return null{}
}

// DeepCopy returns a deep copy of Resource.
func (t *ResourceLabels[T]) DeepCopy() resource.Resource { //nolint:ireturn
	return &ResourceLabels[T]{t.md}
}

// ResourceDefinition implements spec.ResourceDefinitionProvider interface.
func (t *ResourceLabels[T]) ResourceDefinition() spec.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResourceLabelsType[T](),
		DefaultNamespace: resources.DefaultNamespace,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (t *ResourceLabels[T]) UnmarshalProto(md *resource.Metadata, _ []byte) error {
	t.md = *md

	return nil
}

// ResourceLabelsType generates labels type for a resource type.
func ResourceLabelsType[T generic.ResourceWithRD]() string {
	var r T

	t, rest, ok := strings.Cut(r.ResourceDefinition().Type, ".")
	if !ok {
		panic(fmt.Sprintf("couldn't split resource type %q into parts", r.ResourceDefinition().Type))
	}

	t = pluralize.NewClient().Singular(t)

	return t + suffix + "." + rest
}
