// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import (
	"net/http"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
)

// Exported for testing: the mapping from an image and its download parameters onto the boot asset the
// image factory serves, so it can be checked without an Omni to answer the RPC.
func MediaSpec(image ImageInfo, params Params) imagefactory.MediaSpec {
	return mediaSpec(image, params)
}

// Exported for testing: the download of the installation media to a file, so the handling of
// incomplete downloads can be checked against a local HTTP server.
func DownloadToFile(req *http.Request, dest string) error {
	return downloadToFile(req, dest)
}

// Exported for testing: the write of a response body to its destination through the temporary file,
// so the cleanup on a failed move into place can be checked.
func DownloadResponseTo(dest string, resp *http.Response) error {
	return downloadResponseTo(dest, resp)
}
