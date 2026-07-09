// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func newTestState(t *testing.T) state.State {
	t.Helper()

	return state.WrapCore(namespaced.NewState(inmem.Build))
}

func TestClientURLIsCanonical(t *testing.T) {
	t.Parallel()

	for _, configured := range []string{
		"https://factory.example.org",
		"https://factory.example.org/",
		"https://factory.example.org///",
	} {
		client, err := imagefactory.NewClient(configured, "", "")
		require.NoError(t, err)

		require.Equal(t, "https://factory.example.org", client.URL(), "configured as %q", configured)
		require.Equal(t, "factory.example.org", client.Host(), "configured as %q", configured)
	}
}

func TestClientsForURL(t *testing.T) {
	t.Parallel()

	// The primary is configured with a trailing slash, the secondary without: both forms must resolve.
	primary, err := imagefactory.NewClient("https://factory.example.org/", "", "")
	require.NoError(t, err)

	secondary, err := imagefactory.NewClient("https://secondary.example.org", "", "")
	require.NoError(t, err)

	clients := imagefactory.NewClients(newTestState(t), primary)
	clients.SetSecondary(secondary)

	for _, tt := range []struct {
		expected imagefactory.FactoryClient
		name     string
		url      string
	}{
		{name: "primary, no trailing slash", url: "https://factory.example.org", expected: primary},
		{name: "primary, trailing slash", url: "https://factory.example.org/", expected: primary},
		{name: "secondary, no trailing slash", url: "https://secondary.example.org", expected: secondary},
		{name: "secondary, trailing slash", url: "https://secondary.example.org/", expected: secondary},
		{name: "unconfigured factory", url: "https://other.example.org", expected: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expected == nil {
				require.Nil(t, clients.ForURL(tt.url))

				return
			}

			require.Same(t, tt.expected, clients.ForURL(tt.url))
		})
	}
}

func TestClientsForTalosVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	primary, err := imagefactory.NewClient("https://factory.example.org", "", "")
	require.NoError(t, err)

	secondary, err := imagefactory.NewClient("https://secondary.example.org", "", "")
	require.NoError(t, err)

	st := newTestState(t)

	// A version recorded with a trailing slash still has to route to the secondary factory.
	secondaryOnly := omni.NewTalosVersion("1.13.0")
	secondaryOnly.TypedSpec().Value.ImageFactoryUrl = "https://secondary.example.org/"
	require.NoError(t, st.Create(ctx, secondaryOnly))

	primaryVersion := omni.NewTalosVersion("1.14.0")
	primaryVersion.TypedSpec().Value.ImageFactoryUrl = "https://factory.example.org"
	require.NoError(t, st.Create(ctx, primaryVersion))

	unrouted := omni.NewTalosVersion("1.12.0")
	require.NoError(t, st.Create(ctx, unrouted))

	clients := imagefactory.NewClients(st, primary)
	clients.SetSecondary(secondary)

	for _, tt := range []struct {
		expected imagefactory.FactoryClient
		name     string
		version  string
	}{
		{name: "secondary-only version recorded with a trailing slash", version: "1.13.0", expected: secondary},
		{name: "strips the v prefix", version: "v1.13.0", expected: secondary},
		{name: "primary version", version: "1.14.0", expected: primary},
		{name: "version without a recorded factory falls back to the primary", version: "1.12.0", expected: primary},
		{name: "unknown version falls back to the primary", version: "1.11.0", expected: primary},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := clients.ForTalosVersion(ctx, tt.version)
			require.NoError(t, err)
			require.Same(t, tt.expected, got)
		})
	}
}
