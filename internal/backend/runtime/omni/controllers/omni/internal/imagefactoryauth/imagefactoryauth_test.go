// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactoryauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/imagefactoryauth"
	omnicfg "github.com/siderolabs/omni/internal/pkg/config"
)

func TestBuildDoc(t *testing.T) {
	t.Parallel()

	t.Run("no credentials returns nil", func(t *testing.T) {
		t.Parallel()

		doc, err := imagefactoryauth.BuildDoc(omnicfg.Registries{})
		require.NoError(t, err)
		assert.Nil(t, doc)
	})

	t.Run("only username returns nil", func(t *testing.T) {
		t.Parallel()

		registries := omnicfg.Registries{}
		registries.SetImageFactoryUsername("user")

		doc, err := imagefactoryauth.BuildDoc(registries)
		require.NoError(t, err)
		assert.Nil(t, doc)
	})

	t.Run("credentials configured", func(t *testing.T) {
		t.Parallel()

		registries := omnicfg.Registries{}
		registries.SetImageFactoryBaseURL("https://factory.example.org")
		registries.SetImageFactoryUsername("user")
		registries.SetImageFactoryPassword("pass")

		doc, err := imagefactoryauth.BuildDoc(registries)
		require.NoError(t, err)
		require.NotNil(t, doc)

		assert.Equal(t, "factory.example.org", doc.Name())
		assert.Equal(t, "user", doc.Username())
		assert.Equal(t, "pass", doc.Password())
	})
}

func TestBuildDocs(t *testing.T) {
	t.Parallel()

	t.Run("no credentials returns nil", func(t *testing.T) {
		t.Parallel()

		docs, err := imagefactoryauth.BuildDocs(omnicfg.Registries{})
		require.NoError(t, err)
		assert.Empty(t, docs)
	})

	t.Run("only primary credentials", func(t *testing.T) {
		t.Parallel()

		registries := omnicfg.Registries{}
		registries.SetImageFactoryBaseURL("https://factory.example.org")
		registries.SetImageFactoryUsername("user")
		registries.SetImageFactoryPassword("pass")

		docs, err := imagefactoryauth.BuildDocs(registries)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		assert.Equal(t, "factory.example.org", docs[0].Name())
		assert.Equal(t, "user", docs[0].Username())
		assert.Equal(t, "pass", docs[0].Password())
	})

	t.Run("primary and secondary credentials", func(t *testing.T) {
		t.Parallel()

		registries := omnicfg.Registries{}
		registries.SetImageFactoryBaseURL("https://factory.example.org")
		registries.SetImageFactoryUsername("user")
		registries.SetImageFactoryPassword("pass")

		var secondary omnicfg.Factory

		secondary.SetUrl("https://factory.secondary.example.org")
		secondary.SetUsername("secondary-user")
		secondary.SetPassword("secondary-pass")

		registries.Factories.Secondary = secondary

		docs, err := imagefactoryauth.BuildDocs(registries)
		require.NoError(t, err)
		require.Len(t, docs, 2)

		assert.Equal(t, "factory.example.org", docs[0].Name())
		assert.Equal(t, "user", docs[0].Username())
		assert.Equal(t, "pass", docs[0].Password())

		assert.Equal(t, "factory.secondary.example.org", docs[1].Name())
		assert.Equal(t, "secondary-user", docs[1].Username())
		assert.Equal(t, "secondary-pass", docs[1].Password())
	})

	t.Run("secondary without credentials is skipped", func(t *testing.T) {
		t.Parallel()

		registries := omnicfg.Registries{}
		registries.SetImageFactoryBaseURL("https://factory.example.org")
		registries.SetImageFactoryUsername("user")
		registries.SetImageFactoryPassword("pass")

		var secondary omnicfg.Factory

		secondary.SetUrl("https://factory.secondary.example.org")

		registries.Factories.Secondary = secondary

		docs, err := imagefactoryauth.BuildDocs(registries)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		assert.Equal(t, "factory.example.org", docs[0].Name())
	})
}
