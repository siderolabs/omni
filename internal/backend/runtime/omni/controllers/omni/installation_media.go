// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/boards"
	"github.com/siderolabs/omni/internal/pkg/config"
)

const (
	amd64Arch = "amd64"
	arm64Arch = "arm64"
	isoType   = "iso"
	tarType   = "tar"
	rawType   = "raw"
	ovaType   = "ova"
	vhdType   = "vhd"
	qcowType  = "qcow2"
)

type installationMediaSpec struct {
	Name         string
	Architecture string
	Profile      string
	Type         string
	ContentType  string
	Overlay      string
	SBC          bool
}

var installationMedia = []installationMediaSpec{
	// ISOs
	{
		Name:         "ISO (amd64)",
		Architecture: amd64Arch,
		Profile:      "metal",
		Type:         isoType,
		ContentType:  "application/x-iso-stream",
	},
	{
		Name:         "ISO (arm64)",
		Architecture: arm64Arch,
		Profile:      "metal",
		Type:         isoType,
		ContentType:  "application/x-iso-stream",
	},

	// CloudImages
	{
		Name:         "Akamai (amd64)",
		Architecture: amd64Arch,
		Profile:      "akamai",
		Type:         rawType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "Akamai (arm64)",
		Architecture: arm64Arch,
		Profile:      "akamai",
		Type:         rawType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "AWS AMI (amd64)",
		Architecture: amd64Arch,
		Profile:      "aws",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "AWS AMI (arm64)",
		Architecture: arm64Arch,
		Profile:      "aws",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Azure (amd64)",
		Architecture: amd64Arch,
		Profile:      "azure",
		Type:         vhdType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Azure (arm64)",
		Architecture: arm64Arch,
		Profile:      "azure",
		Type:         vhdType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Digital Ocean (amd64)",
		Architecture: amd64Arch,
		Profile:      "digital-ocean",
		Type:         rawType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "Digital Ocean (arm64)",
		Architecture: arm64Arch,
		Profile:      "digital-ocean",
		Type:         rawType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "GCP (amd64)",
		Architecture: amd64Arch,
		Profile:      "gcp",
		Type:         rawType + "." + tarType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "GCP (arm64)",
		Architecture: arm64Arch,
		Profile:      "gcp",
		Type:         rawType + "." + tarType,
		ContentType:  "application/gzip",
	},
	{
		Name:         "Hetzner Cloud (amd64)",
		Architecture: amd64Arch,
		Profile:      "hcloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Hetzner Cloud (arm64)",
		Architecture: arm64Arch,
		Profile:      "hcloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "NoCloud (amd64)",
		Architecture: amd64Arch,
		Profile:      "nocloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "NoCloud (arm64)",
		Architecture: arm64Arch,
		Profile:      "nocloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "OpenStack (amd64)",
		Architecture: amd64Arch,
		Profile:      "openstack",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "OpenStack (arm64)",
		Architecture: arm64Arch,
		Profile:      "openstack",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Scaleway (amd64)",
		Architecture: amd64Arch,
		Profile:      "scaleway",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Scaleway (arm64)",
		Architecture: arm64Arch,
		Profile:      "scaleway",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "VMWare (amd64)",
		Architecture: amd64Arch,
		Profile:      "vmware",
		Type:         ovaType,
		ContentType:  "application/ovf",
	},
	{
		Name:         "VMWare (arm64)",
		Architecture: arm64Arch,
		Profile:      "vmware",
		Type:         ovaType,
		ContentType:  "application/ovf",
	},
	{
		Name:         "Oracle (amd64)",
		Architecture: amd64Arch,
		Profile:      "oracle",
		Type:         qcowType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Oracle (arm64)",
		Architecture: arm64Arch,
		Profile:      "oracle",
		Type:         qcowType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Vultr (amd64)",
		Architecture: amd64Arch,
		Profile:      "vultr",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Vultr (arm64)",
		Architecture: arm64Arch,
		Profile:      "vultr",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Upcloud (amd64)",
		Architecture: amd64Arch,
		Profile:      "upcloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Upcloud (arm64)",
		Architecture: arm64Arch,
		Profile:      "upcloud",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Generic image (amd64)",
		Architecture: amd64Arch,
		Profile:      "metal",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Generic image (arm64)",
		Architecture: arm64Arch,
		Profile:      "metal",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},

	// SBC images
	{
		Name:         "Banana Pi BPI-M64 (arm64)",
		Architecture: arm64Arch,
		Profile:      constants.BoardBananaPiM64,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Jetson Nano",
		Architecture: arm64Arch,
		Profile:      constants.BoardJetsonNano,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Libre Computer Profile ALL-H3-CC",
		Architecture: arm64Arch,
		Profile:      constants.BoardLibretechAllH3CCH5,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Pine64",
		Architecture: arm64Arch,
		Profile:      constants.BoardPine64,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Pine64 Rock64",
		Architecture: arm64Arch,
		Profile:      constants.BoardRock64,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Radxa ROCK PI 4",
		Architecture: arm64Arch,
		Profile:      constants.BoardRockpi4,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Radxa ROCK PI 4C",
		Architecture: arm64Arch,
		Profile:      constants.BoardRockpi4c,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Raspberry Pi 4 Model B",
		Architecture: arm64Arch,
		Profile:      constants.BoardRPiGeneric,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Nano Pi R4S",
		Architecture: arm64Arch,
		Profile:      constants.BoardNanoPiR4S,
		Type:         rawType,
		SBC:          true,
		ContentType:  "application/x-xz",
	},
}

