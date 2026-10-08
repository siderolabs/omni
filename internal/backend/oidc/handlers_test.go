// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/internal/backend/oidc"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

const testAuthURL = "https://idp.example.com/authorize"

func TestLoginPrompt(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                    string
		requireReauthForNewKeys bool
		reauthCookie            bool
		forced                  bool
	}{
		{
			name:                    "required for new keys",
			requireReauthForNewKeys: true,
			forced:                  true,
		},
		{
			name: "IdP session reused",
		},
		{
			name:         "first login after logout",
			reauthCookie: true,
			forced:       true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, target := range []string{
				"/login?flow=frontend",
				"/login?flow=cli&public-key-id=key-id",
				"/login",
			} {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)

				if tt.reauthCookie {
					req.AddCookie(&http.Cookie{Name: auth.ReauthCookieName, Value: "1"})
				}

				rec := httptest.NewRecorder()

				oidc.NewTestHandler(testAuthURL, "", "", tt.requireReauthForNewKeys, zaptest.NewLogger(t)).Login(rec, req)

				resp := rec.Result()
				resp.Body.Close() //nolint:errcheck

				require.Equal(t, http.StatusFound, resp.StatusCode, target)

				redirectURL, err := url.Parse(resp.Header.Get("Location"))
				require.NoError(t, err)

				assert.Equal(t, "idp.example.com", redirectURL.Host, target)

				if tt.forced {
					assert.Equal(t, "login", redirectURL.Query().Get("prompt"), target)
					assert.Equal(t, "120", redirectURL.Query().Get("max_age"), target)
				} else {
					assert.False(t, redirectURL.Query().Has("prompt"), target)
					assert.False(t, redirectURL.Query().Has("max_age"), target)
				}
			}
		})
	}
}

func TestLogoutSetsReauthCookie(t *testing.T) {
	t.Parallel()

	for _, logoutURL := range []string{"", "https://idp.example.com/logout"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/idp-logout", nil)
		rec := httptest.NewRecorder()

		oidc.NewTestHandler(testAuthURL, "", logoutURL, true, zaptest.NewLogger(t)).Logout(rec, req)

		cookie := findCookie(t, rec, auth.ReauthCookieName)
		require.NotNil(t, cookie, "logout URL %q", logoutURL)
		assert.Positive(t, cookie.MaxAge)

		assertStateCookieDeleted(t, rec)
	}
}

func TestConsumeClearsReauthCookie(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
			"access_token": "access-token",
			"token_type":   "bearer",
			"id_token":     "id-token",
		}))
	}))
	t.Cleanup(tokenServer.Close)

	handler := oidc.NewTestHandler(testAuthURL, tokenServer.URL, "", false, zaptest.NewLogger(t))

	loginReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?flow=frontend", nil)
	loginRec := httptest.NewRecorder()

	handler.Login(loginRec, loginReq)

	stateCookie := findCookie(t, loginRec, "state")
	require.NotNil(t, stateCookie)
	assert.Equal(t, "/", stateCookie.Path)

	consumeReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		oidc.RedirectURL+"?"+url.Values{"state": {stateCookie.Value}, "code": {"code"}}.Encode(), nil)
	consumeReq.AddCookie(stateCookie)
	consumeReq.AddCookie(&http.Cookie{Name: auth.ReauthCookieName, Value: "1"})

	consumeRec := httptest.NewRecorder()

	handler.OIDCConsume(consumeRec, consumeReq)

	resp := consumeRec.Result()
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/authenticate", location.Path)

	cookie := findCookie(t, consumeRec, auth.ReauthCookieName)
	require.NotNil(t, cookie)
	assert.Equal(t, -1, cookie.MaxAge)

	assertStateCookieDeleted(t, consumeRec)
}

