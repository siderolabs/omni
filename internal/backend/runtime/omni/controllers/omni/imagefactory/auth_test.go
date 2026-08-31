// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:unparam
package imagefactory_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/pkg/config"
)

const testFactoryURL = "https://factory.example.com"

// testRegistries builds a configuration with a single primary factory at testFactoryURL, given basic
// auth when a username is passed.
func testRegistries(username, password string) *config.Registries {
	primary := config.Factory{}
	primary.SetUrl(testFactoryURL)

	if username != "" {
		primary.SetUsername(username)
		primary.SetPassword(password)
	}

	return &config.Registries{Factories: config.Factories{Primary: primary}}
}

// registerImageFactoryAuthController registers the controller under test.
func registerImageFactoryAuthController(t *testing.T, tc testutils.TestContext, registries *config.Registries) {
	require.NoError(t, tc.Runtime.RegisterController(imagefactory.NewAuthController(registries)))
}

// createImageFactoryAuth seeds a resource that looks like one the controller wrote, since the
// controller only modifies resources it owns.
func createImageFactoryAuth(ctx context.Context, t *testing.T, st state.State, factoryURL, username string) {
	auth := omni.NewImageFactoryAuth(factoryURL)
	auth.TypedSpec().Value.Username = username

	require.NoError(t, st.Create(ctx, auth, state.WithCreateOwner(imagefactory.AuthControllerName)))
}

// TestImageFactoryAuthBasicAuthOnly covers a factory configured with basic auth: the controller
// takes over what the Omni startup path used to write.
func TestImageFactoryAuthBasicAuthOnly(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Equal("factory-user", spec.GetUsername())
					assert.Equal("factory-pass", spec.GetPassword())
				},
			)
		},
	)
}

// TestImageFactoryAuthPrune covers a factory that no longer authenticates Omni at all.
func TestImageFactoryAuthPrune(t *testing.T) {
	t.Parallel()

	const retiredURL = "https://retired.example.com"

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryAuth(ctx, t, tc.State, retiredURL, "retired-user")

			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertNoResource[*omni.ImageFactoryAuth](ctx, t, tc.State, retiredURL)
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(*omni.ImageFactoryAuth, *assert.Assertions) {},
			)
		},
	)
}

// TestImageFactoryAuthNoFurtherWork covers that the controller reconciles once and then stops
// waking up rather than idling on a timer forever, since the factories come from static
// configuration and nothing on the resources expires.
func TestImageFactoryAuthNoFurtherWork(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()

				auth, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)
				require.Equal(t, "factory-user", auth.TypedSpec().Value.GetUsername())

				// Clear the username behind the controller's back. A write of identical content does
				// not bump the resource version, so the version alone cannot show whether the
				// controller ran; a reconciliation would put the configured username back, and that
				// is observable.
				auth.TypedSpec().Value.Username = ""
				require.NoError(t, tc.State.Update(ctx, auth, state.WithUpdateOwner(imagefactory.AuthControllerName)))

				// Well past any interval the controller might have scheduled.
				time.Sleep(72 * time.Hour)
				synctest.Wait()

				auth, err = safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)

				assert.Empty(t, auth.TypedSpec().Value.GetUsername(), "the controller reconciled again despite having nothing to do")
			},
		)
	})
}

// TestImageFactoryAuthPruneWaitsForFinalizer covers removing the credentials of an unconfigured
// factory while another controller still holds a finalizer on them: the resource has to be torn down
// and left in place until the finalizer goes, rather than destroyed outright.
func TestImageFactoryAuthPruneWaitsForFinalizer(t *testing.T) {
	t.Parallel()

	const (
		retiredURL = "https://retired.example.com"
		finalizer  = "SomeOtherController"
	)

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(ctx context.Context, tc testutils.TestContext) {
				retired := omni.NewImageFactoryAuth(retiredURL)
				retired.TypedSpec().Value.Username = "retired-user"
				retired.Metadata().Finalizers().Add(finalizer)

				require.NoError(t, tc.State.Create(ctx, retired, state.WithCreateOwner(imagefactory.AuthControllerName)))

				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()

				// Torn down, but still present: destroying it now would fail on the finalizer.
				retired, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, retiredURL)
				require.NoError(t, err)
				assert.Equal(t, resource.PhaseTearingDown, retired.Metadata().Phase())

				require.NoError(t, tc.State.RemoveFinalizer(ctx, retired.Metadata(), finalizer))

				// The controller comes back for the unfinished teardown on its own.
				time.Sleep(imagefactory.DefaultAuthRetryInterval + time.Second)
				synctest.Wait()

				_, err = safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, retiredURL)
				assert.Truef(t, state.IsNotFoundError(err), "credentials should have been destroyed, got err: %v", err)
			},
		)
	})
}

// TestImageFactoryAuthPruneContinuesPastFailure covers one stale factory the controller cannot touch
// — credentials the ownership migration missed, so another owner holds them — alongside others it
// can. The unreachable resource must not strand the removal of the rest.
func TestImageFactoryAuthPruneContinuesPastFailure(t *testing.T) {
	t.Parallel()

	// The unreachable resource is flanked by reachable ones so that the assertion holds whichever
	// order the list comes back in: one stale resource is always removed after the failure.
	const (
		retiredBeforeURL = "https://a-retired.example.com"
		unownedURL       = "https://b-unowned.example.com"
		retiredAfterURL  = "https://c-retired.example.com"
	)

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryAuth(ctx, t, tc.State, retiredBeforeURL, "retired-user")
			createImageFactoryAuth(ctx, t, tc.State, retiredAfterURL, "retired-user")

			// Owned by somebody else, so the controller's teardown of it fails outright.
			unowned := omni.NewImageFactoryAuth(unownedURL)
			require.NoError(t, tc.State.Create(ctx, unowned, state.WithCreateOwner("SomeOtherController")))

			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			// Every reachable one goes, and the configured factory is still written.
			rtestutils.AssertNoResource[*omni.ImageFactoryAuth](ctx, t, tc.State, retiredBeforeURL)
			rtestutils.AssertNoResource[*omni.ImageFactoryAuth](ctx, t, tc.State, retiredAfterURL)
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Equal("factory-user", res.TypedSpec().Value.GetUsername())
				},
			)

			// The one it cannot touch is left alone rather than half-torn-down.
			unowned, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, unownedURL)
			require.NoError(t, err)
			assert.Equal(t, resource.PhaseRunning, unowned.Metadata().Phase())
		},
	)
}