// InstallationMediaController manages omni.InstallationMedia.
type InstallationMediaController struct{}

// Name implements controller.Controller interface.
func (ctrl *InstallationMediaController) Name() string {
	return "InstallationMediaController"
}

// Inputs implements controller.Controller interface.
func (ctrl *InstallationMediaController) Inputs() []controller.Input {
	return []controller.Input{}
}

// Outputs implements controller.Controller interface.
func (ctrl *InstallationMediaController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.InstallationMediaType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InstallationMediaController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	tracker := trackResource(r, resources.EphemeralNamespace, omni.InstallationMediaType)

	for _, m := range installationMedia {
		fname := generateFilename(m)

		err := safe.WriterModify(ctx, r, omni.NewInstallationMedia(resources.EphemeralNamespace, fname.id), func(newMedia *omni.InstallationMedia) error {
			newMedia.TypedSpec().Value.Architecture = m.Architecture
			newMedia.TypedSpec().Value.Name = m.Name
			newMedia.TypedSpec().Value.Profile = m.Profile
			newMedia.TypedSpec().Value.ContentType = m.ContentType
			newMedia.TypedSpec().Value.DestFilePrefix = fmt.Sprintf("%s-omni-%s", fname.srcPrefix, config.Config.Name)
			newMedia.TypedSpec().Value.Extension = fname.extension
			newMedia.TypedSpec().Value.NoSecureBoot = m.SBC

			overlay := boards.GetOverlay(m.Profile)

			if overlay != nil {
				newMedia.TypedSpec().Value.Overlay = overlay.Name
			}

			tracker.keep(newMedia)

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to create installation media resource: %w", err)
		}
	}

	return tracker.cleanup(ctx)
}

type filename struct {
	id        string
	srcPrefix string
	extension string
}

// Generate filenames for installation media at runtime so we know the account name.
func generateFilename(m installationMediaSpec) filename {
	profile := m.Profile

	if m.SBC {
		profile = "metal-" + m.Profile
	}

	extension := m.Type

	switch m.ContentType {
	case "application/gzip":
		extension += ".gz"
	case "application/x-xz":
		extension += ".xz"
	}

	return filename{
		id:        fmt.Sprintf("%s-%s.%s", profile, m.Architecture, extension),
		srcPrefix: fmt.Sprintf("%s-%s", profile, m.Architecture),
		extension: extension,
	}
}
