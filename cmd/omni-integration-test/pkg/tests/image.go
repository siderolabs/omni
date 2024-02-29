// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	clientconsts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertSomeImagesAreDownloadable verifies generated image download.
func AssertSomeImagesAreDownloadable(testCtx context.Context, client *client.Client, signer HTTPRequestSignerFunc, httpEndpoint string) TestFunc {
	st := client.Omni().State()

	return func(t *testing.T) {
		t.Parallel()

		media, err := safe.StateListAll[*omni.InstallationMedia](testCtx, st)
		require.NoError(t, err)

		var images []*omni.InstallationMedia

		for it := media.Iterator(); it.Next(); {
			spec := it.Value().TypedSpec().Value

			switch {
			case spec.Profile == constants.BoardRPiGeneric:
				fallthrough
			case spec.Profile == "aws":
				fallthrough
			case spec.Profile == "iso":
				images = append(images, it.Value())
			}
		}

		require.Greater(t, len(images), 2)

		for _, image := range images {
			t.Run(image.Metadata().ID(), func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(testCtx, time.Minute*5)
				defer cancel()

				u, err := url.Parse(httpEndpoint)
				require.NoError(t, err)

				schematic, err := client.Management().CreateSchematic(ctx, &management.CreateSchematicRequest{})
				require.NoError(t, err)

				u.Path, err = url.JoinPath(u.Path, "image", schematic.SchematicId, clientconsts.DefaultTalosVersion, image.Metadata().ID())
				require.NoError(t, err)

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
				require.NoError(t, err)

				require.NoError(t, signer(ctx, req))

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				require.Equal(t, http.StatusOK, resp.StatusCode)

				n, err := io.Copy(io.Discard, resp.Body)
				require.NoError(t, err)

				require.Greater(t, n, int64(1024*1024))

				require.NoError(t, resp.Body.Close())
			})
		}
	}
}
