// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CheckSensitivity checks access to the sensitive resources of a Talos node.
func CheckSensitivity(ctx context.Context, st state.State, resourceType string) error {
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

	if rd.TypedSpec().Sensitivity == meta.Sensitive {
		return status.Errorf(codes.PermissionDenied, "access to the sensitive resource type %q is not permitted", resourceType)
	}

	return nil
}
