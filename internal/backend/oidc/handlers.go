// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

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
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}

	http.SetCookie(w, c)
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
	rawJson, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return authState{}, err
	}

	var state authState

	if err = json.Unmarshal(rawJson, &state); err != nil {
		return state, err
	}

	return state, nil
}

// Handler is the collection of HTTP routes required for the OIDC auth.
type Handler struct {
	oauth2Config oauth2.Config
	key          string
	logoutURL    string
	endpoint     string
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

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state), http.StatusFound)
}

// Logout handles the logout flow of OIDC auth.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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

	http.Redirect(w, r, "/authenticate?"+query.Encode(), http.StatusSeeOther)
}

// NewOIDCHandler creates a new OIDC handler.
func NewOIDCHandler(endpoint string, config config.OIDC, provider *oidc.Provider) (*Handler, error) {
	key, err := randString(16)
	if err != nil {
		return nil, err
	}

	fullRedirectURL, err := url.JoinPath(endpoint + RedirectURL)
	if err != nil {
		return nil, err
	}

	oauth2Config := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  fullRedirectURL,

		// Discovery returns the OAuth2 endpoints.
		Endpoint: provider.Endpoint(),

		Scopes: config.Scopes,
	}

	return &Handler{
		key:          key,
		endpoint:     endpoint,
		logoutURL:    config.LogoutURL,
		oauth2Config: oauth2Config,
	}, nil
}
