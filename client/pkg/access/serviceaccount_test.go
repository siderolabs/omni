// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/pkg/access"
)

func TestParseFromFullID(t *testing.T) {
	t.Parallel()

	t.Run("not a service account", func(t *testing.T) {
		t.Parallel()

		_, isSa := access.ParseServiceAccountFromFullID("foobar@example.org")
		assert.False(t, isSa)
	})

	t.Run("normal service account", func(t *testing.T) {
		t.Parallel()

		sa, isSa := access.ParseServiceAccountFromFullID("foobar@serviceaccount.omni.sidero.dev")
		assert.True(t, isSa)

		assert.Equal(t, "foobar", sa.BaseName)
		assert.Equal(t, "@serviceaccount.omni.sidero.dev", sa.Suffix)
		assert.False(t, sa.IsCloudProvider)

		assert.Equal(t, "foobar", sa.NameWithPrefix())
		assert.Equal(t, "foobar@serviceaccount.omni.sidero.dev", sa.FullID())
	})

	t.Run("cloud provider service account", func(t *testing.T) {
		t.Parallel()

		sa, isSa := access.ParseServiceAccountFromFullID("aws-1@cloud-provider.serviceaccount.omni.sidero.dev")
		assert.True(t, isSa)

		assert.Equal(t, "aws-1", sa.BaseName)
		assert.Equal(t, "@cloud-provider.serviceaccount.omni.sidero.dev", sa.Suffix)
		assert.True(t, sa.IsCloudProvider)

		assert.Equal(t, "cloud-provider:aws-1", sa.NameWithPrefix())
		assert.Equal(t, "aws-1@cloud-provider.serviceaccount.omni.sidero.dev", sa.FullID())
	})
}

func TestParseFromName(t *testing.T) {
	t.Parallel()

	t.Run("normal service account", func(t *testing.T) {
		t.Parallel()

		sa := access.ParseServiceAccountFromName("foobar")

		assert.Equal(t, "foobar", sa.BaseName)
		assert.Equal(t, "@serviceaccount.omni.sidero.dev", sa.Suffix)
		assert.False(t, sa.IsCloudProvider)

		assert.Equal(t, "foobar", sa.NameWithPrefix())
		assert.Equal(t, "foobar@serviceaccount.omni.sidero.dev", sa.FullID())
	})

	t.Run("cloud provider service account", func(t *testing.T) {
		t.Parallel()

		sa := access.ParseServiceAccountFromName("cloud-provider:aws-1")

		assert.Equal(t, "aws-1", sa.BaseName)
		assert.Equal(t, "@cloud-provider.serviceaccount.omni.sidero.dev", sa.Suffix)
		assert.True(t, sa.IsCloudProvider)

		assert.Equal(t, "cloud-provider:aws-1", sa.NameWithPrefix())
		assert.Equal(t, "aws-1@cloud-provider.serviceaccount.omni.sidero.dev", sa.FullID())
	})
}
