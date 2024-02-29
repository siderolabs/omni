// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package image contains the container image helpers.
package image

import (
	"fmt"

	"github.com/containers/image/docker/reference"
)

// GetTag returns image tag if it's available.
func GetTag(imageRef string) (string, error) {
	ref, err := reference.Parse(imageRef)
	if err != nil {
		return "", err
	}

	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag(), nil
	}

	return "", fmt.Errorf("image reference %q doesn't have a tag", imageRef)
}
