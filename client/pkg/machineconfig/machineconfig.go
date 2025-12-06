// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machineconfig provides functionality to generate machine configuration.
package machineconfig

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	machineapi "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// GenerateInput holds the input parameters for generating machine configuration.
type GenerateInput struct {
	Secrets                  *secrets.Bundle
	ClusterID                string
	MachineID                string
	InitialTalosVersion      string
	InitialKubernetesVersion string
	SiderolinkEndpoint       string
	InstallDisk              string
	InstallImage             string
	ExtraGenOptions          []generate.Option
	IsControlPlane           bool
}

// GenerateOutput holds the output of the machine configuration generation.
type GenerateOutput struct {
	Config config.Provider
}

// Generate generates machine configuration based on the provided input.
func Generate(input GenerateInput) (GenerateOutput, error) {
	genOptions := []generate.Option{
		generate.WithInstallImage(input.InstallImage),
	}

	genOptions = append(genOptions, input.ExtraGenOptions...)

	if input.InstallDisk != "" {
		genOptions = append(genOptions, generate.WithInstallDisk(input.InstallDisk))
	}

	versionContract, err := config.ParseContractFromVersion(input.InitialTalosVersion)
	if err != nil {
		return GenerateOutput{}, err
	}

	genOptions = append(genOptions, generate.WithVersionContract(versionContract))

	// For Talos 1.5+, enable KubePrism feature. It's not enabled by default in the machine generation.
	if versionContract.Greater(config.TalosVersion1_4) {
		genOptions = append(genOptions, generate.WithKubePrismPort(constants.DefaultKubePrismPort))
	}

	genOptions = append(genOptions, generate.WithSecretsBundle(input.Secrets))

	genInput, err := generate.NewInput(
		input.ClusterID,
		input.SiderolinkEndpoint,
		input.InitialKubernetesVersion,
		genOptions...,
	)
	if err != nil {
		return GenerateOutput{}, err
	}

	machineType := machineapi.TypeWorker

	if input.IsControlPlane {
		machineType = machineapi.TypeControlPlane
	}

	cfg, err := genInput.Config(machineType)
	if err != nil {
		return GenerateOutput{}, err
	}

	return GenerateOutput{
		Config: cfg,
	}, nil
}
