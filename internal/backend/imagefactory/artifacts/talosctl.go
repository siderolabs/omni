// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package artifacts

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/cache"
)

// NewTalosctlHandler returns a handler listing the talosctl downloads available for a Talos
// version Omni tracks as a TalosVersion resource.
func NewTalosctlHandler(st state.State, clients *imagefactory.Clients, logger *zap.Logger) http.Handler {
	// The list of versions does not update very often, so we can cache it.
	var cacherMap sync.Map

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type result struct {
			Status    string   `json:"status"`
			Downloads []string `json:"downloads,omitempty"`
		}

		writeResult := func(a any, code int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)

			if err := json.NewEncoder(w).Encode(a); err != nil {
				logger.Error("failed to encode result", zap.Error(err))
			}
		}

		talosVersion := r.PathValue("version")
		if _, err := semver.ParseTolerant(talosVersion); err != nil {
			logger.Info("invalid Talos version", zap.Error(err))
			writeResult(result{Status: "invalid Talos version"}, http.StatusBadRequest)

			return
		}

		ctx := actor.MarkContextAsInternalActor(r.Context())

		if _, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, st, talosVersion); err != nil {
			if state.IsNotFoundError(err) {
				writeResult(result{Status: "unknown Talos version"}, http.StatusNotFound)

				return
			}

			logger.Error("failed to get Talos version", zap.Error(err))
			writeResult(result{Status: "failed to get Talos version"}, http.StatusInternalServerError)

			return
		}

		actual, _ := cacherMap.LoadOrStore(talosVersion, &cache.Value[[]string]{Duration: time.Hour})

		cacher, ok := actual.(*cache.Value[[]string])
		if !ok {
			logger.Error("failed to load version cache")
			writeResult(result{Status: "failed to load version cache"}, http.StatusInternalServerError)

			return
		}

		data, err := cacher.GetOrUpdate(func() ([]string, error) {
			client, err := clients.ForTalosVersion(ctx, talosVersion)
			if err != nil {
				return nil, err
			}

			return client.TalosctlList(ctx, talosVersion)
		})
		if err != nil {
			logger.Error("failed to get latest talosctl release", zap.Error(err))
			writeResult(result{Status: "failed to get latest talosctl release"}, http.StatusInternalServerError)

			return
		}

		writeResult(result{
			Status:    "ok",
			Downloads: data,
		}, http.StatusOK)
	})
}
