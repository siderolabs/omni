// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package registry provides container registry scanning utils.
package registry

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// UpgradeCandidates fetches release tags from the Github.
func UpgradeCandidates(ctx context.Context, source string) ([]string, error) {
	repo, err := name.NewRepository(source)
	if err != nil {
		return nil, err
	}

	tags, err := remote.List(repo, remote.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	return tags, nil
}
