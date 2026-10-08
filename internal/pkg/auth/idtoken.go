// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	// LoginMaxAge is how long ago the login at the identity provider may be when a fresh login is required.
	LoginMaxAge = 2 * time.Minute

	// AllowedClockSkew is the allowed clock difference between Omni and the identity provider.
	AllowedClockSkew = 5 * time.Minute

	// idTokenMaxAge leaves room for Microsoft Entra, which backdates iat by 5 minutes.
	idTokenMaxAge = 10 * time.Minute
)

// CheckLoginAge returns an error if the login at the identity provider was more than LoginMaxAge ago, or in the
// future, allowing AllowedClockSkew in both directions.
func CheckLoginAge(loginTime time.Time) error {
	if loginTime.After(time.Now().Add(AllowedClockSkew)) {
		return errors.New("login time is in the future")
	}

	if time.Since(loginTime) > LoginMaxAge+AllowedClockSkew {
		return errors.New("re-authentication required")
	}

	return nil
}

// LogStaleLogin logs a warning if the login at the identity provider was not fresh, without rejecting it, so that
// identity providers which ignore the request for a fresh login keep working.
func LogStaleLogin(logger *zap.Logger, user string, loginTime time.Time) {
	if err := CheckLoginAge(loginTime); err != nil {
		logger.Warn("could not confirm a fresh login at the identity provider",
			zap.String("user", user), zap.Time("login_time", loginTime), zap.Error(err))
	}
}

// CheckIDTokenAge returns an error if the ID token was issued more than idTokenMaxAge ago, or more than
// AllowedClockSkew in the future.
func CheckIDTokenAge(issuedAt time.Time) error {
	if issuedAt.After(time.Now().Add(AllowedClockSkew)) {
		return errors.New("token is issued in the future")
	}

	if time.Since(issuedAt) > idTokenMaxAge {
		return errors.New("token is too old")
	}

	return nil
}

// EmailNotVerifiedError is an error that occurs when the email address is not verified.
type EmailNotVerifiedError struct {
	Email string
}

// Error implements the error interface.
func (e EmailNotVerifiedError) Error() string {
	return "email not verified: " + e.Email
}
