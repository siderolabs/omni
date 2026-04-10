// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/talosupgrade"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// minKubernetesHealthCheckInterval is the minimum interval allowed for a healthcheck re-check.
const minKubernetesHealthCheckInterval = 5 * time.Second

func kubernetesHealthcheckValidationOptions() []validated.StateOption {
	validate := func(res *omni.KubernetesHealthCheck) error {
		if len(res.TypedSpec().Value.GetJob()) == 0 {
			return errors.New("healthcheck job manifest must be set")
		}

		// the ID is used to derive the runner job name, so it must produce a valid Kubernetes object name once
		// the runner prefix is prepended
		runnerName := talosupgrade.HealthCheckRunnerNamePrefix + res.Metadata().ID()
		if errs := validation.IsDNS1123Subdomain(runnerName); len(errs) > 0 {
			return fmt.Errorf("healthcheck ID %q cannot be used to build the runner name %q: %s",
				res.Metadata().ID(), runnerName, strings.Join(errs, "; "))
		}

		// the job manifest must be a valid YAML-encoded Kubernetes Job manifest, as Omni unmarshals and runs it
		if _, err := talosupgrade.ComposeHealthCheckJob(runnerName, res.TypedSpec().Value.GetJob()); err != nil {
			return fmt.Errorf("healthcheck job manifest is invalid: %w", err)
		}

		if interval := res.TypedSpec().Value.GetInterval().AsDuration(); interval > 0 && interval < minKubernetesHealthCheckInterval {
			return fmt.Errorf("healthcheck interval must be at least %s", minKubernetesHealthCheckInterval)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.KubernetesHealthCheck, _ ...state.CreateOption) error {
			return validate(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, _, newRes *omni.KubernetesHealthCheck, _ ...state.UpdateOption) error {
			return validate(newRes)
		})),
	}
}
