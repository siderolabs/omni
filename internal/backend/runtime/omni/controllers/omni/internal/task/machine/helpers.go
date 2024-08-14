// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func forEachResource[T resource.Resource](
	ctx context.Context,
	c *client.Client,
	namespace resource.Namespace,
	resourceType resource.Type,
	callback func(T) error,
) error {
	items, err := safe.StateList[T](ctx, c.COSI, resource.NewMetadata(namespace, resourceType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for val := range items.All() {
		if err = callback(val); err != nil {
			return err
		}
	}

	return nil
}

// QueryRegisteredTypes gets all registered types from the meta namespace.
func QueryRegisteredTypes(ctx context.Context, st state.State) (map[resource.Type]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	// query all resources to start watching only resources that are defined for running version of talos
	resources, err := safe.StateList[*meta.ResourceDefinition](ctx, st, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, "", resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("failed to list resource definitions: %w", err)
	}

	registeredTypes := map[resource.Type]struct{}{}

	resources.ForEach(func(rd *meta.ResourceDefinition) {
		registeredTypes[rd.TypedSpec().Type] = struct{}{}
	})

	return registeredTypes, nil
}
