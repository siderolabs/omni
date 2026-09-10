// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:unparam
package imagefactory_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/golang-jwt/jwt/v5"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/imagefactory/tokenfile"
)

// testFactoryURL is what the mock factory client reports as its URL, so that the controller finds the
// client for the configured factory.
const testFactoryURL = "https://image.factory.test"

const testOmniToken = "omni-token"

const testAccountName = "my-omni"

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

// testTokenRegistries builds a configuration with a single primary factory at testFactoryURL that Omni
// authenticates to with an API token read from a file with the given content, and the loaded token set.
func testTokenRegistries(t *testing.T, token string) (*config.Registries, *tokenfile.Set) {
	t.Helper()

	return testTokenRegistriesAt(t, filepath.Join(t.TempDir(), "token"), token)
}

// testTokenRegistriesAt is testTokenRegistries with the token file at path, for the tests that rotate it.
func testTokenRegistriesAt(t *testing.T, path, token string) (*config.Registries, *tokenfile.Set) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(token), 0o600))

	primary := config.Factory{}
	primary.SetUrl(testFactoryURL)
	primary.SetTokenFile(path)

	tokenFile, err := tokenfile.Load(testFactoryURL, path, zaptest.NewLogger(t))
	require.NoError(t, err)

	return &config.Registries{Factories: config.Factories{Primary: primary}}, tokenfile.NewSet(tokenFile)
}

// registerImageFactoryAuthController registers the controller under test with a factory client that
// issues no tokens.
func registerImageFactoryAuthController(t *testing.T, tc testutils.TestContext, registries *config.Registries) {
	registerImageFactoryAuthControllerWithFactory(t, tc, registries, nil, &testutils.ImageFactoryClientMock{})
}

func registerImageFactoryAuthControllerWithFactory(
	t *testing.T, tc testutils.TestContext, registries *config.Registries, tokens *tokenfile.Set, factory *testutils.ImageFactoryClientMock,
) {
	require.NoError(t, tc.Runtime.RegisterController(imagefactory.NewAuthController(registries, testAccountName, testutils.NewFactoryClientSet(factory), tokens)))
}

// createImageFactoryAuth seeds a resource that looks like one the controller wrote, since the
// controller only modifies resources it owns.
func createImageFactoryAuth(ctx context.Context, t *testing.T, st state.State, factoryURL, username string) {
	auth := omni.NewImageFactoryAuth(factoryURL)
	auth.TypedSpec().Value.Username = username

	require.NoError(t, st.Create(ctx, auth, state.WithCreateOwner(imagefactory.AuthControllerName)))
}

// createImageFactoryTokenAuth seeds a resource of a token factory that looks like one the controller wrote.
func createImageFactoryTokenAuth(ctx context.Context, t *testing.T, st state.State, apiToken, machineToken string) {
	auth := omni.NewImageFactoryAuth(testFactoryURL)
	auth.TypedSpec().Value.ApiToken = apiToken
	auth.TypedSpec().Value.MachineToken = machineToken

	require.NoError(t, st.Create(ctx, auth, state.WithCreateOwner(imagefactory.AuthControllerName)))
}

// machineToken builds a token with the given issue and expiry times, signed with a throwaway key: the
// controller reads the claims without verifying the signature.
func machineToken(t *testing.T, issuedAt, expiresAt time.Time) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "org",
		ID:        "jti",
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}).SignedString([]byte("test"))
	require.NoError(t, err)

	return token
}

// tokenFactory is a factory client that hands out year-long machine tokens and counts the requests.
type tokenFactory struct {
	*testutils.ImageFactoryClientMock

	last     atomic.Pointer[client.TokenCreateOptions]
	requests atomic.Int32
}

func newTokenFactory(t *testing.T, err error) *tokenFactory {
	f := &tokenFactory{ImageFactoryClientMock: &testutils.ImageFactoryClientMock{}}

	f.TokenCreateFunc = func(_ context.Context, opts client.TokenCreateOptions) (string, string, error) {
		f.requests.Add(1)
		f.last.Store(&opts)

		if err != nil {
			return "", "", err
		}

		now := time.Now()

		return "id", machineToken(t, now, now.Add(365*24*time.Hour)), nil
	}

	return f
}

