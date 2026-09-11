// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jointoken_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestEncodeParse(t *testing.T) {
	t.Parallel()

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		token := jointoken.NewPlain("1234")

		encoded, err := token.Encode()

		require.NoError(t, err)

		assert.True(t, token.IsValid("1234"))

		parsed, err := jointoken.Parse(encoded)

		require.NoError(t, err)

		assert.True(t, parsed.IsValid("1234"))
	})

	t.Run("v1", func(t *testing.T) {
		t.Parallel()

		tokenWithExtraData(t, jointoken.Version1)
	})

	t.Run("v2", func(t *testing.T) {
		t.Parallel()

		tokenWithExtraData(t, jointoken.Version2)
	})
}

func tokenWithExtraData(t *testing.T, version string) {
	token, err := jointoken.NewWithExtraData("1234", version, map[string]string{
		"a": "b",
	})

	require.NoError(t, err)

	encoded, err := token.Encode()

	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(encoded, "v"+version+":"))

	parsed, err := jointoken.Parse(encoded)

	require.NoError(t, err)

	assert.True(t, parsed.IsValid("1234"))
}

func TestGenerateJoinToken(t *testing.T) {
	token, err := jointoken.Generate()

	assert.NoError(t, err)

	tokenLen := len(token)
	assert.Less(t, tokenLen, 52)
	assert.Greater(t, tokenLen, 42)
}

func TestV3Labels(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		token, err := jointoken.NewWithLabels("1234", map[string]string{"env": "prod", "rack": "a12"}, nil)
		require.NoError(t, err)

		assert.Equal(t, jointoken.Fingerprint("1234"), token.TokenFingerprint)

		encoded, err := token.Encode()
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(encoded, "v3:"))

		parsed, err := jointoken.Parse(encoded)
		require.NoError(t, err)

		assert.Equal(t, jointoken.Version3, parsed.Version)
		assert.Equal(t, map[string]string{"env": "prod", "rack": "a12"}, parsed.Labels)
		assert.Equal(t, jointoken.Fingerprint("1234"), parsed.TokenFingerprint)
		assert.True(t, parsed.IsValid("1234"))
		assert.False(t, parsed.IsValid("4321"))
	})

	t.Run("keeps the extra data", func(t *testing.T) {
		t.Parallel()

		token, err := jointoken.NewWithLabels(
			"1234",
			map[string]string{"env": "prod"},
			map[string]string{omni.LabelMachineRequest: "req-1"},
		)
		require.NoError(t, err)

		encoded, err := token.Encode()
		require.NoError(t, err)

		parsed, err := jointoken.Parse(encoded)
		require.NoError(t, err)

		assert.Equal(t, "req-1", parsed.ExtraData[omni.LabelMachineRequest])
		assert.True(t, parsed.IsValid("1234"))
	})

	// a provider token is resolved by the provider ID, and its secret is not the token the machine
	// was handed, so the fingerprint must not be derived from it
	t.Run("no fingerprint for the provider tokens", func(t *testing.T) {
		t.Parallel()

		token, err := jointoken.NewWithLabels(
			"1234",
			map[string]string{"env": "prod"},
			map[string]string{omni.LabelInfraProviderID: "bare-metal"},
		)
		require.NoError(t, err)

		assert.Empty(t, token.TokenFingerprint)
	})

	t.Run("tampering invalidates the signature", func(t *testing.T) {
		t.Parallel()

		token, err := jointoken.NewWithLabels("1234", map[string]string{"env": "prod"}, nil)
		require.NoError(t, err)

		require.True(t, token.IsValid("1234"))

		tampered := token
		tampered.Labels = map[string]string{"env": "prod", "role": "admin"}

		assert.False(t, tampered.IsValid("1234"))

		tampered = token
		tampered.TokenFingerprint = jointoken.Fingerprint("4321")

		assert.False(t, tampered.IsValid("1234"))
	})
}

func TestFingerprint(t *testing.T) {
	t.Parallel()

	assert.Equal(t, jointoken.Fingerprint("1234"), jointoken.Fingerprint("1234"))
	assert.NotEqual(t, jointoken.Fingerprint("1234"), jointoken.Fingerprint("4321"))
	assert.Len(t, jointoken.Fingerprint("1234"), jointoken.FingerprintLen)
	// the fingerprint must not be reversible into the secret
	assert.NotContains(t, jointoken.Fingerprint("1234"), "1234")
}

func TestValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		labels    map[string]string
		extraData map[string]string
		name      string
		errString string
	}{
		{
			name:      "system label",
			labels:    map[string]string{omni.LabelCluster: "c1"},
			errString: "is reserved for the system labels",
		},
		{
			name:      "empty label key",
			labels:    map[string]string{"": "v"},
			errString: "label key must not be empty",
		},
		{
			name:      "control characters",
			labels:    map[string]string{"env": "pr\x00od"},
			errString: "must not contain control characters",
		},
		{
			name:      "unsupported extra data key",
			labels:    map[string]string{"env": "prod"},
			extraData: map[string]string{omni.LabelCluster: "c1"},
			errString: "is not supported",
		},
		{
			name:      "too long",
			labels:    map[string]string{"env": strings.Repeat("a", jointoken.MaxEncodedTokenLen)},
			errString: "join token is too long",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := jointoken.NewWithLabels("1234", tt.labels, tt.extraData)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errString)
		})
	}
}

// TestSignatureCompatibility pins the v1/v2 signature payload: tokens issued before v3 was added
// must keep validating, so the bytes the HMAC is computed over may not change.
func TestSignatureCompatibility(t *testing.T) {
	t.Parallel()

	for _, version := range []string{jointoken.Version1, jointoken.Version2} {
		token, err := jointoken.NewWithExtraData("secret", version, map[string]string{
			omni.LabelMachineRequest: "req-1",
		})
		require.NoError(t, err)

		// HMAC-SHA256("secret", `{"omni.sidero.dev/machine-request":"req-1"}`)
		mac := hmac.New(sha256.New, []byte("secret"))
		_, err = mac.Write([]byte(`{"omni.sidero.dev/machine-request":"req-1"}`))
		require.NoError(t, err)

		assert.Equal(t, mac.Sum(nil), token.Signature, "the version %s signature payload changed", version)
	}
}
