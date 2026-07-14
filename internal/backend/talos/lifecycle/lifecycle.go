// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package lifecycle runs Talos's LifecycleService install/upgrade for a single machine.
package lifecycle

import (
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// PullTimeout bounds pulling the installer image into containerd.
const PullTimeout = 5 * time.Minute

// InstallTimeout bounds writing the image to disk (LifecycleService install or upgrade).
const InstallTimeout = 5 * time.Minute

// PreRebootHookTimeout bounds each pre-reboot hook.
const PreRebootHookTimeout = time.Minute

// RebootTimeout is the deadline for the post-operation reboot RPC.
const RebootTimeout = time.Minute

// CordonTimeout bounds the node cordon patch issued before reboot and the uncordon patch issued after it.
const CordonTimeout = 30 * time.Second

// LivenessProbeTimeout caps the pre-flight liveness probe so a stale "connected" flag can't commit us to an unresponsive machine.
const LivenessProbeTimeout = 5 * time.Second

// RetryInterval paces a caller's retry when Run can't make progress yet for a machine (ErrAlreadyInFlight,
// or the machine is still rebooting from a prior operation).
const RetryInterval = 30 * time.Second

// Kind identifies a lifecycle operation.
type Kind int

const (
	// KindInstall writes Talos to a fresh disk on a machine with no on-disk Talos installed yet.
	KindInstall Kind = iota + 1
	// KindUpgrade replaces the existing on-disk Talos.
	KindUpgrade
)

func (k Kind) String() string {
	switch k {
	case KindInstall:
		return "install"
	case KindUpgrade:
		return "upgrade"
	default:
		return "unknown"
	}
}

// Operation describes a single install or upgrade to perform on one machine.
type Operation struct {
	// MachineStatus is the live status of the target machine; used to build the install image and to reach the machine's maintenance endpoint.
	MachineStatus *omni.MachineStatus
	// InstallImage is the target install image spec. When nil, the image is built from the machine's current schematic at Version (for callers without a target, e.g. the management RPC).
	InstallImage *specs.MachineConfigGenOptionsSpec_InstallImage
	MachineID    string
	// Version is the Talos version to install ("" = the machine's running version). Ignored when InstallImage is set.
	Version string
	// Disk is the target install disk (e.g. "/dev/sda"). Required for KindInstall; ignored for KindUpgrade.
	Disk string
	Kind Kind
}

// ErrAlreadyInFlight is returned by Run when another operation is already in progress for the machine.
var ErrAlreadyInFlight = errors.New("an operation is already in progress for this machine")

// WrapErr wraps an error with a prefix. If the error is a gRPC status, the returned error will have the same status code and the message will be prefixed.
func WrapErr(err error, prefix string) error {
	if err == nil {
		return nil
	}

	if prefix != "" {
		prefix += ": "
	}

	if s, ok := status.FromError(err); ok {
		return status.Errorf(s.Code(), "%s%s", prefix, s.Message())
	}

	return status.Errorf(codes.Internal, "%s%v", prefix, err)
}
