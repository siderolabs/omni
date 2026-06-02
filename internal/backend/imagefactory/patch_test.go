// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory_test

import (
	"errors"
	"testing"

	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/imagefactory"
)

func TestPatchSchematic_PreservesAllFields(t *testing.T) {
	// Real-world enterprise schematic captured from a machine: includes Owner
	// and customization.secureboot which today's reconstruction silently drops.
	raw := `owner: admin
customization:
    extraKernelArgs:
        - console=tty0
        - console=ttyS0
    systemExtensions:
        officialExtensions:
            - siderolabs/hello-world-service
            - siderolabs/qemu-guest-agent
    secureboot:
        includeWellKnownCertificates: true
`

	patched, err := imagefactory.PatchSchematic(
		raw,
		[]string{"siderolabs/hello-world-service", "siderolabs/qemu-guest-agent"},
		[]string{"console=tty0", "console=ttyS0"},
	)
	require.NoError(t, err)

	// Same content → same hash as the source.
	origID, err := mustUnmarshal(t, raw).ID()
	require.NoError(t, err)

	patchedID, err := patched.ID()
	require.NoError(t, err)

	assert.Equal(t, origID, patchedID, "patching with the same extensions and kernel args must yield the same schematic ID")

	// Fields Omni does not manage must survive verbatim.
	assert.Equal(t, "admin", patched.Owner)
	assert.True(t, patched.Customization.SecureBoot.IncludeWellKnownCertificates)
}

func TestPatchSchematic_ChangesExtensionsAndKernelArgs(t *testing.T) {
	raw := `owner: admin
customization:
    extraKernelArgs:
        - console=tty0
    systemExtensions:
        officialExtensions:
            - siderolabs/qemu-guest-agent
    secureboot:
        includeWellKnownCertificates: true
`

	patched, err := imagefactory.PatchSchematic(
		raw,
		[]string{"siderolabs/qemu-guest-agent", "siderolabs/hello-world-service"},
		[]string{"console=tty0", "console=ttyS0", "talos.events.sink=localhost:8090"},
	)
	require.NoError(t, err)

	assert.Equal(t, "admin", patched.Owner)
	assert.True(t, patched.Customization.SecureBoot.IncludeWellKnownCertificates)
	assert.Equal(t,
		[]string{"siderolabs/qemu-guest-agent", "siderolabs/hello-world-service"},
		patched.Customization.SystemExtensions.OfficialExtensions)
	assert.Equal(t,
		[]string{"console=tty0", "console=ttyS0", "talos.events.sink=localhost:8090"},
		patched.Customization.ExtraKernelArgs)

	// Different content → different hash than the source.
	origID, err := mustUnmarshal(t, raw).ID()
	require.NoError(t, err)

	patchedID, err := patched.ID()
	require.NoError(t, err)

	assert.NotEqual(t, origID, patchedID)
}

func TestPatchSchematic_PreservesOverlay(t *testing.T) {
	raw := `overlay:
    name: rpi_generic
    image: ghcr.io/siderolabs/sbc-raspberrypi
    options:
        data: mydata
        count: 3
customization:
    systemExtensions:
        officialExtensions:
            - siderolabs/qemu-guest-agent
`

	patched, err := imagefactory.PatchSchematic(
		raw,
		[]string{"siderolabs/qemu-guest-agent"},
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, "rpi_generic", patched.Overlay.Name)
	assert.Equal(t, "ghcr.io/siderolabs/sbc-raspberrypi", patched.Overlay.Image)
	assert.Equal(t, "mydata", patched.Overlay.Options["data"])
	assert.EqualValues(t, 3, patched.Overlay.Options["count"])
}

func TestPatchSchematic_EmptyRawErrors(t *testing.T) {
	_, err := imagefactory.PatchSchematic("", []string{"siderolabs/qemu-guest-agent"}, nil)
	assert.ErrorIs(t, err, imagefactory.ErrEmptyRawSchematic)
}

func TestPatchSchematic_UnknownFieldErrors(t *testing.T) {
	raw := `owner: admin
customization:
    futureFieldFromTomorrowsFactory: should-not-be-silently-dropped
    systemExtensions:
        officialExtensions:
            - siderolabs/qemu-guest-agent
`

	_, err := imagefactory.PatchSchematic(raw, nil, nil)
	require.Error(t, err, "unknown fields must surface as errors so deps get bumped")
	assert.False(t, errors.Is(err, imagefactory.ErrEmptyRawSchematic))
}

func mustUnmarshal(t *testing.T, raw string) *schematic.Schematic {
	t.Helper()

	s, err := schematic.Unmarshal([]byte(raw))
	require.NoError(t, err)

	return s
}
