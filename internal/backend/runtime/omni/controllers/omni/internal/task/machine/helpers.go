// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
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

	iter := items.Iterator()

	for iter.Next() {
		if err = callback(iter.Value()); err != nil {
			return err
		}
	}

	return nil
}
