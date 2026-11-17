// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// WithRedaction asks Talos to redact sensitive resources, and deny those that can't be redacted.
//
// Enforce, because accept serves os:admin in full, and the maintenance API grants Omni all the roles.
func WithRedaction(ctx context.Context) context.Context {
	return redactable.WithMode(ctx, redactable.ModeEnforce)
}

// CheckSensitivity denies reads of sensitive resource types, unless machineClient is set and its node supports
// redaction. ctx should come from WithRedaction.
func CheckSensitivity(ctx context.Context, st state.State, machineClient machine.MachineServiceClient, resourceType string) error {
	rd, err := safe.StateGet[*meta.ResourceDefinition](
		ctx, st,
		resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, strings.ToLower(resourceType), resource.VersionUndefined),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return status.Errorf(codes.PermissionDenied, "resource type %q is not supported", resourceType)
		}

		return err
	}

	if rd.TypedSpec().Sensitivity != meta.Sensitive {
		return nil
	}

	if machineClient == nil {
		return status.Errorf(codes.PermissionDenied, "access to the sensitive resource type %q is not permitted", resourceType)
	}

	supported, err := supportsRedaction(ctx, machineClient)
	if err != nil {
		return err
	}

	if !supported {
		return status.Errorf(codes.PermissionDenied, "access to the sensitive resource type %q is not permitted", resourceType)
	}

	return nil
}

// supportsRedaction asks the node for its version, which has to report the ResourceRedaction feature.
func supportsRedaction(ctx context.Context, machineClient machine.MachineServiceClient) (bool, error) {
	resp, err := machineClient.Version(ctx, &emptypb.Empty{})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return false, nil
		}

		return false, err
	}

	messages := resp.GetMessages()

	// the feature is unset by Talos < 1.15, and in the answer of a node which failed
	return len(messages) > 0 && !slices.ContainsFunc(messages, func(message *machine.Version) bool {
		return !message.GetFeatures().GetResourceRedaction()
	}), nil
}
