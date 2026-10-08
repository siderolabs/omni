// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"net/http"
	"time"
)

// ReauthCookieName is the cookie that makes the next login from the browser require a fresh login at the identity provider.
//
// It is set on logout, so that the identity provider session left behind cannot sign the same user back in.
const ReauthCookieName = "omni_reauth"

// reauthCookieTTL is the longest lifetime browsers allow, so that the cookie outlives the identity provider session.
const reauthCookieTTL = 400 * 24 * time.Hour

// SetReauthCookie sets the cookie that requires a fresh login at the identity provider on the next login.
func SetReauthCookie(w http.ResponseWriter) {
	http.SetCookie(w, reauthCookie("1", int(reauthCookieTTL.Seconds())))
}

// DeleteReauthCookie removes the cookie set by SetReauthCookie.
func DeleteReauthCookie(w http.ResponseWriter) {
	http.SetCookie(w, reauthCookie("", -1))
}

func reauthCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     ReauthCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// HasReauthCookie returns true if the request carries the cookie set by SetReauthCookie.
func HasReauthCookie(r *http.Request) bool {
	_, err := r.Cookie(ReauthCookieName)

	return err == nil
}
