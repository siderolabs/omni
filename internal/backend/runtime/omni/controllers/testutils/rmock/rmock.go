// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package rmock provides utilities for creating resources with default values in tests.
package rmock

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	gensecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/machineconfig"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/pkg/certs"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

const (
	defaultSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	imageFactoryHost = "factory-test.talos.dev"
)

var (
	defaults = map[resource.ID]func(ctx context.Context, st state.State, res resource.Resource) error{}
	owners   = map[resource.ID]string{}
)

func addDefaults[T generic.ResourceWithRD](setter func(ctx context.Context, st state.State, r T) error) {
	var zero T

	defaults[zero.ResourceDefinition().Type] = func(ctx context.Context, st state.State, res resource.Resource) error {
		return setter(ctx, st, res.(T)) //nolint:forcetypeassert,errcheck
	}
}

func setOwner[T generic.ResourceWithRD](owner string) {
	var r T

	owners[r.ResourceDefinition().Type] = owner
}

//nolint:gocognit,gocyclo,cyclop,maintidx
func init() {
	addDefaults(func(_ context.Context, _ state.State, res *omni.Cluster) error {
		res.TypedSpec().Value.KubernetesVersion = constants.DefaultKubernetesVersion
		res.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion

		return nil
	})

	addDefaults(func(_ context.Context, _ state.State, res *omni.ClusterStatus) error {
		res.TypedSpec().Value.Available = true
		res.TypedSpec().Value.ControlplaneReady = true
		res.TypedSpec().Value.HasConnectedControlPlanes = true
		res.TypedSpec().Value.KubernetesAPIReady = true
		res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING
		res.TypedSpec().Value.Ready = true

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.ClusterMachine) error {
		machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		if machineSetNode != nil {
			helpers.CopyAllLabels(machineSetNode, res)
		}

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.ClusterMachineStatus) error {
		clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		if clusterMachine != nil {
			helpers.CopyAllLabels(clusterMachine, res)
		}

		res.TypedSpec().Value.ApidAvailable = true
		res.TypedSpec().Value.ConfigApplyStatus = specs.ConfigApplyStatus_APPLIED

		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if machineStatus != nil {
			res.TypedSpec().Value.ManagementAddress = machineStatus.TypedSpec().Value.ManagementAddress
		}

		return nil
	})

	addDefaults(func(_ context.Context, _ state.State, res *omni.LoadBalancerStatus) error {
		res.TypedSpec().Value.Healthy = true

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *system.ResourceLabels[*omni.MachineStatus]) error {
		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if machineStatus != nil {
			helpers.CopyLabels(machineStatus, res, omni.LabelCluster, omni.LabelMachineSet)
		}

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.MachineStatus) error {
		machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if machineSetNode != nil {
			helpers.CopyLabels(machineSetNode, res, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
		}

		talosVersion := constants.DefaultTalosVersion

		clusterName, ok := machineSetNode.Metadata().Labels().Get(omni.LabelCluster)
		if ok {
			cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}

			if cluster != nil {
				talosVersion = cluster.TypedSpec().Value.TalosVersion
			}
		}

		res.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
		res.TypedSpec().Value.TalosVersion = talosVersion

		res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
			Id:           defaultSchematic,
			FullId:       defaultSchematic,
			InitialState: &specs.MachineStatusSpec_Schematic_InitialState{},
		}

		res.TypedSpec().Value.InitialTalosVersion = talosVersion
		res.TypedSpec().Value.SecurityState = &specs.SecurityState{}
		res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
			Platform: talosconstants.PlatformMetal,
		}
		res.TypedSpec().Value.Connected = true

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.ClusterSecrets) error {
		version := constants.DefaultTalosVersion

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if cluster != nil {
			version = cluster.TypedSpec().Value.TalosVersion
		}

		vc, err := config.ParseContractFromVersion("v" + version)
		if err != nil {
			return err
		}

		bundle, err := gensecrets.NewBundle(gensecrets.NewFixedClock(time.Now()), vc)
		if err != nil {
			return err
		}

		res.TypedSpec().Value.Data, err = json.Marshal(bundle)

		return err
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.ClusterMachineConfigPatches) error {
		return res.TypedSpec().Value.SetUncompressedPatches([]string{
			`---
machine:
  install:
    disk: "/dev/vda"`,
		})
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.TalosConfig) error {
		secrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		clientCert, CA, err := certs.TalosAPIClientCertificateFromSecrets(secrets, time.Hour*24, role.MakeSet(role.Admin))
		if err != nil {
			return err
		}

		res.TypedSpec().Value.Ca = base64.StdEncoding.EncodeToString(CA)
		res.TypedSpec().Value.Crt = base64.StdEncoding.EncodeToString(clientCert.Crt)
		res.TypedSpec().Value.Key = base64.StdEncoding.EncodeToString(clientCert.Key)

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.MachineConfigGenOptions) error {
		talosVersion := constants.DefaultTalosVersion

		machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if machineSetNode != nil {
			clusterName, ok := machineSetNode.Metadata().Labels().Get(omni.LabelCluster)
			if ok {
				cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if cluster != nil {
					talosVersion = cluster.TypedSpec().Value.TalosVersion
				}
			}
		}

		res.TypedSpec().Value.InstallDisk = "/dev/vda"
		res.TypedSpec().Value.InstallImage = &specs.MachineConfigGenOptionsSpec_InstallImage{
			TalosVersion:         talosVersion,
			SchematicId:          defaultSchematic,
			Platform:             "metal",
			SchematicInitialized: true,
			SecurityState:        &specs.SecurityState{SecureBoot: false},
		}

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.MachineSetStatus) error {
		machineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		helpers.CopyAllLabels(machineSet, res)

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.MachineSetConfigStatus) error {
		machineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		helpers.CopyAllLabels(machineSet, res)

		res.TypedSpec().Value.ConfigUpdatesAllowed = true
		res.TypedSpec().Value.ShouldResetGraceful = true

		return nil
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.ClusterMachineConfig) error {
		clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		machineConfigGenOptions, err := safe.ReaderGetByID[*omni.MachineConfigGenOptions](ctx, st, res.Metadata().ID())
		if err != nil {
			return err
		}

		clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return errors.New("ClusterMachine missing cluster label")
		}

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
		if err != nil {
			return err
		}

		clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, st, clusterName)
		if err != nil {
			return err
		}

		secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.GetData())
		if err != nil {
			return err
		}

		helpers.CopyAllLabels(clusterMachine, res)

		_, isControlPlane := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole)

		installImage, err := installimage.Build(
			imageFactoryHost,
			machineConfigGenOptions.Metadata().ID(),
			machineConfigGenOptions.TypedSpec().Value.InstallImage,
			"ghcr.io/siderolabs/installer",
		)
		if err != nil {
			return err
		}

		genOutput, err := machineconfig.Generate(machineconfig.GenerateInput{
			ClusterID:                cluster.Metadata().ID(),
			MachineID:                clusterMachine.Metadata().ID(),
			InitialTalosVersion:      cluster.TypedSpec().Value.TalosVersion,
			InitialKubernetesVersion: cluster.TypedSpec().Value.KubernetesVersion,
			IsControlPlane:           isControlPlane,
			SiderolinkEndpoint:       "localhost:6443",
			InstallDisk:              machineConfigGenOptions.TypedSpec().Value.InstallDisk,
			InstallImage:             installImage,
			Secrets:                  secretsBundle,
		})
		if err != nil {
			return err
		}

		cfg := genOutput.Config

		data, err := cfg.Bytes()
		if err != nil {
			return err
		}

		return res.TypedSpec().Value.SetUncompressedData(data)
	})

	addDefaults(func(ctx context.Context, st state.State, res *omni.MachineStatusSnapshot) error {
		res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_RUNNING,
		}

		return nil
	})

	setOwner[*omni.ClusterStatus](omnictrl.NewClusterStatusController(false).ControllerName)
	setOwner[*omni.ClusterMachine](omnictrl.NewMachineSetStatusController().ControllerName)
	setOwner[*omni.ClusterMachineStatus](omnictrl.NewClusterMachineStatusController().ControllerName)
	setOwner[*omni.ClusterSecrets](secrets.NewSecretsController(nil).ControllerName)
	setOwner[*omni.ClusterMachineConfig](omnictrl.NewClusterMachineController().ControllerName)
	setOwner[*omni.MachineConfigGenOptions](omnictrl.NewMachineConfigGenOptionsController().ControllerName)
	setOwner[*omni.TalosConfig](secrets.NewTalosConfigController(omnictrl.DefaultDebounceDuration).ControllerName)
	setOwner[*omni.Machine](omnictrl.NewMachineController().ControllerName)
	setOwner[*omni.MachineSetStatus](omnictrl.NewMachineSetStatusController().ControllerName)
}

// GetOwner returns the default owner used by the mock library for the resource.
func GetOwner[R generic.ResourceWithRD]() string {
	var r R

	return owners[r.ResourceDefinition().Type]
}
