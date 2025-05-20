// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package imagefactory implements image factory operations, enriching them with the Omni state.
package imagefactory

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Client is the image factory client.
type Client struct {
	state state.State
	*client.Client
	host string
}

// NewClient creates a new image factory client.
func NewClient(omniState state.State, imageFactoryBaseURL string) (*Client, error) {
	factoryClient, err := client.New(imageFactoryBaseURL)
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
	// FullID is the ID of the full schematic - with the whole content of the schematic.Schematic, i.e., the extra kernel arguments, META values, and system extensions.
	FullID string

	// PlainID is the ID of the plain schematic - with only the system extensions.
	PlainID string
}

// EnsureSchematic ensures that the given schematic exists in the image factory.
//
// It ensures two schematics: one with the full content of the schematic.Schematic, and another with only the system extensions.
//
// We do not call the image factory for the schematics that are already known to (cached by) Omni.
func (cli *Client) EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (EnsuredSchematic, error) {
	fullSchematicID, err := cli.ensureSingleSchematic(ctx, inputSchematic)
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
		plainSchematicID, err = cli.ensureSingleSchematic(ctx, plainSchematic)
		if err != nil {
			return EnsuredSchematic{}, fmt.Errorf("failed to ensure plain schematic: %w", err)
		}
	}

	return EnsuredSchematic{
		FullID:  fullSchematicID,
		PlainID: plainSchematicID,
	}, nil
}

func (cli *Client) ensureSingleSchematic(ctx context.Context, schematic schematic.Schematic) (string, error) {
	schematicID, err := schematic.ID()
	if err != nil {
		return "", fmt.Errorf("failed to generate schematic ID: %w", err)
	}

	schematicResource := omni.NewSchematic(
		resources.DefaultNamespace, schematicID,
	)

	res, err := safe.StateGetByID[*omni.Schematic](ctx, cli.state, schematicID)
	if err != nil && !state.IsNotFoundError(err) {
		return "", err
	}

	if res != nil {
		return schematicID, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err = cli.SchematicCreate(callCtx, schematic); err != nil {
		return "", fmt.Errorf("failed to create schematic: %w", err)
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	if err = cli.state.Create(ctx, schematicResource); err != nil && !state.IsConflictError(err) {
		return "", err
	}

	return schematicID, nil
}
