// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd_test

import (
	"regexp"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/cmd/omni/cmd"
)

// credentialFlag matches the flag names that take a secret as their value. "key" on its own is left
// out, because most of them name a key file or a key source rather than a value.
var credentialFlag = regexp.MustCompile(`(?i)password|passphrase|secret|token|api-?key`)

// plainFlags are the flags that match credentialFlag and do not take a secret, each with the reason.
// Anything else that matches has to be deprecated in favor of an environment variable.
var plainFlags = map[string]string{
	"account-posthog-api-key":             "the PostHog project key, served to the browser",
	"user-pilot-app-token":                "the UserPilot application token, served to the browser",
	"primary-factory-token-file":          "a path, not the token",
	"secondary-factory-token-file":        "a path, not the token",
	"primary-factory-machine-token-ttl":   "a duration",
	"secondary-factory-machine-token-ttl": "a duration",
	"join-tokens-mode":                    "the name of a mode, not a token",
	"machine-api-key":                     "a path to a TLS key file",
}

// TestSecretFlagsAreDeprecated checks that a flag taking a secret as its value is deprecated, so that
// the value is supplied through an environment variable or a config file instead.
//
// It matches on the flag name, so it catches a new flag named the way the existing ones are. A secret
// under a name that reads like nothing of the sort still gets through.
func TestSecretFlagsAreDeprecated(t *testing.T) {
	root, err := cmd.BuildRootCommand()
	require.NoError(t, err)

	root.Flags().VisitAll(func(f *pflag.Flag) {
		if !credentialFlag.MatchString(f.Name) {
			return
		}

		if _, plain := plainFlags[f.Name]; plain {
			return
		}

		assert.NotEmpty(t, f.Deprecated, "--%s takes a secret as its value, deprecate it in favor of an environment variable", f.Name)
	})
}
