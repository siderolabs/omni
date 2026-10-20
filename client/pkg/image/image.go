// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package image contains the container image helpers.
package image

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

// GetTag returns image tag if it's available.
func GetTag(imageRef string) (string, error) {
	if _, err := name.ParseReference(imageRef); err != nil {
		return "", err
	}

	// ParseReference drops the tag of a "repo:tag@digest" reference
	repoWithTag, _, _ := strings.Cut(imageRef, "@")

	tag, err := name.NewTag(repoWithTag, name.WithDefaultTag(""))
	if err != nil {
		return "", err
	}

	if tag.TagStr() == "" {
		return "", fmt.Errorf("image reference %q doesn't have a tag", imageRef)
	}

	return tag.TagStr(), nil
}
