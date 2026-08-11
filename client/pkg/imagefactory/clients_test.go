// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func newTestState(t *testing.T) state.State {
	t.Helper()

	return state.WrapCore(namespaced.NewState(inmem.Build))
}

// failingGetState is a state.State whose Get always fails with the given error.
type failingGetState struct {
	state.State

	err error
}

func (s failingGetState) Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error) {
	return nil, s.err
}

func TestCredentials(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("factory without credentials", func(t *testing.T) {
		t.Parallel()

		username, password, err := imagefactory.Credentials(ctx, newTestState(t), "https://factory.example.org")
		require.NoError(t, err)
		require.Empty(t, username)
		require.Empty(t, password)
	})

	t.Run("stored credentials, trailing slash trimmed", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		auth := omni.NewImageFactoryAuth("https://factory.example.org")
		auth.TypedSpec().Value.Username = "user"
		auth.TypedSpec().Value.Password = "pass"
		require.NoError(t, st.Create(ctx, auth))

		username, password, err := imagefactory.Credentials(ctx, st, "https://factory.example.org/")
		require.NoError(t, err)
		require.Equal(t, "user", username)
		require.Equal(t, "pass", password)
	})

	t.Run("a denied read surfaces rather than downgrading to anonymous", func(t *testing.T) {
		t.Parallel()

		// Credentials itself reports the denial. Tolerating it is the caller's decision, taken by
		// credentialsAllowingDenied, which is what ResolveEndpoint and NewClientsFromState use.
		st := failingGetState{State: newTestState(t), err: status.Error(codes.PermissionDenied, "nope")}

		_, _, err := imagefactory.Credentials(ctx, st, "https://factory.example.org")
		require.ErrorContains(t, err, "nope")
	})

	t.Run("any other lookup failure surfaces", func(t *testing.T) {
		t.Parallel()

		st := failingGetState{State: newTestState(t), err: errors.New("state is unavailable")}

		_, _, err := imagefactory.Credentials(ctx, st, "https://factory.example.org")
		require.ErrorContains(t, err, "state is unavailable")
	})
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

// TestNewClientsFromStateDeniedCredentials covers the servers the infra provider's direct client exists for:
// an Omni that predates ImageFactoryAuth, or one whose role cannot read it, denies the read. Building the
// client set has to survive that, since a factory serving assets anonymously needs no credentials, and one
// that does need them answers with a 401 of its own.
func TestNewClientsFromStateDeniedCredentials(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	config := omni.NewFeaturesConfig(omni.FeaturesConfigID)
	config.TypedSpec().Value.ImageFactoryBaseUrl = "https://factory.example.org"
	config.TypedSpec().Value.SecondaryImageFactoryBaseUrl = "https://secondary.example.org"
	require.NoError(t, st.Create(ctx, config))

	clients, err := imagefactory.NewClientsFromState(ctx, deniedAuthState{State: st})
	require.NoError(t, err)
	require.Equal(t, "https://factory.example.org", clients.Primary().URL())

	secondary, ok := clients.Secondary()
	require.True(t, ok)
	require.Equal(t, "https://secondary.example.org", secondary.URL())
}

// TestNewClientsFromStateDefaultsBaseURL pins the fallback to the public factory. An Omni that reports no
// base URL would otherwise produce a client pointed at the empty string.
func TestNewClientsFromStateDefaultsBaseURL(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	require.NoError(t, st.Create(ctx, omni.NewFeaturesConfig(omni.FeaturesConfigID)))

	clients, err := imagefactory.NewClientsFromState(ctx, st)
	require.NoError(t, err)
	require.Equal(t, constants.ImageFactoryBaseURL, clients.Primary().URL())

	_, ok := clients.Secondary()
	require.False(t, ok)
}
