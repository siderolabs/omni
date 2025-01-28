// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package factory_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/factory"
)

func TestParseRequest(t *testing.T) {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	specs := map[string]*specs.InstallationMediaSpec{
		"iso-metal-arm64": {
			Name:           "ISO",
			Architecture:   "arm64",
			Profile:        "metal",
			ContentType:    "application/x-iso",
			DestFilePrefix: "omni-metal-arm64",
			Extension:      "iso",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	for id, spec := range specs {
		res := omni.NewInstallationMedia(resources.EphemeralNamespace, id)
		res.TypedSpec().Value = spec

		require.NoError(t, state.Create(ctx, res))
	}

	for _, tt := range []struct {
		expectedParams *factory.ProxyParams
		name           string
		incomingURL    string
		shouldFail     bool
	}{
		{
			name:        "with secureboot",
			incomingURL: "/image/schematic/1.6.0/iso-metal-arm64?secureboot=true",
			expectedParams: &factory.ProxyParams{
				ProxyURL:            "https://factory.talos.dev/image/schematic/1.6.0/metal-arm64-secureboot.iso",
				ContentType:         "application/x-iso",
				DestinationFilename: "omni-metal-arm64-1.6.0-secureboot.iso",
			},
		},
		{
			name:        "without secureboot",
			incomingURL: "/image/schematic/1.6.0/iso-metal-arm64",
			expectedParams: &factory.ProxyParams{
				ProxyURL:            "https://factory.talos.dev/image/schematic/1.6.0/metal-arm64.iso",
				ContentType:         "application/x-iso",
				DestinationFilename: "omni-metal-arm64-1.6.0.iso",
			},
		},
		{
			name:        "not found: incomplete path",
			incomingURL: "/image/schematic/",
			shouldFail:  true,
		},
		{
			name:        "not found: no /image prefix",
			incomingURL: "/unknown",
			shouldFail:  true,
		},
		{
			name:        "not found: no such image",
			incomingURL: "/image/schematic/1.6.0/the-image",
			shouldFail:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			request, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://localhost"+tt.incomingURL, nil)
			require.NoError(err)

			params, err := factory.ParseRequest(request, state)
			if tt.shouldFail {
				require.Error(err)

				return
			}

			require.EqualValues(tt.expectedParams, params)
		})
	}
}
