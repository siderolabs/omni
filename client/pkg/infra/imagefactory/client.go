// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"

	omniclient "github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
)

// DirectClient goes directly to the image factory using the provided endpoint (or default one).
type DirectClient struct {
	client *client.Client
}

// ClientOptions configures the direct image factory client.
type ClientOptions struct {
	FactoryEndpoint string
}

// EnsureSchematic creates the schematic in the image factory.
func (se DirectClient) EnsureSchematic(ctx context.Context, schematic schematic.Schematic) (string, error) {
	schematicID, err := schematic.ID()
	if err != nil {
		return "", fmt.Errorf("failed to generate schematic ID: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err = se.client.SchematicCreate(callCtx, schematic); err != nil {
		return "", fmt.Errorf("failed to create schematic: %w", err)
	}

	return schematicID, nil
}

// NewDirectClient creates new Omni based image factory client.
func NewDirectClient(options ClientOptions) (*DirectClient, error) {
	if options.FactoryEndpoint == "" {
		options.FactoryEndpoint = constants.ImageFactoryBaseURL
	}

	client, err := client.New(options.FactoryEndpoint)
	if err != nil {
		return nil, err
	}

	return &DirectClient{
		client: client,
	}, nil
}

// ProxiedClient is the image factory client which proxies the requests through Omni API.
type ProxiedClient struct {
	client *omniclient.Client
}

// NewProxiedClient creates new proxied image factory client.
func NewProxiedClient(client *omniclient.Client) (*ProxiedClient, error) {
	return &ProxiedClient{
		client: client,
	}, nil
}

// EnsureSchematic creates the schematic in the image factory through Omni API.
func (pc *ProxiedClient) EnsureSchematic(ctx context.Context, schematic schematic.Schematic) (string, error) {
	resp, err := pc.client.Management().CreateSchematicFromRaw(ctx, &schematic)
	if err != nil {
		return "", fmt.Errorf("failed to create schematic through Omni API: %w", err)
	}

	return resp.GetSchematicId(), nil
}
