// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import "github.com/siderolabs/omni/client/pkg/imagefactory"

// Exported for testing: the mapping from an image and its download parameters onto the boot asset the
// image factory serves, so it can be checked without an Omni to answer the RPC.
func MediaSpec(image ImageInfo, params Params) imagefactory.MediaSpec {
	return mediaSpec(image, params)
}
