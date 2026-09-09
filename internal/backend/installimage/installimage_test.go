// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package installimage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/installimage"
)

func TestBuild(t *testing.T) {
	const factoryHost = "factory.example.com"

	for _, tt := range []struct {
		name         string
		installImage *specs.MachineConfigGenOptionsSpec_InstallImage
		expected     string
		expectErr    bool
	}{
		{
			name: "builds the installer image from the given factory host",
			installImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
				TalosVersion:         "1.9.0",
				SchematicId:          "abc",
				SchematicInitialized: true,
				Platform:             "metal",
				SecurityState:        &specs.SecurityState{},
				ImageFactoryHost:     factoryHost,
			},
			expected: "factory.example.com/installer/abc:v1.9.0",
		},
		{
			name: "platform prefixed installer for Talos 1.10+",
			installImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
				TalosVersion:         "1.10.0",
				SchematicId:          "abc",
				SchematicInitialized: true,
				Platform:             "metal",
				SecurityState:        &specs.SecurityState{},
				ImageFactoryHost:     factoryHost,
			},
			expected: "factory.example.com/metal-installer/abc:v1.10.0",
		},
		{
			name: "secure boot installer",
			installImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
				TalosVersion:         "1.9.0",
				SchematicId:          "abc",
				SchematicInitialized: true,
				Platform:             "metal",
				SecurityState:        &specs.SecurityState{SecureBoot: true},
				ImageFactoryHost:     factoryHost,
			},
			expected: "factory.example.com/installer-secureboot/abc:v1.9.0",
		},
		{
			name: "invalid schematic still installs from the factory",
			installImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
				TalosVersion:         "1.9.0",
				SchematicId:          "abc",
				SchematicInitialized: true,
				SchematicInvalid:     true,
				Platform:             "metal",
				SecurityState:        &specs.SecurityState{},
				ImageFactoryHost:     factoryHost,
			},
			expected: "factory.example.com/installer/abc:v1.9.0",
		},
		{
			name: "empty schematic ID is an error, there is no plain installer to fall back to",
			installImage: &specs.MachineConfigGenOptionsSpec_InstallImage{
				TalosVersion:         "1.9.0",
				SchematicInitialized: true,
				Platform:             "metal",
				SecurityState:        &specs.SecurityState{},
				ImageFactoryHost:     factoryHost,
			},
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := installimage.Build("machine-1", tt.installImage)
			if tt.expectErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