// TestImageFactoryAuthBasicAuthOnly covers a factory configured with basic auth: the controller
// takes over what the Omni startup path used to write, and creates no machine token.
func TestImageFactoryAuthBasicAuthOnly(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registerImageFactoryAuthControllerWithFactory(t, tc, testRegistries("factory-user", "factory-pass"), nil, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Equal("factory-user", spec.GetUsername())
					assert.Equal("factory-pass", spec.GetPassword())
					assert.Empty(spec.GetApiToken())
					assert.Empty(spec.GetMachineToken())
				},
			)

			assert.Zero(t, factory.requests.Load(), "basic auth needs no machine token")
		},
	)
}

// TestImageFactoryAuthToken covers a factory Omni authenticates to with an API token: the controller
// creates the pull-only machine token on the factory and stores both.
func TestImageFactoryAuthToken(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registries, tokens := testTokenRegistries(t, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Empty(spec.GetUsername())
					assert.Empty(spec.GetPassword())
					assert.Equal(testOmniToken, spec.GetApiToken())
					assert.NotEmpty(spec.GetMachineToken())
					assert.NotEqual(testOmniToken, spec.GetMachineToken(), "the machines must never get Omni's own token")
				},
			)

			assert.EqualValues(t, 1, factory.requests.Load())

			request := factory.last.Load()
			require.NotNil(t, request)
			assert.Equal(t, []string{"image:read"}, request.Scopes, "the machine token pulls images and nothing else")
			assert.False(t, request.Ephemeral, "without a configured lifetime the machine token is stored: an ephemeral one lives hours, a stored one a year")
			assert.Zero(t, request.TTL, "without a configured lifetime the factory's default applies")
			assert.Equal(t, "omni-machines-my-omni-"+time.Now().UTC().Format(time.DateOnly), request.Name)
		},
	)
}

// TestImageFactoryAuthMachineTokenTTL covers a configured machine token lifetime: the token is
// requested short-lived, so the factory does not record it and it does not count against the
// token limit of the organization.
func TestImageFactoryAuthMachineTokenTTL(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			registries, tokens := testTokenRegistries(t, testOmniToken)
			registries.Factories.Primary.SetMachineTokenTTL(2 * time.Hour)

			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.NotEmpty(res.TypedSpec().Value.GetMachineToken())
				},
			)

			request := factory.last.Load()
			require.NotNil(t, request)
			assert.True(t, request.Ephemeral)
			assert.Equal(t, 2*time.Hour, request.TTL)
		},
	)
}

// TestImageFactoryAuthTokenKept covers a restart: a fresh machine token created earlier is kept rather
// than replaced, so that a restart does not eat into the factory's per-org token cap.
func TestImageFactoryAuthTokenKept(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	now := time.Now()
	existing := machineToken(t, now.Add(-time.Hour), now.Add(365*24*time.Hour))

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryTokenAuth(ctx, t, tc.State, testOmniToken, existing)

			registries, tokens := testTokenRegistries(t, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Equal(existing, res.TypedSpec().Value.GetMachineToken())
				},
			)

			assert.Zero(t, factory.requests.Load(), "a fresh machine token is kept")
		},
	)
}

// TestImageFactoryAuthTokenRenewed covers a machine token past half of its lifetime: it is
// replaced while it still works, so the machines get the new one before the old one expires.
func TestImageFactoryAuthTokenRenewed(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	now := time.Now()
	old := machineToken(t, now.Add(-300*24*time.Hour), now.Add(65*24*time.Hour))

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryTokenAuth(ctx, t, tc.State, testOmniToken, old)

			registries, tokens := testTokenRegistries(t, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.NotEmpty(res.TypedSpec().Value.GetMachineToken())
					assert.NotEqual(old, res.TypedSpec().Value.GetMachineToken())
				},
			)

			assert.EqualValues(t, 1, factory.requests.Load())
		},
	)
}

