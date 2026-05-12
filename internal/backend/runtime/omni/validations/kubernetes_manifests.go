// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func kubernetesManifestsValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.KubernetesManifestGroup, _ ...state.CreateOption) error {
			return validateKubernetesManifests(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, _, res *omni.KubernetesManifestGroup, _ ...state.UpdateOption) error {
			return validateKubernetesManifests(res)
		})),
	}
}

func validateKubernetesManifests(res *omni.KubernetesManifestGroup) error {
	if res.TypedSpec().Value.Mode == specs.KubernetesManifestGroupSpec_UNKNOWN {
		return fmt.Errorf("the manifest should have mode field set")
	}

	if _, ok := res.Metadata().Labels().Get(omni.LabelCluster); !ok {
		return fmt.Errorf("the resource must have %s label set", omni.LabelCluster)
	}

	if _, ok := res.Metadata().Labels().Get(omni.LabelSystemManifest); ok {
		return fmt.Errorf("system manifests can't be created by the user")
	}

	_, err := res.TypedSpec().Value.GetManifests()
	if err != nil {
		return err
	}

	return nil
}
