// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
	Name            string
	Architecture    string
	Profile         string
	Type            string
	ContentType     string
	Overlay         string
	MinTalosVersion string
}

var installationMedia = []installationMediaSpec{
	// ISOs
	{
		Name:         "ISO (amd64)",
		Architecture: amd64Arch,
		Profile:      constants.PlatformMetal,
		Type:         isoType,
		ContentType:  "application/x-iso-stream",
	},
	{
		Name:         "ISO (arm64)",
		Architecture: arm64Arch,
		Profile:      constants.PlatformMetal,
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
		Name:         "Equinix Metal (amd64)",
		Architecture: amd64Arch,
		Profile:      "equinixMetal",
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Equinix Metal (arm64)",
		Architecture: arm64Arch,
		Profile:      "equinixMetal",
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
		Profile:      constants.PlatformMetal,
		Type:         rawType,
		ContentType:  "application/x-xz",
	},
	{
		Name:         "Generic image (arm64)",
		Architecture: arm64Arch,
		Profile:      constants.PlatformMetal,
		Type:         rawType,
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

		err := safe.WriterModify(ctx, r, omni.NewInstallationMedia(fname.id), func(newMedia *omni.InstallationMedia) error {
			newMedia.TypedSpec().Value.Architecture = m.Architecture
			newMedia.TypedSpec().Value.Name = m.Name
			newMedia.TypedSpec().Value.Profile = m.Profile
			newMedia.TypedSpec().Value.ContentType = m.ContentType
			newMedia.TypedSpec().Value.DestFilePrefix = fmt.Sprintf("%s-omni-%s", fname.srcPrefix, config.Config.Account.GetName())
			newMedia.TypedSpec().Value.Extension = fname.extension
			newMedia.TypedSpec().Value.MinTalosVersion = m.MinTalosVersion
			newMedia.TypedSpec().Value.Overlay = m.Overlay

			tracker.keep(newMedia)

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to create installation media resource: %w", err)
		}
	}

	for _, cfg := range platforms.SBCs() {
		err := safe.WriterModify(ctx, r, omni.NewInstallationMedia(cfg.Name+".raw.xz"), func(newMedia *omni.InstallationMedia) error {
			newMedia.TypedSpec().Value.Architecture = "arm64"
			newMedia.TypedSpec().Value.Name = cfg.Label
			newMedia.TypedSpec().Value.Profile = constants.PlatformMetal
			newMedia.TypedSpec().Value.ContentType = "application/x-xz"
			newMedia.TypedSpec().Value.DestFilePrefix = fmt.Sprintf("metal-%s-omni-%s", cfg.OverlayName, config.Config.Account.GetName())
			newMedia.TypedSpec().Value.Extension = "raw.xz"
			newMedia.TypedSpec().Value.NoSecureBoot = true
			newMedia.TypedSpec().Value.MinTalosVersion = cfg.MinVersion.String()
			newMedia.TypedSpec().Value.Overlay = cfg.OverlayName

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
