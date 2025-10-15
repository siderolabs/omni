// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package image contains the container image helpers.
package image

import (
	"fmt"

	"github.com/containers/image/v5/docker/reference"
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
