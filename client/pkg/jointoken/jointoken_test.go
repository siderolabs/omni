// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jointoken_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
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

// TestLegacyTokensRejectV3Fields pins the fix for a token forgery: the version 1 and 2 signature
// covers the extra data alone, so a holder of such a token - they travel in the kernel command
// line - could splice labels into its JSON and keep the HMAC valid, seeding labels of their
// choosing onto the machine. Those fields must be refused before the signature is ever checked.
func TestLegacyTokensRejectV3Fields(t *testing.T) {
	t.Parallel()

	for _, version := range []string{jointoken.Version1, jointoken.Version2} {
		t.Run("v"+version, func(t *testing.T) {
			t.Parallel()

			legit, err := jointoken.NewWithExtraData("secret", version, map[string]string{
				omni.LabelMachineRequest: "req-1",
			})
			require.NoError(t, err)

			encoded, err := legit.Encode()
			require.NoError(t, err)

			prefix := "v" + version + ":"

			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, prefix))
			require.NoError(t, err)

			var payload map[string]any

			require.NoError(t, json.Unmarshal(raw, &payload))

			for _, field := range []struct {
				value any
				name  string
			}{
				{name: "labels", value: map[string]string{"role": "somebody-elses-cluster"}},
				{name: "token_fp", value: jointoken.Fingerprint("secret")},
			} {
				tampered := maps.Clone(payload)
				tampered[field.name] = field.value

				marshaled, marshalErr := json.Marshal(tampered)
				require.NoError(t, marshalErr)

				_, err = jointoken.Parse(prefix + base64.StdEncoding.EncodeToString(marshaled))

				require.Error(t, err, "a version %s token carrying %q must be rejected", version, field.name)
				require.Contains(t, err.Error(), "must not carry labels or a token fingerprint")
			}

			// the untouched token still parses and validates
			parsed, err := jointoken.Parse(encoded)
			require.NoError(t, err)
			require.True(t, parsed.IsValid("secret"))
		})
	}
}

// TestParseValidatesInbound covers the decode side: the signature only proves who wrote a token,
// and the provision handler writes its labels to a state which does not run the resource metadata
// validations, so Parse has to reject what the constructors would never have produced.
func TestParseValidatesInbound(t *testing.T) {
	t.Parallel()

	// craft a payload by hand, the way a join token holder would
	forge := func(t *testing.T, mutate func(payload map[string]any)) string {
		t.Helper()

		legit, err := jointoken.NewWithLabels("secret", map[string]string{"env": "prod"}, nil)
		require.NoError(t, err)

		encoded, err := legit.Encode()
		require.NoError(t, err)

		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "v3:"))
		require.NoError(t, err)

		var payload map[string]any

		require.NoError(t, json.Unmarshal(raw, &payload))

		mutate(payload)

		marshaled, err := json.Marshal(payload)
		require.NoError(t, err)

		return "v3:" + base64.StdEncoding.EncodeToString(marshaled)
	}

	for _, tt := range []struct {
		mutate    func(payload map[string]any)
		name      string
		errString string
	}{
		{
			name:      "control characters in a label value",
			mutate:    func(p map[string]any) { p["labels"] = map[string]string{"env": "pr\x00od"} },
			errString: "must not contain control characters",
		},
		{
			name:      "empty label key",
			mutate:    func(p map[string]any) { p["labels"] = map[string]string{"": "v"} },
			errString: "label key must not be empty",
		},
		{
			name:      "oversized label value",
			mutate:    func(p map[string]any) { p["labels"] = map[string]string{"env": strings.Repeat("v", 64*1024)} },
			errString: "too long",
		},
		{
			name: "unsupported extra data key",
			mutate: func(p map[string]any) {
				p["extra_data"] = map[string]string{omni.LabelCluster: "c1"}
			},
			errString: "is not supported",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := jointoken.Parse(forge(t, tt.mutate))

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errString)
		})
	}

	// tokens the constructors produce keep parsing, for every version
	t.Run("legitimate tokens still parse", func(t *testing.T) {
		t.Parallel()

		v3, err := jointoken.NewWithLabels("secret", map[string]string{"env": "prod"}, map[string]string{
			omni.LabelMachineRequest: "req-1",
		})
		require.NoError(t, err)

		versions := []string{jointoken.Version1, jointoken.Version2}

		tokens := make([]jointoken.JoinToken, 0, len(versions)+1)
		tokens = append(tokens, v3)

		for _, version := range versions {
			legacy, legacyErr := jointoken.NewWithExtraData("secret", version, map[string]string{
				omni.LabelInfraProviderID: "bare-metal",
			})
			require.NoError(t, legacyErr)

			tokens = append(tokens, legacy)
		}

		for _, token := range tokens {
			encoded, encodeErr := token.Encode()
			require.NoError(t, encodeErr)

			parsed, parseErr := jointoken.Parse(encoded)

			require.NoError(t, parseErr, "version %s must still parse", token.Version)
			require.True(t, parsed.IsValid("secret"))
		}
	})
}
