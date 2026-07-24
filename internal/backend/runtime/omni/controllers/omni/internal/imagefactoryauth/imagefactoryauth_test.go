// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactoryauth_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/imagefactoryauth"
)

func newImageFactoryAuth(id, username, password string) *omni.ImageFactoryAuth {
	return typed.NewResource[omni.ImageFactoryAuthSpec, omni.ImageFactoryAuthExtension](
		resource.NewMetadata(resources.DefaultNamespace, omni.ImageFactoryAuthType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ImageFactoryAuthSpec{
			Username: username,
			Password: password,
		}),
	)
}

func TestBuildDocs(t *testing.T) {
	t.Parallel()

	t.Run("no credentials returns nil", func(t *testing.T) {
		t.Parallel()

		docs, err := imagefactoryauth.BuildDocs(nil)
		require.NoError(t, err)
		assert.Empty(t, docs)
	})

	t.Run("only primary credentials", func(t *testing.T) {
		t.Parallel()

		creds := []*omni.ImageFactoryAuth{
			newImageFactoryAuth("https://factory.example.org", "user", "pass"),
		}

		docs, err := imagefactoryauth.BuildDocs(creds)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		assert.Equal(t, "factory.example.org", docs[0].Name())
		assert.Equal(t, "user", docs[0].Username())
		assert.Equal(t, "pass", docs[0].Password())
	})

	t.Run("primary and secondary credentials", func(t *testing.T) {
		t.Parallel()

		creds := []*omni.ImageFactoryAuth{
			newImageFactoryAuth("https://factory.example.org", "user", "pass"),
			newImageFactoryAuth("https://factory.secondary.example.org", "secondary-user", "secondary-pass"),
		}

		docs, err := imagefactoryauth.BuildDocs(creds)
		require.NoError(t, err)
		require.Len(t, docs, 2)

		assert.Equal(t, "factory.example.org", docs[0].Name())
		assert.Equal(t, "user", docs[0].Username())
		assert.Equal(t, "pass", docs[0].Password())

		assert.Equal(t, "factory.secondary.example.org", docs[1].Name())
		assert.Equal(t, "secondary-user", docs[1].Username())
		assert.Equal(t, "secondary-pass", docs[1].Password())
	})

	t.Run("factory without credentials is skipped", func(t *testing.T) {
		t.Parallel()

		creds := []*omni.ImageFactoryAuth{
			newImageFactoryAuth("https://factory.example.org", "user", "pass"),
			newImageFactoryAuth("https://factory.secondary.example.org", "", ""),
		}

		docs, err := imagefactoryauth.BuildDocs(creds)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		assert.Equal(t, "factory.example.org", docs[0].Name())
	})
}
