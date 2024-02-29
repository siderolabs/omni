// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package omnictl ...
package omnictl

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

func resolveResourceType(ctx context.Context, st state.State, resourceType string) (*meta.ResourceDefinition, error) {
	rds, err := safe.StateListAll[*meta.ResourceDefinition](ctx, st)
	if err != nil {
		return nil, err
	}

	var matched []*meta.ResourceDefinition

	for it := rds.Iterator(); it.Next(); {
		if strings.EqualFold(it.Value().Metadata().ID(), resourceType) {
			matched = append(matched, it.Value())

			continue
		}

		spec := it.Value().TypedSpec()

		for _, alias := range spec.AllAliases {
			if strings.EqualFold(alias, resourceType) {
				matched = append(matched, it.Value())

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
