// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package dnslabel provides a check for RFC 1123 DNS labels. Use it for user-supplied
// names that have to appear in a DNS label, a Kubernetes name, or the local part of an
// email-style identity.
package dnslabel

import (
	"fmt"
	"regexp"
)

// MaxLength is the maximum length of an RFC 1123 DNS label.
const MaxLength = 63

// dnsLabelRegexp matches the charset shape of an RFC 1123 DNS label: lowercase
// alphanumeric and hyphens, starting and ending with an alphanumeric. The length cap is
// enforced separately by IsValid and Validate.
var dnsLabelRegexp = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// IsValid reports whether s is a valid RFC 1123 DNS label.
func IsValid(s string) bool {
	return len(s) <= MaxLength && dnsLabelRegexp.MatchString(s)
}

// Validate returns nil if s is a valid RFC 1123 DNS label, and a descriptive error
// otherwise. The error for an over-length input intentionally omits the value to avoid
// echoing arbitrarily large user input back into logs and responses.
func Validate(s string) error {
	if len(s) > MaxLength {
		return fmt.Errorf("DNS-1123 label is too long: %d bytes (max %d)", len(s), MaxLength)
	}

	if !dnsLabelRegexp.MatchString(s) {
		return fmt.Errorf("%q is not a valid DNS-1123 label (lowercase alphanumeric and hyphens, starting and ending with alphanumeric, max %d characters)", s, MaxLength)
	}

	return nil
}
