// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// redactionVersion is the first Talos version that supports redaction.
var redactionVersion = semver.MustParse("1.15.0-beta.0")

// SupportsRedaction reports whether the given Talos version supports redaction. Unknown versions don't.
func SupportsRedaction(talosVersion string) bool {
	version, err := semver.ParseTolerant(talosVersion)
	if err != nil {
		return false
	}

	return version.GTE(redactionVersion)
}

// WithRedaction asks Talos to redact sensitive resources, and deny those that can't be redacted.
//
// Enforce, because accept serves os:admin in full, and the maintenance API grants Omni all the roles.
func WithRedaction(ctx context.Context) context.Context {
	return redactable.WithMode(ctx, redactable.ModeEnforce)
}

// CheckSensitivity checks access to the sensitive resources of a Talos node.
//
// Sensitive types are allowed only with redaction: the node supports it, and ctx comes from WithRedaction.
// Talos then redacts or denies them per type.
//
// Support is judged from the Talos version Omni last recorded, which can be briefly stale after a downgrade.
func CheckSensitivity(ctx context.Context, st state.State, resourceType string, redaction bool) error {
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

	if rd.TypedSpec().Sensitivity == meta.Sensitive && !redaction {
		return status.Errorf(codes.PermissionDenied, "access to the sensitive resource type %q is not permitted", resourceType)
	}

	return nil
}
