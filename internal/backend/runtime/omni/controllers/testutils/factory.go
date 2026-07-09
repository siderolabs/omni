// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"net/http"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

// NewFactoryClientSet creates a new ImageFactoryClientProvider with the given clients.
func NewFactoryClientSet(clients ...imagefactory.FactoryClient) omni.ImageFactoryClientProvider {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	var primaryClient imagefactory.FactoryClient

	if len(clients) == 1 {
		primaryClient = clients[0]
	} else {
		primaryClient = &ImageFactoryClientMock{}
	}

	res := imagefactory.NewClients(st, primaryClient)

	if len(clients) == 2 {
		res.SetSecondary(clients[1])
	}

	return res
}

// ImageFactoryClientMock is a mock implementation of the ImageFactoryClient interface for testing purposes.
type ImageFactoryClientMock struct {
	schematics map[string]schematic.Schematic
	Owner      string
	mu         sync.Mutex
}

func (i *ImageFactoryClientMock) EnsureSchematic(_ context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error) {
	if i.Owner != "" {
		inputSchematic.Owner = i.Owner
	}

	id, err := inputSchematic.ID()
	if err != nil {
		return "", nil, err
	}

	stored := inputSchematic

	i.mu.Lock()
	if i.schematics == nil {
		i.schematics = map[string]schematic.Schematic{}
	}

	i.schematics[id] = stored
	i.mu.Unlock()

	return id, &stored, nil
}

func (i *ImageFactoryClientMock) SchematicGet(_ context.Context, id string) (*schematic.Schematic, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	s, ok := i.schematics[id]
	if !ok {
		return nil, &client.HTTPError{Code: http.StatusNotFound, Message: "not found"}
	}

	return &s, nil
}

func (i *ImageFactoryClientMock) Host() string {
	return "image.factory.test"
}

func (i *ImageFactoryClientMock) URL() string {
	return "https://image.factory.test"
}

func (i *ImageFactoryClientMock) OverlaysVersions(_ context.Context, _ string) ([]client.OverlayInfo, error) {
	return nil, nil
}

func (i *ImageFactoryClientMock) Versions(_ context.Context) ([]string, error) {
	return []string{"v1.13.6"}, nil
}

func (i *ImageFactoryClientMock) ExtensionsVersions(_ context.Context, _ string) ([]client.ExtensionInfo, error) {
	return nil, nil
}

func (i *ImageFactoryClientMock) CachedIsEnterprise() bool {
	return false
}

func (i *ImageFactoryClientMock) TalosctlList(_ context.Context, _ string) ([]string, error) {
	return []string{"https://image.factory.test/talosctl/v1.13.6/talosctl"}, nil
}

func (i *ImageFactoryClientMock) ScanReport(_ context.Context, _, _, _, _ string) ([]byte, error) {
	return nil, nil
}

// Get is a test helper for reading back what the controller uploaded.
func (m *ImageFactoryClientMock) Get(id string) (schematic.Schematic, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.schematics[id]

	return s, ok
}
