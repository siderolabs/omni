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

// TestEnsureSchematicCached checks that the same schematic is sent to the factory once, different content is sent again.
func TestEnsureSchematicCached(t *testing.T) {
	t.Parallel()

	var creates atomic.Int64

	// the factory answers with an id the client cannot compute itself
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.Write([]byte(`[]`)) //nolint:errcheck

			return
		}

		creates.Add(1)

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

	client, err := imagefactory.NewClient(server.URL, imagefactory.Auth{})
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
