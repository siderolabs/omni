// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package backend

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
)

func MakeTalosctlHandler(imageFactoryClients *imagefactory.Clients, logger *zap.Logger) (http.Handler, error) {
	return makeTalosctlHandler(imageFactoryClients, logger)
}
