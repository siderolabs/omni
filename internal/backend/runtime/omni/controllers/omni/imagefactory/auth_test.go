// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/pkg/auth/oauth2"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/imagefactory/tokens"
)

const (
	testFactoryURL      = "https://factory.example.com"
	testFactoryClientID = "client-id"
	testFactoryAudience = "https://image-factory.example.com"

	// testTokenLifetime is the lifetime an authorization server gives an access token by default, so
	// the controller is exercised on the schedule it actually runs on.
	testTokenLifetime = 24 * time.Hour
)

// fakeIssuer hands out tokens without talking to an authorization server, numbering them so a test
// can tell one issued token from the next.
type fakeIssuer struct { //nolint:govet // grouped by role rather than by alignment
	clientID string
	audience string
	lifetime time.Duration

	mu    sync.Mutex
	count int
	err   error
}

func newFakeIssuer() *fakeIssuer {
	return &fakeIssuer{clientID: testFactoryClientID, audience: testFactoryAudience, lifetime: testTokenLifetime}
}

func (i *fakeIssuer) IssueToken(context.Context) (oauth2.AccessToken, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.err != nil {
		return oauth2.AccessToken{}, i.err
	}

	i.count++

	issuedAt := time.Now()

	return oauth2.AccessToken{
		Token:     tokenValue(i.count),
		TokenType: "Bearer",
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(i.lifetime),
	}, nil
}

func (i *fakeIssuer) ClientID() string { return i.clientID }

func (i *fakeIssuer) Audience() string { return i.audience }

func (i *fakeIssuer) setError(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.err = err
}

func tokenValue(n int) string {
	return "token-" + strconv.Itoa(n)
}

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

// registerImageFactoryAuthController registers the controller under test with the given issuer for
// testFactoryURL. A nil issuer stands for a factory that has no OAuth2 client credentials.
func registerImageFactoryAuthController(t *testing.T, tc testutils.TestContext, registries *config.Registries, issuer *fakeIssuer) {
	issuers := map[string]tokens.Issuer{}
	if issuer != nil {
		issuers[testFactoryURL] = issuer
	}

	require.NoError(t, tc.Runtime.RegisterController(
		imagefactory.NewAuthController(registries, tokens.NewIssuersFromMap(issuers)),
	))
}

// createImageFactoryAuth seeds a resource that looks like one the controller wrote, since the
// controller only modifies resources it owns.
func createImageFactoryAuth(ctx context.Context, t *testing.T, st state.State, factoryURL, username string, token *specs.ImageFactoryAuthSpec_AccessToken) {
	auth := omni.NewImageFactoryAuth(factoryURL)
	auth.TypedSpec().Value.Username = username
	auth.TypedSpec().Value.Token = token

	require.NoError(t, st.Create(ctx, auth, state.WithCreateOwner(imagefactory.AuthControllerName)))
}

// assertRecordedToken requires that the factory's recorded access token is exactly the expected one.
func assertRecordedToken(ctx context.Context, t *testing.T, st state.State, expected string) {
	t.Helper()

	auth, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, st, testFactoryURL)
	require.NoError(t, err)
	assert.Equal(t, expected, auth.TypedSpec().Value.GetToken().GetToken())
}

// TestImageFactoryAuthBasicAuthOnly covers a factory configured with basic auth and no OAuth2
// client: the controller takes over what the Omni startup path used to write, and issues no token.
func TestImageFactoryAuthBasicAuthOnly(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Equal("factory-user", spec.GetUsername())
					assert.Equal("factory-pass", spec.GetPassword())
					assert.Nil(spec.GetToken())
				},
			)
		},
	)
}

// TestImageFactoryAuthTokenOnly covers a factory that authenticates Omni purely with an access
// token.
func TestImageFactoryAuthTokenOnly(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registerImageFactoryAuthController(t, tc, testRegistries("", ""), newFakeIssuer())
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Empty(spec.GetUsername())
					assert.Empty(spec.GetPassword())
					assert.Equal(tokenValue(1), spec.GetToken().GetToken())
					assert.Equal("Bearer", spec.GetToken().GetTokenType())
					assert.Equal(testFactoryClientID, spec.GetToken().GetClientId())
					assert.Equal(testFactoryAudience, spec.GetToken().GetAudience())
					assert.WithinDuration(time.Now().Add(testTokenLifetime), spec.GetToken().GetExpiresAt().AsTime(), time.Minute)
				},
			)
		},
	)
}

// TestImageFactoryAuthRotate covers replacing a token once it is halfway through its lifetime. It
// runs on virtual time, so the real token lifetime can be used rather than a contrived one.
func TestImageFactoryAuthRotate(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registerImageFactoryAuthController(t, tc, testRegistries("", ""), newFakeIssuer())
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(1))

				// Not yet halfway through the lifetime: the first token stands.
				time.Sleep(testTokenLifetime/2 - time.Minute)
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(1))

				// Past the halfway point, it is replaced.
				time.Sleep(2 * time.Minute)
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(2))
			},
		)
	})
}

