// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
)

// newFactory starts a fake image factory which counts the schematic creates and answers with an id the client cannot compute itself.
// It fails the creates while fail is set.
func newFactory(t *testing.T, enterprise bool, fail *atomic.Bool) (string, *atomic.Int64) {
	t.Helper()

	var creates atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enterprise {
			w.Header().Set("Server", "Image Factory Enterprise")
		}

		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.Write([]byte(`[]`)) //nolint:errcheck

			return
		}

		creates.Add(1)

		if fail != nil && fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}

		parsed, err := schematic.Unmarshal(body)
		if !assert.NoError(t, err) {
			return
		}

		id, err := parsed.ID()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"id": "factory-" + id, "schematic": string(body)}))
	}))
	t.Cleanup(server.Close)

	return server.URL, &creates
}

// TestEnsureSchematicCached checks that the same schematic is sent to the factory once, different content is sent again.
func TestEnsureSchematicCached(t *testing.T) {
	t.Parallel()

	url, creates := newFactory(t, false, nil)

	client, err := imagefactory.NewClient(url, imagefactory.Auth{})
	require.NoError(t, err)

	first := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0"}}}
	second := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"console=tty0"}}}

	firstLocalID, err := first.ID()
	require.NoError(t, err)

	firstID, firstData, err := client.EnsureSchematic(t.Context(), first)
	require.NoError(t, err)
	require.EqualValues(t, 1, creates.Load())
	require.Equal(t, "factory-"+firstLocalID, firstID)

	cachedID, cachedData, err := client.EnsureSchematic(t.Context(), first)
	require.NoError(t, err)
	require.EqualValues(t, 1, creates.Load(), "the same schematic should not be sent again")
	require.Equal(t, firstID, cachedID)
	require.Equal(t, firstData, cachedData)

	// the community factory gets the schematic without the owner, so it is the same schematic
	withOwner := first
	withOwner.Owner = "customer"

	ownerID, _, err := client.EnsureSchematic(t.Context(), withOwner)
	require.NoError(t, err)
	require.EqualValues(t, 1, creates.Load(), "the owner is dropped for a community factory, the schematic is the same")
	require.Equal(t, firstID, ownerID)

	secondID, _, err := client.EnsureSchematic(t.Context(), second)
	require.NoError(t, err)
	require.EqualValues(t, 2, creates.Load())
	require.NotEqual(t, firstID, secondID)
}

// TestEnsureSchematicErrorNotCached checks that a failed create is sent to the factory again.
func TestEnsureSchematicErrorNotCached(t *testing.T) {
	t.Parallel()

	var fail atomic.Bool

	fail.Store(true)

	url, creates := newFactory(t, false, &fail)

	client, err := imagefactory.NewClient(url, imagefactory.Auth{})
	require.NoError(t, err)

	input := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0"}}}

	_, _, err = client.EnsureSchematic(t.Context(), input)
	require.Error(t, err)
	require.EqualValues(t, 1, creates.Load())

	fail.Store(false)

	id, _, err := client.EnsureSchematic(t.Context(), input)
	require.NoError(t, err)
	require.EqualValues(t, 2, creates.Load(), "a failed create should not be cached")
	require.NotEmpty(t, id)
}

// TestEnsureSchematicEnterpriseOwner checks that an Enterprise factory gets the owner, so a different owner is a different schematic.
func TestEnsureSchematicEnterpriseOwner(t *testing.T) {
	t.Parallel()

	url, creates := newFactory(t, true, nil)

	client, err := imagefactory.NewClient(url, imagefactory.Auth{})
	require.NoError(t, err)

	input := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0"}}}

	withOwner := input
	withOwner.Owner = "owner"

	id, _, err := client.EnsureSchematic(t.Context(), input)
	require.NoError(t, err)
	require.True(t, client.CachedIsEnterprise())

	ownerID, _, err := client.EnsureSchematic(t.Context(), withOwner)
	require.NoError(t, err)
	require.EqualValues(t, 2, creates.Load(), "the owner is kept for an Enterprise factory, the schematic is different")
	require.NotEqual(t, id, ownerID)

	cachedID, _, err := client.EnsureSchematic(t.Context(), withOwner)
	require.NoError(t, err)
	require.EqualValues(t, 2, creates.Load())
	require.Equal(t, ownerID, cachedID)
}
