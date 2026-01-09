// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// GenerateFilename gets the final part of the image factory URL.
func (media *InstallationMediaSpec) GenerateFilename(legacy, secureBoot, withExtension bool) string {
	var builder strings.Builder

	// SBC handling
	if media.Overlay != "" {
		if legacy {
			builder.WriteString(constants.PlatformMetal + "-" + media.Profile)
		} else {
			builder.WriteString(constants.PlatformMetal)
		}
	} else {
		builder.WriteString(media.Profile)
	}

	builder.WriteString("-" + media.Architecture)

	if secureBoot {
		builder.WriteString("-secureboot")
	}

	if withExtension {
		builder.WriteString("." + media.Extension)
	}

	return builder.String()
}
