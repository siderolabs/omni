// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	omnijsonschema "github.com/siderolabs/omni/internal/pkg/jsonschema"
)

// machineClassValidationOptions returns the validation options for the machine class resource.
//
//nolint:gocognit
func machineClassValidationOptions(st state.State) []validated.StateOption {
	validate := func(ctx context.Context, oldRes, res *omni.MachineClass) error {
		if res.TypedSpec().Value.AutoProvision != nil && res.TypedSpec().Value.MatchLabels != nil {
			return errors.New("can't set both auto provision and match labels at the same time")
		}

		if res.TypedSpec().Value.AutoProvision != nil {
			autoProvision := res.TypedSpec().Value.AutoProvision

			if autoProvision.ProviderId == "" {
				return errors.New("providerID can not be empty")
			}

			if oldRes == nil || oldRes.TypedSpec().Value.AutoProvision.ProviderData != autoProvision.ProviderData {
				if err := validateProviderData(ctx, st, autoProvision.ProviderId, autoProvision.ProviderData); err != nil {
					return err
				}
			}

			if err := validateUserStringSlice("auto provision kernel args", autoProvision.GetKernelArgs(), MaxKernelArgsCount, MaxKernelArgLength); err != nil {
				return err
			}

			return nil
		}

		queries, err := labels.ParseSelectors(res.TypedSpec().Value.MatchLabels)
		if err != nil {
			return fmt.Errorf("failed to parse matchLabels: %w", err)
		}

		if len(queries) == 0 {
			return fmt.Errorf("machine class should either have auto provision or match labels set")
		}

		if slices.IndexFunc(queries, func(s resource.LabelQuery) bool {
			return slices.IndexFunc(s.Terms, func(term resource.LabelTerm) bool {
				return term.Key == omni.LabelNoManualAllocation
			}) != -1
		}) != -1 {
			return fmt.Errorf("selectors using label %s are not allowed", omni.LabelNoManualAllocation)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineClass, _ ...state.CreateOption) error {
			return validate(ctx, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineClass, res *omni.MachineClass, _ ...state.UpdateOption) error {
			return validate(ctx, oldRes, res)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.MachineClass, _ ...state.DestroyOption) error {
			machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, st)
			if err != nil {
				return err
			}

			var inUseBy []string

			machineSets.ForEach(func(r *omni.MachineSet) {
				if alloc := omni.GetMachineAllocation(r); alloc != nil && res.Metadata().ID() == alloc.Name {
					inUseBy = append(inUseBy, r.Metadata().ID())
				}
			})

			if len(inUseBy) > 0 {
				return fmt.Errorf("can not delete the machine class as it is still in use by machine sets: %s", strings.Join(inUseBy, ", "))
			}

			return nil
		})),
	}
}

func validateProviderData(ctx context.Context, st state.State, providerID, providerData string) error {
	validateSchema := func(providerStatus *infra.ProviderStatus) error {
		if providerStatus.TypedSpec().Value.Schema == "" {
			return nil
		}

		schema, err := omnijsonschema.Parse(providerStatus.Metadata().ID(), providerStatus.TypedSpec().Value.Schema)
		if err != nil {
			return fmt.Errorf("failed to load json schema for provider %q: %w", providerID, err)
		}

		return schema.Validate(providerData)
	}

	providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, st, providerID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	if _, static := providerStatus.Metadata().Labels().Get(omni.LabelIsStaticInfraProvider); static {
		return fmt.Errorf("cannot use static provider in the auto-provisioned machine class")
	}

	return validateSchema(providerStatus)
}