// TestImageFactoryAuthTokenRenewalTimer covers the renewal happening on its own, at half of the
// machine token's lifetime, without anything else waking the controller.
func TestImageFactoryAuthTokenRenewalTimer(t *testing.T) {
	t.Parallel()

	const lifetime = 20 * time.Minute

	synctest.Test(t, func(t *testing.T) {
		factory := &tokenFactory{ImageFactoryClientMock: &testutils.ImageFactoryClientMock{}}

		factory.TokenCreateFunc = func(_ context.Context, opts client.TokenCreateOptions) (string, string, error) {
			factory.requests.Add(1)
			factory.last.Store(&opts)

			now := time.Now()

			return "id", machineToken(t, now, now.Add(lifetime)), nil
		}

		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registries, tokens := testTokenRegistries(t, testOmniToken)
				registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()
				require.EqualValues(t, 1, factory.requests.Load())

				first, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)

				// Short of half: nothing happens.
				time.Sleep(9 * time.Minute)
				synctest.Wait()
				require.EqualValues(t, 1, factory.requests.Load())

				// Past it: the token is replaced, and the resource carries the new one.
				time.Sleep(2 * time.Minute)
				synctest.Wait()
				require.EqualValues(t, 2, factory.requests.Load())

				second, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)
				require.NotEqual(t, first.TypedSpec().Value.GetMachineToken(), second.TypedSpec().Value.GetMachineToken())
			},
		)
	})
}

// TestImageFactoryAuthTokenChanged covers Omni's own token changing: the machine token created with
// the previous one is not trusted and is replaced.
func TestImageFactoryAuthTokenChanged(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	now := time.Now()
	old := machineToken(t, now.Add(-time.Hour), now.Add(365*24*time.Hour))

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryTokenAuth(ctx, t, tc.State, "previous-omni-token", old)

			registries, tokens := testTokenRegistries(t, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Equal(testOmniToken, res.TypedSpec().Value.GetApiToken())
					assert.NotEqual(old, res.TypedSpec().Value.GetMachineToken())
				},
			)

			assert.EqualValues(t, 1, factory.requests.Load())
		},
	)
}

// TestImageFactoryAuthTokenToBasicAuth covers a factory switched from a token to basic auth: nothing of
// the token mode is left in the resource, so the machines pull with the basic auth credentials.
func TestImageFactoryAuthTokenToBasicAuth(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)

	now := time.Now()

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryTokenAuth(ctx, t, tc.State, testOmniToken, machineToken(t, now, now.Add(365*24*time.Hour)))

			registerImageFactoryAuthControllerWithFactory(t, tc, testRegistries("factory-user", "factory-pass"), nil, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					spec := res.TypedSpec().Value

					assert.Equal("factory-user", spec.GetUsername())
					assert.Equal("factory-pass", spec.GetPassword())
					assert.Empty(spec.GetApiToken())
					assert.Empty(spec.GetMachineToken())
				},
			)

			assert.Zero(t, factory.requests.Load())
		},
	)
}

// TestImageFactoryAuthTokenRenewalFailureKeepsCurrent covers a factory that is unreachable at renewal
// time: the reconcile fails and the resource is left as it is, so the machines keep the current
// token, which works until it expires.
func TestImageFactoryAuthTokenRenewalFailureKeepsCurrent(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, errors.New("factory unreachable"))

	now := time.Now()
	old := machineToken(t, now.Add(-300*24*time.Hour), now.Add(65*24*time.Hour))

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(ctx context.Context, tc testutils.TestContext) {
			createImageFactoryTokenAuth(ctx, t, tc.State, testOmniToken, old)

			registries, tokens := testTokenRegistries(t, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			// The resource is seeded, so wait for the controller to have tried before reading it back.
			require.Eventually(t, func() bool { return factory.requests.Load() >= 1 }, 10*time.Second, 10*time.Millisecond)

			rtestutils.AssertResources(
				ctx, t, tc.State, []string{testFactoryURL},
				func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
					assert.Equal(testOmniToken, res.TypedSpec().Value.GetApiToken())
					assert.Equal(old, res.TypedSpec().Value.GetMachineToken())
				},
			)
		},
	)
}

