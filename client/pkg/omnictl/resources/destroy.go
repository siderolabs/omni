// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resources

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
)

// destroyConcurrency caps how many TeardownAndDestroy calls run in parallel.
const destroyConcurrency = 8

// Destroy tears down and destroys resources of the specified type.
//
// Each resource is deleted via [state.State.TeardownAndDestroy], which over the
// gRPC adapter is a single atomic call that handles teardown, the wait for
// finalizers, and destroy server-side. This means the caller does not need read
// access to the resource, only destroy access.
//
// When --all or --selector is used, the resource type must support List (read
// access) so the IDs can be enumerated. Per-ID deletion has no such requirement.
func Destroy(ctx context.Context, st state.State, resNS, resType, selector string, all bool, ids []resource.ID) error {
	rd, err := ResolveType(ctx, st, resType)
	if err != nil {
		return err
	}

	if resNS == "" {
		resNS = rd.TypedSpec().DefaultNamespace
	}

	var resourceIDs []resource.ID

	if len(ids) > 0 {
		resourceIDs = slices.Clone(ids)
	} else {
		if !all && selector == "" {
			return fmt.Errorf("either resource ID or one of all or selector must be specified")
		}

		var listOpts []state.ListOption

		if selector != "" {
			var labelQuery resource.LabelQueryOption

			labelQuery, err = labelQueryForSelector(selector)
			if err != nil {
				return err
			}

			listOpts = append(listOpts, state.WithLabelQuery(labelQuery))
		}

		listMD := resource.NewMetadata(resNS, rd.TypedSpec().Type, "", resource.VersionUndefined)

		if resourceIDs, err = getResourceIDs(ctx, st, listMD, listOpts...); err != nil {
			return err
		}
	}

	eg, gctx := errgroup.WithContext(ctx)
	eg.SetLimit(destroyConcurrency)

	for _, resourceID := range resourceIDs {
		eg.Go(func() error {
			md := resource.NewMetadata(resNS, rd.TypedSpec().Type, resourceID, resource.VersionUndefined)

			if err := st.TeardownAndDestroy(gctx, md); err != nil {
				if state.IsNotFoundError(err) {
					return nil
				}

				return err
			}

			fmt.Fprintf(os.Stderr, "destroyed %s %s\n", rd.TypedSpec().Type, resourceID)

			return nil
		})
	}

	return eg.Wait()
}

// ResolveType resolves the resource type to a resource definition.
func ResolveType(ctx context.Context, st state.State, resourceType string) (*meta.ResourceDefinition, error) {
	rds, err := safe.StateListAll[*meta.ResourceDefinition](ctx, st)
	if err != nil {
		return nil, err
	}

	var matched []*meta.ResourceDefinition

	for val := range rds.All() {
		if strings.EqualFold(val.Metadata().ID(), resourceType) {
			matched = append(matched, val)

			continue
		}

		spec := val.TypedSpec()

		for _, alias := range spec.AllAliases {
			if strings.EqualFold(alias, resourceType) {
				matched = append(matched, val)

				break
			}
		}
	}

	switch {
	case len(matched) == 1:
		return matched[0], nil
	case len(matched) > 1:
		matchedTypes := make([]string, 0, len(matched))

		for _, rd := range matched {
			matchedTypes = append(matchedTypes, rd.Metadata().ID())
		}

		return nil, fmt.Errorf("resource type %q is ambiguous: %v", resourceType, matchedTypes)
	default:
		return nil, fmt.Errorf("resource %q is not registered", resourceType)
	}
}

func labelQueryForSelector(selector string) (resource.LabelQueryOption, error) {
	query, err := labels.ParseQuery(selector)
	if err != nil {
		return nil, err
	}

	return resource.RawLabelQuery(*query), nil
}

func getResourceIDs(ctx context.Context, st state.State, listMD resource.Metadata, listOpts ...state.ListOption) ([]resource.ID, error) {
	list, err := st.List(ctx, listMD, listOpts...)
	if err != nil {
		return nil, err
	}

	resourceIDs := make([]resource.ID, 0, len(list.Items))

	for _, item := range list.Items {
		resourceIDs = append(resourceIDs, item.Metadata().ID())
	}

	return resourceIDs, nil
}
