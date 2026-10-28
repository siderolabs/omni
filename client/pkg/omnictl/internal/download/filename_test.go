// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

func TestDownloadImageToFilename(t *testing.T) {
	t.Parallel()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("image contents"))
		assert.NoError(t, err)
	}))
	t.Cleanup(httpServer.Close)

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	management.RegisterManagementServiceServer(server, &filenameManagementServer{url: httpServer.URL})

	go func() {
		assert.NoError(t, server.Serve(listener))
	}()

	c, err := client.New("grpc://" + listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, c.Close()) })

	for _, tt := range []struct {
		platform  string
		extension string
		output    string
		filename  string
	}{
		{platform: "metal", extension: "iso", filename: "my-preset-1.14.5-aaaaaa.iso"},
		{platform: "metal", extension: "raw.xz", filename: "my-preset-1.14.5-aaaaaa.raw.xz"},
		{platform: "metal", extension: "qcow2", filename: "my-preset-1.14.5-aaaaaa.qcow2"},
		{platform: "azure", extension: "vhd.xz", filename: "my-preset-1.14.5-aaaaaa.vhd.xz"},
		{platform: "gcp", extension: "raw.tar.gz", filename: "my-preset-1.14.5-aaaaaa.raw.tar.gz"},
		{platform: "metal", extension: "raw.xz", output: "custom.img", filename: "custom.img"},
	} {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			output := dir
			if tt.output != "" {
				output = filepath.Join(dir, tt.output)
			}

			err := download.DownloadImageTo(t.Context(), c, download.ImageInfo{
				DestFilePrefix: "my-preset",
				Profile:        tt.platform,
				Architecture:   "amd64",
				Extension:      tt.extension,
			}, download.Params{Output: output, TalosVersion: "1.14.5"})
			require.NoError(t, err)

			contents, err := os.ReadFile(filepath.Join(dir, tt.filename))
			require.NoError(t, err)
			require.Equal(t, "image contents", string(contents))
		})
	}
}

type filenameManagementServer struct {
	management.UnimplementedManagementServiceServer

	url string
}

func (*filenameManagementServer) CreateSchematic(context.Context, *management.CreateSchematicRequest) (*management.CreateSchematicResponse, error) {
	return &management.CreateSchematicResponse{SchematicId: strings.Repeat("a", 64)}, nil
}

func (s *filenameManagementServer) GetInstallationMediaURL(context.Context, *management.InstallationMediaURLRequest) (*management.InstallationMediaURLResponse, error) {
	return &management.InstallationMediaURLResponse{Url: s.url}, nil
}
