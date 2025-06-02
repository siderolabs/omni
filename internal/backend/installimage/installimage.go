// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package installimage provides utilities to work with the install images.
package installimage

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
	appconfig "github.com/siderolabs/omni/internal/pkg/config"
)

// Build builds the install image for the provided properties.
func Build(imageFactoryHost string, resID resource.ID, installImage *specs.MachineConfigGenOptionsSpec_InstallImage) (string, error) {
	if imageFactoryHost == "" {
		return "", fmt.Errorf("image factory host is not set")
	}

	if installImage == nil {
		return "", fmt.Errorf("install image is nil for machine %q", resID)
	}

	if !installImage.SchematicInitialized {
		return "", fmt.Errorf("machine %q has no schematic information set", resID)
	}

	schematicID := installImage.SchematicId

	securityState := installImage.SecurityState
	if securityState == nil { // should never happen - must have been handled before entering this function
		return "", fmt.Errorf("machine %q has no secure boot status set", resID)
	}

	installerName := "installer"
	if securityState.SecureBoot {
		installerName = "installer-secureboot"
	}

	if installImage.SchematicInvalid {
		schematicID = ""
	}

	desiredTalosVersion := installImage.TalosVersion

	if desiredTalosVersion == "" {
		return "", fmt.Errorf("machine %q has no talos version set", resID)
	}

	version, err := semver.ParseTolerant(desiredTalosVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse Talos version %q: %w", desiredTalosVersion, err)
	}

	if version.Major >= 1 && version.Minor >= 10 { // prepend the platform to the installer name for Talos 1.10+
		if installImage.Platform == "" {
			return "", fmt.Errorf("machine %q has no platform set", resID)
		}

		installerName = installImage.Platform + "-" + installerName
	}

	if !strings.HasPrefix(desiredTalosVersion, "v") {
		desiredTalosVersion = "v" + desiredTalosVersion
	}

	if schematicID != "" {
		return imageFactoryHost + "/" + installerName + "/" + schematicID + ":" + desiredTalosVersion, nil
	}

	return appconfig.Config.TalosRegistry + ":" + desiredTalosVersion, nil
}
