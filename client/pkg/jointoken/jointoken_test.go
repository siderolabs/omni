// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jointoken_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/jointoken"
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
