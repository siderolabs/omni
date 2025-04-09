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

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
)

// Destroy tears down and destroys resources of the specified type.
//
//nolint:gocognit,gocyclo,cyclop
func Destroy(ctx context.Context, st state.State, resNS, resType, selector string, all bool, ids []resource.ID) error {
	rd, err := ResolveType(ctx, st, resType)
	if err != nil {
		return err
	}

	if resNS == "" {
		resNS = rd.TypedSpec().DefaultNamespace
	}

	listMD := resource.NewMetadata(resNS, rd.TypedSpec().Type, "", resource.VersionUndefined)

	var (
		resourceIDs   []resource.ID
		useWatchKind  bool
		watchKindOpts []state.WatchKindOption
	)

	if len(ids) > 0 {
		resourceIDs = slices.Clone(ids)
		useWatchKind = len(resourceIDs) > 10
	} else {
		useWatchKind = true

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
			watchKindOpts = append(watchKindOpts, state.WatchWithLabelQuery(labelQuery))
		}

		if resourceIDs, err = getResourceIDs(ctx, st, listMD, listOpts...); err != nil {
			return err
		}
	}

	watchCh := make(chan state.Event)

	if useWatchKind {
		if err = st.WatchKind(ctx, listMD, watchCh, watchKindOpts...); err != nil {
			return err
		}
	} else {
		for _, resourceID := range resourceIDs {
			err = st.Watch(ctx, resource.NewMetadata(resNS, rd.TypedSpec().Type, resourceID, resource.VersionUndefined), watchCh)
			if err != nil {
				return err
			}
		}
	}

	resourceIDsLeft := map[resource.ID]struct{}{}

	// teardown all resources
	for _, resourceID := range resourceIDs {
		destroyReady, teardownErr := st.Teardown(ctx, resource.NewMetadata(resNS, rd.TypedSpec().Type, resourceID, resource.VersionUndefined))
		if teardownErr != nil {
			return teardownErr
		}

		fmt.Fprintf(os.Stderr, "torn down %s %s\n", rd.TypedSpec().Type, resourceID)

		if destroyReady {
			if err = st.Destroy(ctx, resource.NewMetadata(resNS, rd.TypedSpec().Type, resourceID, resource.VersionUndefined)); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "destroyed %s %s\n", rd.TypedSpec().Type, resourceID)

			continue
		}

		resourceIDsLeft[resourceID] = struct{}{}
	}

	// until some resources are not deleted yet...
	for len(resourceIDsLeft) > 0 {
		var event state.Event

		select {
		case <-ctx.Done():
			return ctx.Err()
		case event = <-watchCh:
		}

		switch event.Type {
		case state.Destroyed:
			delete(resourceIDsLeft, event.Resource.Metadata().ID())
		case state.Created, state.Updated:
			if _, ours := resourceIDsLeft[event.Resource.Metadata().ID()]; !ours {
				continue
			}

			if event.Resource.Metadata().Phase() == resource.PhaseTearingDown && event.Resource.Metadata().Finalizers().Empty() {
				if err = st.Destroy(ctx, event.Resource.Metadata()); err != nil && !state.IsNotFoundError(err) {
					return err
				}

				fmt.Fprintf(os.Stderr, "destroyed %s %s\n", rd.TypedSpec().Type, event.Resource.Metadata().ID())
			}
		case state.Bootstrapped, state.Noop:
			// ignore
		case state.Errored:
			return fmt.Errorf("error watching for resource deletion: %w", event.Error)
		}
	}

	return nil
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
