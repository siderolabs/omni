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

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// clusterValidationOptions returns the validation options for the Talos and Kubernetes versions on the cluster resource.
// Validation is only syntactic - they are checked whether they are valid semver strings.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func clusterValidationOptions(st state.State, etcdBackupConfig config.EtcdBackup, embeddedDiscoveryServiceConfig config.EmbeddedDiscoveryService) []validated.StateOption {
	validateVersions := func(ctx context.Context, existingRes *omni.Cluster, res *omni.Cluster, skipTalosVersion, skipKubernetesVersion bool) error {
		if skipTalosVersion && skipKubernetesVersion {
			return nil
		}

		talosVersion, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(res.TypedSpec().Value.TalosVersion).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) && skipTalosVersion {
				return nil
			}

			return fmt.Errorf("invalid talos version %q: %w", res.TypedSpec().Value.TalosVersion, err)
		}

		var currentTalosVersion string

		if existingRes != nil {
			currentTalosVersion = existingRes.TypedSpec().Value.TalosVersion
		}

		if err = validateTalosVersion(ctx, st, currentTalosVersion, res.TypedSpec().Value.TalosVersion); err != nil {
			return err
		}

		clusterConfigVersion, err := safe.StateGetByID[*omni.ClusterConfigVersion](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get cluster config version: %w", err)
		}

		if existingRes != nil && clusterConfigVersion != nil {
			initialTalosVersion, initialVersionErr := semver.ParseTolerant(clusterConfigVersion.TypedSpec().Value.Version)
			if initialVersionErr != nil {
				return fmt.Errorf("invalid initial talos version %q: %w", clusterConfigVersion.TypedSpec().Value.Version, initialVersionErr)
			}

			newTalosVersion, newVersionErr := semver.ParseTolerant(res.TypedSpec().Value.TalosVersion)
			if newVersionErr != nil {
				return fmt.Errorf("invalid current talos version %q: %w", res.TypedSpec().Value.TalosVersion, newVersionErr)
			}

			if newTalosVersion.Major < initialTalosVersion.Major || (newTalosVersion.Major == initialTalosVersion.Major && newTalosVersion.Minor < initialTalosVersion.Minor) {
				return fmt.Errorf("downgrading from version %q to %q is not supported", initialTalosVersion.String(), res.TypedSpec().Value.TalosVersion)
			}
		}

		if skipKubernetesVersion {
			return nil
		}

		var currentKubernetesVersion string

		if existingRes != nil {
			currentKubernetesVersion = existingRes.TypedSpec().Value.KubernetesVersion
		}

		upgradeStatus, err := safe.ReaderGetByID[*omni.KubernetesUpgradeStatus](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if err = validateKubernetesVersion(currentKubernetesVersion, res.TypedSpec().Value.KubernetesVersion, upgradeStatus); err != nil {
			return err
		}

		if slices.Contains(talosVersion.TypedSpec().Value.CompatibleKubernetesVersions, res.TypedSpec().Value.KubernetesVersion) {
			return nil
		}

		return fmt.Errorf("invalid kubernetes version %q: is not compatible with talos version %q", res.TypedSpec().Value.KubernetesVersion, res.TypedSpec().Value.TalosVersion)
	}

	validateBackupInterval := func(res *omni.Cluster) error {
		if conf := res.TypedSpec().Value.GetBackupConfiguration(); conf != nil {
			switch conf := conf.GetInterval().AsDuration(); {
			case conf < etcdBackupConfig.GetMinInterval():
				return fmt.Errorf(
					"backup interval must be greater than %s, actual %s",
					etcdBackupConfig.MinInterval.String(),
					conf.String(),
				)
			case conf > etcdBackupConfig.GetMaxInterval():
				return fmt.Errorf(
					"backup interval must be less than %s, actual %s",
					etcdBackupConfig.MaxInterval.String(),
					conf.String(),
				)
			}
		}

		return nil
	}

	validateEmbeddedDiscoveryServiceSetting := func(oldRes, newRes *omni.Cluster) error {
		newValue := newRes.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService()
		if !newValue { // feature being disabled is always valid
			return nil
		}

		// if this is a create operation or if the setting is changed, validate that the feature is available
		if oldRes == nil || oldRes.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService() != newValue {
			if !embeddedDiscoveryServiceConfig.GetEnabled() {
				return errors.New("embedded discovery service is not enabled")
			}
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.Cluster, _ ...state.CreateOption) error {
			var multiErr error

			validator := omni.ClusterValidator{
				ID:                res.Metadata().ID(),
				KubernetesVersion: res.TypedSpec().Value.KubernetesVersion,
				TalosVersion:      res.TypedSpec().Value.TalosVersion,
				EncryptionEnabled: omni.GetEncryptionEnabled(res),
			}

			if err := validator.Validate(); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateBackupInterval(res); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateEmbeddedDiscoveryServiceSetting(nil, res); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateVersions(ctx, nil, res, false, false); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			return multiErr
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, existingRes *omni.Cluster, newRes *omni.Cluster, _ ...state.UpdateOption) error {
			if existingRes == nil {
				// shouldn't happen - skip the validation, so that the original error (NotFound) will be returned
				return nil
			}

			if newRes.Metadata().Phase() == resource.PhaseTearingDown {
				// tearing down validations are done at destroy validations
				return nil
			}

			_, wasLocked := existingRes.Metadata().Annotations().Get(omni.ClusterLocked)
			_, stillLocked := newRes.Metadata().Annotations().Get(omni.ClusterLocked)
			_, wasImporting := existingRes.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)

			_, stillImporting := newRes.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)
			if wasLocked && stillLocked && !wasImporting && !stillImporting {
				return fmt.Errorf("updating cluster configuration is not allowed: the cluster %q is locked", newRes.Metadata().ID())
			}

			var multiErr error

			skipTalosVersion := existingRes.TypedSpec().Value.TalosVersion == newRes.TypedSpec().Value.TalosVersion
			skipKubernetesVersion := skipTalosVersion && existingRes.TypedSpec().Value.KubernetesVersion == newRes.TypedSpec().Value.KubernetesVersion
			encryptionEnabled := omni.GetEncryptionEnabled(newRes)

			validator := omni.ClusterValidator{
				ID:                         newRes.Metadata().ID(),
				SkipClusterIDCheck:         true,
				KubernetesVersion:          newRes.TypedSpec().Value.KubernetesVersion,
				TalosVersion:               newRes.TypedSpec().Value.TalosVersion,
				EncryptionEnabled:          encryptionEnabled,
				SkipTalosVersionCheck:      skipTalosVersion,
				SkipKubernetesVersionCheck: skipKubernetesVersion,
			}

			if err := validator.Validate(); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if omni.GetEncryptionEnabled(existingRes) != encryptionEnabled {
				multiErr = multierror.Append(multiErr, errors.New("updating disk encryption settings is not allowed"))
			}

			if err := validateBackupInterval(newRes); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateEmbeddedDiscoveryServiceSetting(existingRes, newRes); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			if err := validateVersions(ctx, existingRes, newRes, skipTalosVersion, skipKubernetesVersion); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			return multiErr
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.Cluster, option ...state.DestroyOption) error {
			if res == nil {
				return nil
			}

			clusterStatus, err := safe.StateGetByID[*omni.ClusterStatus](ctx, st, res.Metadata().ID())
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}

			if clusterStatus == nil {
				return nil
			}

			_, locked := res.Metadata().Annotations().Get(omni.ClusterLocked)
			_, importing := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)

			if locked && !importing {
				return fmt.Errorf("deletion is not allowed: the cluster %q is locked", res.Metadata().ID())
			}

			return nil
		})),
	}
}
