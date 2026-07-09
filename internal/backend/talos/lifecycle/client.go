// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/installimage"
)

// AuditFunc is invoked once for each Talos call so callers can record an audit entry. Returning an error aborts the operation. Use NoAudit when audit is not required (e.g. the controller flow).
type AuditFunc func(ctx context.Context, fullMethodName, machineID string) error

// NoAudit is an AuditFunc that does nothing.
func NoAudit(_ context.Context, _, _ string) error { return nil }

// buildInstallImage constructs the installer image reference.
func (m *Manager) buildInstallImage(ctx context.Context, machineID string, ms *omni.MachineStatus, version string, target *specs.MachineConfigGenOptionsSpec_InstallImage) (string, error) {
	spec := ms.TypedSpec().Value

	if spec.GetPlatformMetadata().GetPlatform() == "" {
		return "", status.Error(codes.FailedPrecondition, "machine platform is not known yet")
	}

	if spec.SecurityState == nil {
		return "", status.Error(codes.FailedPrecondition, "machine security state is not known yet")
	}

	if version == "" {
		version = strings.TrimPrefix(spec.TalosVersion, "v")
	}

	if target != nil {
		if target.TalosVersion == "" {
			target.TalosVersion = version
		}

		return installimage.Build(machineID, target, m.talosRegistry)
	}

	imageFactoryClient, err := m.imageFactoryClients.ForTalosVersion(ctx, version)
	if err != nil {
		return "", fmt.Errorf("failed to get image factory client for Talos version %q: %w", version, err)
	}

	installImage := omni.NewInstallImage(ms, version, spec.Schematic.FullId, imageFactoryClient.Host(), true)

	return installimage.Build(machineID, installImage, m.talosRegistry)
}

// pullInstallerImage pulls the installer image into the machine's containerd and returns the resolved image name, as required by LifecycleService.{Install,Upgrade}'s InstallArtifactsSource.
func (m *Manager) pullInstallerImage(
	ctx context.Context,
	talosClient *talosclient.Client,
	imageRef, machineID string,
	cfg runConfig,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, PullTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] pulling installer image %s", imageRef)

	if err := cfg.audit(ctx, machineapi.ImageService_Pull_FullMethodName, machineID); err != nil {
		return "", err
	}

	pullClient, err := talosClient.ImageClient.Pull(ctx, &machineapi.ImageServicePullRequest{
		Containerd: m.containerdInstance,
		ImageRef:   imageRef,
	})
	if err != nil {
		return "", err
	}

	if err = pullClient.CloseSend(); err != nil {
		return "", err
	}

	var resolved string

	for {
		msg, recvErr := pullClient.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}

			return "", recvErr
		}

		if name := msg.GetName(); name != "" {
			resolved = name
		}
	}

	if resolved == "" {
		resolved = imageRef
	}

	emitf(cfg.progress, "[omni] installer image pulled: %s", resolved)

	return resolved, nil
}

// install runs LifecycleService.Install and relays installer progress.
// Note: Talos rejects with AlreadyExists when the machine reports as already installed.
func (m *Manager) install(
	ctx context.Context,
	talosClient *talosclient.Client,
	resolvedImage, disk, machineID string,
	cfg runConfig,
) error {
	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()

	if err := cfg.audit(ctx, machineapi.LifecycleService_Install_FullMethodName, machineID); err != nil {
		return err
	}

	installClient, err := talosClient.LifecycleClient.Install(ctx, &machineapi.LifecycleServiceInstallRequest{
		Containerd: m.containerdInstance,
		Source: &machineapi.InstallArtifactsSource{
			ImageName: resolvedImage,
		},
		Destination: &machineapi.InstallDestination{
			Disk: disk,
		},
	})
	if err != nil {
		return err
	}

	if err = installClient.CloseSend(); err != nil {
		return err
	}

	for {
		msg, recvErr := installClient.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return nil
			}

			return recvErr
		}

		if err = relayProgress(msg.GetProgress(), "installation", cfg.progress); err != nil {
			return err
		}
	}
}

// upgrade runs LifecycleService.Upgrade and relays installer progress.
// Note: Talos rejects with FailedPrecondition when the machine reports as not installed.
func (m *Manager) upgrade(
	ctx context.Context,
	talosClient *talosclient.Client,
	resolvedImage, machineID string,
	cfg runConfig,
) error {
	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()

	if err := cfg.audit(ctx, machineapi.LifecycleService_Upgrade_FullMethodName, machineID); err != nil {
		return err
	}

	upgradeClient, err := talosClient.LifecycleClient.Upgrade(ctx, &machineapi.LifecycleServiceUpgradeRequest{
		Containerd: m.containerdInstance,
		Source: &machineapi.InstallArtifactsSource{
			ImageName: resolvedImage,
		},
	})
	if err != nil {
		return err
	}

	if err = upgradeClient.CloseSend(); err != nil {
		return err
	}

	for {
		msg, recvErr := upgradeClient.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return nil
			}

			return recvErr
		}

		if err = relayProgress(msg.GetProgress(), "upgrade", cfg.progress); err != nil {
			return err
		}
	}
}

// reboot triggers a Talos reboot on the machine.
func (m *Manager) reboot(ctx context.Context, talosClient *talosclient.Client, machineID string, cfg runConfig) error {
	ctx, cancel := context.WithTimeout(ctx, RebootTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] rebooting machine to boot into installed Talos")

	if err := cfg.audit(ctx, machineapi.MachineService_Reboot_FullMethodName, machineID); err != nil {
		return err
	}

	return talosClient.Reboot(ctx)
}

// emitf safely sends a formatted progress message if progress is non-nil.
func emitf(progress func(string), format string, args ...any) {
	if progress == nil {
		return
	}

	progress(fmt.Sprintf(format, args...))
}

// relayProgress translates a LifecycleService progress payload into operator-visible messages.
func relayProgress(
	progress *machineapi.LifecycleServiceInstallProgress,
	operationLabel string,
	send func(string),
) error {
	switch payload := progress.GetResponse().(type) {
	case *machineapi.LifecycleServiceInstallProgress_Message:
		// Talos coalesces multiple log.Printf calls into a single message. Split on '\n' and trim trailing whitespace.
		for line := range strings.SplitSeq(progress.GetMessage(), "\n") {
			line = strings.TrimRight(line, " \t\r")
			if line == "" {
				continue
			}

			if send != nil {
				send("[talos] " + line)
			}
		}
	case *machineapi.LifecycleServiceInstallProgress_ExitCode:
		if payload.ExitCode != 0 {
			return status.Errorf(codes.Internal, "%s failed with exit code %d", operationLabel, payload.ExitCode)
		}
	}

	return nil
}