// TestImageFactoryAuthSchedulesFullLifetimeAfterIssuing covers the wake-up scheduled after a token is
// issued: it must be derived from the new token's lifetime, not from the one it replaced. Getting
// this wrong reports a fresh token as unusable and brings the controller straight back.
func TestImageFactoryAuthSchedulesFullLifetimeAfterIssuing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), newFakeIssuer())
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(1))

				// Clear the username behind the controller's back; any further pass restores it.
				auth, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)

				auth.TypedSpec().Value.Username = ""
				require.NoError(t, tc.State.Update(ctx, auth, state.WithUpdateOwner(imagefactory.AuthControllerName)))

				// A token good for a full day must not bring the controller back on the retry interval.
				time.Sleep(2 * imagefactory.DefaultAuthRetryInterval)
				synctest.Wait()

				auth, err = safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)
				assert.Empty(t, auth.TypedSpec().Value.GetUsername(), "the controller retried as though the fresh token were unusable")

				// It does come back at the halfway point, on the new token's own schedule.
				time.Sleep(testTokenLifetime / 2)
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(2))
			},
		)
	})
}

// TestImageFactoryAuthReissueOnCredentialChange covers a factory whose OAuth2 client credentials
// were reconfigured: the token issued to the previous client must not be left in place until it expires.
func TestImageFactoryAuthReissueOnCredentialChange(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryAuth(ctx, t, tc.State, testFactoryURL, "", &specs.ImageFactoryAuthSpec_AccessToken{
				Token:     "stale-token",
				TokenType: "Bearer",
				IssuedAt:  timestamppb.New(time.Now()),
				ExpiresAt: timestamppb.New(time.Now().Add(testTokenLifetime)),
				ClientId:  "previous-client-id",
				Audience:  testFactoryAudience,
			})

			registerImageFactoryAuthController(t, tc, testRegistries("", ""), newFakeIssuer())
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Equal(tokenValue(1), res.TypedSpec().Value.GetToken().GetToken())
					assert.Equal(testFactoryClientID, res.TypedSpec().Value.GetToken().GetClientId())
				},
			)
		},
	)
}

// TestImageFactoryAuthDropTokenWhenOAuth2Removed covers dropping a token whose OAuth2 client
// credentials are gone from the configuration, while the factory's basic auth stays.
func TestImageFactoryAuthDropTokenWhenOAuth2Removed(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryAuth(ctx, t, tc.State, testFactoryURL, "factory-user", &specs.ImageFactoryAuthSpec_AccessToken{
				Token:     "orphaned-token",
				IssuedAt:  timestamppb.New(time.Now()),
				ExpiresAt: timestamppb.New(time.Now().Add(testTokenLifetime)),
			})

			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Nil(res.TypedSpec().Value.GetToken())
					assert.Equal("factory-user", res.TypedSpec().Value.GetUsername())
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
			createImageFactoryAuth(ctx, t, tc.State, retiredURL, "retired-user", nil)

			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
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

// TestImageFactoryAuthNoFurtherWorkWithoutOAuth2 covers an installation where no factory has OAuth2
// client credentials: nothing expires, the factories come from static configuration, so the controller must
// reconcile once and then stop waking up rather than idling on a timer forever.
func TestImageFactoryAuthNoFurtherWorkWithoutOAuth2(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
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

				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
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
			createImageFactoryAuth(ctx, t, tc.State, retiredBeforeURL, "retired-user", nil)
			createImageFactoryAuth(ctx, t, tc.State, retiredAfterURL, "retired-user", nil)

			// Owned by somebody else, so the controller's teardown of it fails outright.
			unowned := omni.NewImageFactoryAuth(unownedURL)
			require.NoError(t, tc.State.Create(ctx, unowned, state.WithCreateOwner("SomeOtherController")))

			registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), nil)
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

// TestImageFactoryAuthBasicAuthSurvivesIssuerFailure is the reason basic auth is written before the
// token is resolved: the machines' registry configuration is built from it, so an unreachable
// authorization server must not keep it out of state. On virtual time, so the real retry interval is exercised.
func TestImageFactoryAuthBasicAuthSurvivesIssuerFailure(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		issuer := newFakeIssuer()
		issuer.setError(errors.New("the authorization server is unreachable"))

		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registerImageFactoryAuthController(t, tc, testRegistries("factory-user", "factory-pass"), issuer)
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()

				auth, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)

				assert.Equal(t, "factory-user", auth.TypedSpec().Value.GetUsername())
				assert.Nil(t, auth.TypedSpec().Value.GetToken())

				issuer.setError(nil)

				// The controller retries on its own; nothing pokes it.
				time.Sleep(imagefactory.DefaultAuthRetryInterval + time.Second)
				synctest.Wait()

				assertRecordedToken(ctx, t, tc.State, tokenValue(1))
			},
		)
	})
}