func TestConsumeFailureKeepsReauthCookie(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
	}))
	t.Cleanup(tokenServer.Close)

	handler := oidc.NewTestHandler(testAuthURL, tokenServer.URL, "", false, zaptest.NewLogger(t))

	loginReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?flow=frontend", nil)
	loginRec := httptest.NewRecorder()

	handler.Login(loginRec, loginReq)

	stateCookie := findCookie(t, loginRec, "state")
	require.NotNil(t, stateCookie)

	for _, tt := range []struct {
		name  string
		state string
	}{
		{
			name:  "state mismatch",
			state: "other-state",
		},
		{
			name:  "failed code exchange",
			state: stateCookie.Value,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			consumeReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
				oidc.RedirectURL+"?"+url.Values{"state": {tt.state}, "code": {"code"}}.Encode(), nil)
			consumeReq.AddCookie(stateCookie)
			consumeReq.AddCookie(&http.Cookie{Name: auth.ReauthCookieName, Value: "1"})

			consumeRec := httptest.NewRecorder()

			handler.OIDCConsume(consumeRec, consumeReq)

			assert.NotContains(t, consumeRec.Header().Get("Location"), "/authenticate")
			assert.Nil(t, findCookie(t, consumeRec, auth.ReauthCookieName))
		})
	}
}

func TestConsumeLogsStaleLogin(t *testing.T) {
	t.Parallel()

	now := time.Now()

	for _, tt := range []struct {
		authTime                *time.Time
		name                    string
		email                   string
		expectedWarning         string
		expectedUser            string
		expectedWarnings        int
		requireReauthForNewKeys bool
	}{
		{
			name:                    "required, fresh login",
			authTime:                new(now.Add(-time.Minute)),
			requireReauthForNewKeys: true,
		},
		{
			name:                    "required, old login",
			email:                   "user@example.com",
			authTime:                new(now.Add(-time.Hour)),
			requireReauthForNewKeys: true,
			expectedWarning:         "could not confirm a fresh login at the identity provider",
			expectedUser:            "user@example.com",
			expectedWarnings:        2,
		},
		{
			name:                    "required, old login without email",
			authTime:                new(now.Add(-time.Hour)),
			requireReauthForNewKeys: true,
			expectedWarning:         "could not confirm a fresh login at the identity provider",
			expectedUser:            "user-id",
			expectedWarnings:        2,
		},
		{
			name:                    "required, auth_time missing",
			requireReauthForNewKeys: true,
			expectedWarning:         "the identity provider does not send auth_time, fresh logins cannot be checked",
			expectedWarnings:        1,
		},
		{
			name:     "not required, old login",
			authTime: new(now.Add(-time.Hour)),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims := map[string]any{"sub": "user-id"}

			if tt.email != "" {
				claims["email"] = tt.email
			}

			// a NumericDate may have a fraction
			if tt.authTime != nil {
				claims["auth_time"] = float64(tt.authTime.Unix()) + 0.5
			}

			payload, err := json.Marshal(claims)
			require.NoError(t, err)

			idToken := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"

			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
					"access_token": "access-token",
					"token_type":   "bearer",
					"id_token":     idToken,
				}))
			}))
			t.Cleanup(tokenServer.Close)

			core, logs := observer.New(zap.WarnLevel)

			handler := oidc.NewTestHandler(testAuthURL, tokenServer.URL, "", tt.requireReauthForNewKeys, zap.New(core))

			// the missing auth_time warning is logged once, not on every login
			for range 2 {
				loginRec := httptest.NewRecorder()

				handler.Login(loginRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?flow=frontend", nil))

				stateCookie := findCookie(t, loginRec, "state")
				require.NotNil(t, stateCookie)

				consumeReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
					oidc.RedirectURL+"?"+url.Values{"state": {stateCookie.Value}, "code": {"code"}}.Encode(), nil)
				consumeReq.AddCookie(stateCookie)

				consumeRec := httptest.NewRecorder()

				handler.OIDCConsume(consumeRec, consumeReq)

				require.Contains(t, consumeRec.Header().Get("Location"), "/authenticate")
			}

			require.Equal(t, tt.expectedWarnings, logs.Len())
			require.Equal(t, tt.expectedWarnings, logs.FilterMessage(tt.expectedWarning).Len())

			if tt.expectedUser != "" {
				require.Equal(t, tt.expectedUser, logs.All()[0].ContextMap()["user"])
			}
		})
	}
}

func assertStateCookieDeleted(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	cookie := findCookie(t, rec, "state")
	require.NotNil(t, cookie)
	assert.Equal(t, -1, cookie.MaxAge)
	assert.Equal(t, "/", cookie.Path)
}

func findCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	resp := rec.Result()
	require.NoError(t, resp.Body.Close())

	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}
