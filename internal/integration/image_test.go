// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
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

		for val := range media.All() {
			spec := val.TypedSpec().Value

			switch spec.Profile {
			case "aws":
				fallthrough
			case "iso":
				images = append(images, val)
			}

			if spec.Overlay == "rpi_generic" {
				images = append(images, val)
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

				schematic, err := client.Management().CreateSchematic(ctx, &management.CreateSchematicRequest{
					MediaId:      image.Metadata().ID(),
					TalosVersion: clientconsts.DefaultTalosVersion,
				})
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
