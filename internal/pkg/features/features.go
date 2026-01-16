// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package features contains the feature flags related functionality.
package features

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// UpdateResources creates or updates the features omni.FeaturesConfig resource with the current feature flags.
func UpdateResources(ctx context.Context, st state.State, logger *zap.Logger) error {
	workloadProxyEnabled := config.Config.Services.WorkloadProxy.GetEnabled()

	updateFeaturesConfig := func(res *omni.FeaturesConfig) error {
		res.TypedSpec().Value.EnableWorkloadProxying = workloadProxyEnabled
		res.TypedSpec().Value.EmbeddedDiscoveryService = config.Config.Services.EmbeddedDiscoveryService.GetEnabled()
		res.TypedSpec().Value.EtcdBackupSettings = &specs.EtcdBackupSettings{
			TickInterval: durationpb.New(config.Config.EtcdBackup.GetTickInterval()),
			MinInterval:  durationpb.New(config.Config.EtcdBackup.GetMinInterval()),
			MaxInterval:  durationpb.New(config.Config.EtcdBackup.GetMaxInterval()),
		}

		res.TypedSpec().Value.AuditLogEnabled = config.Config.Logs.Audit.GetPath() != "" //nolint:staticcheck
		res.TypedSpec().Value.ImageFactoryBaseUrl = config.Config.Registries.GetImageFactoryBaseURL()

		imageFactoryPXEBaseURL, err := config.Config.GetImageFactoryPXEBaseURL()
		if err != nil {
			return err
		}

		res.TypedSpec().Value.ImageFactoryPxeBaseUrl = imageFactoryPXEBaseURL.String()
		res.TypedSpec().Value.UserPilotSettings = &specs.UserPilotSettings{
			AppToken: config.Config.Account.UserPilot.GetAppToken(),
		}
		res.TypedSpec().Value.StripeSettings = &specs.StripeSettings{
			Enabled:   config.Config.Logs.Stripe.GetEnabled(),
			MinCommit: config.Config.Logs.Stripe.GetMinCommit(),
		}
		res.TypedSpec().Value.Account = &specs.Account{
			Id:   config.Config.Account.GetId(),
			Name: config.Config.Account.GetName(),
		}
		res.TypedSpec().Value.TalosPreReleaseVersionsEnabled = config.Config.Features.GetEnableTalosPreReleaseVersions()

		return nil
	}

	featuresConfig := omni.NewFeaturesConfig(omni.FeaturesConfigID)

	_, err := st.Get(ctx, featuresConfig.Metadata())
	if err != nil {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get features config: %w", err)
		}

		if err = updateFeaturesConfig(featuresConfig); err != nil {
			return fmt.Errorf("failed to update features config: %w", err)
		}

		err = st.Create(ctx, featuresConfig)
		if err != nil {
			return fmt.Errorf("failed to create features config: %w", err)
		}

		logger.Info("created features config resource", zap.Bool("enable_workload_proxying", workloadProxyEnabled))

		return nil
	}

	if _, err = safe.StateUpdateWithConflicts(ctx, st, featuresConfig.Metadata(), updateFeaturesConfig); err != nil {
		return fmt.Errorf("failed to update features config: %w", err)
	}

	logger.Info("updated features config resource", zap.Bool("enable_workload_proxying", workloadProxyEnabled))

	return nil
}
