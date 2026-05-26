// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package registry provides container registry scanning utils.
package registry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// UpgradeCandidates fetches all tags from the given container registry source and returns them as a list of strings.
func UpgradeCandidates(ctx context.Context, source string) ([]string, error) {
	repo, err := name.NewRepository(source)
	if err != nil {
		return nil, err
	}

	tags, err := remote.List(
		repo,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(
			authn.NewMultiKeychain(
				authn.DefaultKeychain,
				github.Keychain,
				google.Keychain,
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("fetching tags from registry %s: %w", repo.String(), err)
	}

	return tags, nil
}