// TestImageFactoryAuthTokenCreationFailureWritesNoMachineToken covers a factory that is unreachable
// when there is no machine token yet: the resource carries Omni's token and no machine token, the
// state the machine configs refuse, so none is generated without registry credentials.
func TestImageFactoryAuthTokenCreationFailureWritesNoMachineToken(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, errors.New("factory unreachable"))

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registries, tokens := testTokenRegistries(t, testOmniToken)
				registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
			},
			func(ctx context.Context, tc testutils.TestContext) {
				time.Sleep(time.Minute)
				synctest.Wait()

				rtestutils.AssertResources(
					ctx, t, tc.State, []string{testFactoryURL},
					func(res *omni.ImageFactoryAuth, assert *assert.Assertions) {
						assert.Equal(testOmniToken, res.TypedSpec().Value.GetApiToken())
						assert.Empty(res.TypedSpec().Value.GetMachineToken())
					},
				)

				assert.GreaterOrEqual(t, factory.requests.Load(), int32(1))
			},
		)
	})
}

// TestImageFactoryAuthTokenAlreadyDue covers a machine token that is past its renewal point when it
// arrives (the factory's clock behind Omni's): it is created once, and renewing it waits for the
// next restart rather than looping.
func TestImageFactoryAuthTokenAlreadyDue(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		factory := &tokenFactory{ImageFactoryClientMock: &testutils.ImageFactoryClientMock{}}

		factory.TokenCreateFunc = func(_ context.Context, _ client.TokenCreateOptions) (string, string, error) {
			factory.requests.Add(1)

			now := time.Now()

			return "id", machineToken(t, now.Add(-2*time.Hour), now.Add(time.Hour)), nil
		}

		testutils.WithRuntime(
			t.Context(), t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				registries, tokens := testTokenRegistries(t, testOmniToken)
				registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
			},
			func(ctx context.Context, tc testutils.TestContext) {
				synctest.Wait()
				time.Sleep(time.Hour)
				synctest.Wait()

				require.EqualValues(t, 1, factory.requests.Load())

				res, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)
				require.NoError(t, err)
				require.NotEmpty(t, res.TypedSpec().Value.GetMachineToken())
			},
		)
	})
}

// TestImageFactoryAuthTokenRotated covers Omni's token file being rewritten while the controller
// runs: the controller wakes up on the change, the resource follows, and the machine token created
// with the previous token is replaced.
func TestImageFactoryAuthTokenRotated(t *testing.T) {
	t.Parallel()

	factory := newTokenFactory(t, nil)
	path := filepath.Join(t.TempDir(), "token")

	var tokens *tokenfile.Set

	testutils.WithRuntime(
		t.Context(), t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			var registries *config.Registries

			registries, tokens = testTokenRegistriesAt(t, path, testOmniToken)
			registerImageFactoryAuthControllerWithFactory(t, tc, registries, tokens, factory.ImageFactoryClientMock)
		},
		func(ctx context.Context, tc testutils.TestContext) {
			ctx, cancel := context.WithCancel(ctx)

			var eg errgroup.Group

			eg.Go(func() error { return tokens.Run(ctx) })

			defer func() {
				cancel()
				require.NoError(t, eg.Wait())
			}()

			require.Eventually(t, func() bool { return factory.requests.Load() >= 1 }, 10*time.Second, 10*time.Millisecond)

			// The file is rewritten on every check: a write landing before the watcher is registered
			// is only seen by the poll, which is a minute away.
			require.Eventually(t, func() bool {
				if os.WriteFile(path+".tmp", []byte("rotated-omni-token"), 0o600) != nil || os.Rename(path+".tmp", path) != nil {
					return false
				}

				res, err := safe.StateGetByID[*omni.ImageFactoryAuth](ctx, tc.State, testFactoryURL)

				return err == nil && res.TypedSpec().Value.GetApiToken() == "rotated-omni-token" && factory.requests.Load() == 2
			}, 90*time.Second, 500*time.Millisecond)
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
// configuration and nothing on the resources of a basic auth factory expires.
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
