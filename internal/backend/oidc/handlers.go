// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"cmp"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// RedirectURL is the URL where OIDC flow consumes the resulting token.
const RedirectURL = "/oidc/consume"

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	http.SetCookie(w, callbackCookie(r, name, value, int(time.Hour.Seconds())))
}

func deleteCallbackCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, callbackCookie(r, name, "", -1))
}

func callbackCookie(r *http.Request, name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
}

type authState struct {
	Query     string `json:"query"`
	Signature []byte `json:"signature"`
}

func (e authState) signature(token string) ([]byte, error) {
	mac := hmac.New(sha256.New, []byte(token))

	if _, err := mac.Write([]byte(e.Query)); err != nil {
		return nil, err
	}

	return mac.Sum(nil), nil
}

func (e authState) verify(token string) bool {
	mac, err := e.signature(token)
	if err != nil {
		return false
	}

	return hmac.Equal(mac, e.Signature)
}

func (e authState) encode() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func newAuthState(query, token string) (authState, error) {
	s := authState{Query: query}

	var err error

	s.Signature, err = s.signature(token)

	return s, err
}

func parseAuthState(data string) (authState, error) {
	rawJSON, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return authState{}, err
	}

	var state authState

	if err = json.Unmarshal(rawJSON, &state); err != nil {
		return state, err
	}

	return state, nil
}

// Handler is the collection of HTTP routes required for the OIDC auth.
type Handler struct {
	logger                  *zap.Logger
	oauth2Config            oauth2.Config
	key                     string
	logoutURL               string
	endpoint                string
	missingAuthTimeOnce     sync.Once
	requireReauthForNewKeys bool
}

// Login handles the login flow of OIDC auth.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	s, err := newAuthState(r.URL.RawQuery, h.key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	state, err := s.encode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	setCallbackCookie(w, r, "state", state)

	var opts []oauth2.AuthCodeOption

	if h.requireReauthForNewKeys || auth.HasReauthCookie(r) {
		opts = append(opts,
			oauth2.SetAuthURLParam("prompt", "login"),
			oauth2.SetAuthURLParam("max_age", strconv.Itoa(int(auth.LoginMaxAge.Seconds()))),
		)
	}

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state, opts...), http.StatusFound)
}

// Logout handles the logout flow of OIDC auth.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.SetReauthCookie(w)
	deleteCallbackCookie(w, r, "state") // a login started before the logout must not complete after it

	if h.logoutURL == "" {
		http.Redirect(w, r, h.endpoint, http.StatusFound)

		return
	}

	logoutURL, err := url.Parse(h.logoutURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	query := logoutURL.Query()

	query.Add("post_logout_redirect_uri", h.endpoint)

	logoutURL.RawQuery = query.Encode()

	http.Redirect(w, r, logoutURL.String(), http.StatusFound)
}

// OIDCConsume handles the final stage of the OIDC login flow.
func (h *Handler) OIDCConsume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	state, err := r.Cookie("state")
	if err != nil {
		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)

		return
	}

	deleteCallbackCookie(w, r, "state") // single use, so that a callback replayed from the browser history fails

	if r.URL.Query().Get("state") != state.Value {
		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)

		return
	}

	var st authState

	st, err = parseAuthState(r.URL.Query().Get("state"))
	if err != nil {
		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)

		return
	}

	if !st.verify(h.key) {
		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)

		return
	}

	oauth2Token, err := h.oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "failed to exchange token: "+err.Error(), http.StatusInternalServerError)

		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token field in oauth2 token.", http.StatusInternalServerError)

		return
	}

	query, err := url.ParseQuery(st.Query)
	if err != nil {
		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)

		return
	}

	query.Set("token", rawIDToken)

	if h.requireReauthForNewKeys {
		h.logStaleLogin(rawIDToken)
	}

	auth.DeleteReauthCookie(w)

	http.Redirect(w, r, "/authenticate?"+query.Encode(), http.StatusSeeOther)
}

// logStaleLogin reads the ID token without checking its signature, as it comes straight from the token endpoint.
func (h *Handler) logStaleLogin(rawIDToken string) {
	var claims struct {
		Email    string  `json:"email"`
		Subject  string  `json:"sub"`
		AuthTime float64 `json:"auth_time"`
	}

	_, payload, _ := strings.Cut(rawIDToken, ".")
	payload, _, _ = strings.Cut(payload, ".")

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err == nil {
		err = json.Unmarshal(decoded, &claims)
	}

	if err != nil {
		h.logger.Warn("failed to read the ID token to check that the login is fresh", zap.Error(err))

		return
	}

	if claims.AuthTime == 0 {
		h.missingAuthTimeOnce.Do(func() {
			h.logger.Warn("the identity provider does not send auth_time, fresh logins cannot be checked")
		})

		return
	}

	auth.LogStaleLogin(h.logger, cmp.Or(claims.Email, claims.Subject), time.Unix(int64(claims.AuthTime), 0))
}

// NewOIDCHandler creates a new OIDC handler.
func NewOIDCHandler(endpoint string, config config.OIDC, provider *oidc.Provider, requireReauthForNewKeys bool, logger *zap.Logger) (*Handler, error) {
	key, err := randString(16)
	if err != nil {
		return nil, err
	}

	fullRedirectURL, err := url.JoinPath(endpoint + RedirectURL)
	if err != nil {
		return nil, err
	}

	oauth2Config := oauth2.Config{
		ClientID:     config.GetClientID(),
		ClientSecret: string(config.GetClientSecret()),
		RedirectURL:  fullRedirectURL,

		// Discovery returns the OAuth2 endpoints.
		Endpoint: provider.Endpoint(),

		Scopes: config.Scopes,
	}

	return &Handler{
		logger:                  logger,
		key:                     key,
		endpoint:                endpoint,
		logoutURL:               config.GetLogoutURL(),
		oauth2Config:            oauth2Config,
		requireReauthForNewKeys: requireReauthForNewKeys,
	}, nil
}
