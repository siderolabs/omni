// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// Client is the image factory client.
type Client struct {
	*client.Client

	state state.State
	host  string
}

// NewClient creates a new image factory client.
func NewClient(omniState state.State, imageFactoryBaseURL, username, password string) (*Client, error) {
	var clientOptions []client.Option

	if username != "" && password != "" {
		clientOptions = append(clientOptions, client.WithBasicAuth(username, password))
	}

	factoryClient, err := client.New(imageFactoryBaseURL, clientOptions...)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(imageFactoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory base URL %q: %w", imageFactoryBaseURL, err)
	}

	return &Client{
		state:  omniState,
		Client: factoryClient,
		host:   baseURL.Host,
	}, nil
}

// Host returns the host of the image factory client.
func (cli *Client) Host() string {
	if cli == nil {
		return ""
	}

	return cli.host
}

// EnsuredSchematic contains information on the ensured schematics.
type EnsuredSchematic struct {
	Data    *schematic.Schematic
	FullID  string
	PlainID string
}

// EnsureSchematic ensures that the given schematic exists in the image factory.
//
// It ensures two schematics: one with the full content of the schematic.Schematic, and another with only the system extensions.
//
// We do not call the image factory for the schematics that are already known to (cached by) Omni.
func (cli *Client) EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (EnsuredSchematic, error) {
	fullSchematicID, data, err := cli.SchematicCreate(ctx, inputSchematic)
	if err != nil {
		return EnsuredSchematic{}, fmt.Errorf("failed to ensure single schematic: %w", err)
	}

	plainSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: inputSchematic.Customization.SystemExtensions.OfficialExtensions,
			},
		},
	}

	plainSchematicID, err := plainSchematic.ID()
	if err != nil {
		return EnsuredSchematic{}, fmt.Errorf("failed to generate plain schematic ID: %w", err)
	}

	if plainSchematicID != fullSchematicID {
		plainSchematicID, _, err = cli.SchematicCreate(ctx, plainSchematic)
		if err != nil {
			return EnsuredSchematic{}, fmt.Errorf("failed to ensure plain schematic: %w", err)
		}
	}

	return EnsuredSchematic{
		FullID:  fullSchematicID,
		PlainID: plainSchematicID,
		Data:    data,
	}, nil
}
