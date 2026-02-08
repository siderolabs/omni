// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package features contains the feature flags related functionality.
package features

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Params holds the values needed to update the features config resource.
type Params struct {
	ImageFactoryPXEBaseURL          *url.URL
	ImageFactoryBaseURL             string
	AccountName                     string
	AccountID                       string
	UserPilotAppToken               string
	AuditLogPath                    string
	EtcdBackupMinInterval           time.Duration
	EtcdBackupMaxInterval           time.Duration
	EtcdBackupTickInterval          time.Duration
	StripeMinCommit                 uint32
	WorkloadProxyEnabled            bool
	StripeEnabled                   bool
	EmbeddedDiscoveryServiceEnabled bool
	TalosPreReleaseVersionsEnabled  bool
}

// UpdateResources creates or updates the features omni.FeaturesConfig resource with the current feature flags.
func UpdateResources(ctx context.Context, st state.State, logger *zap.Logger, params Params) error {
	workloadProxyEnabled := params.WorkloadProxyEnabled

	updateFeaturesConfig := func(res *omni.FeaturesConfig) error {
		res.TypedSpec().Value.EnableWorkloadProxying = workloadProxyEnabled
		res.TypedSpec().Value.EmbeddedDiscoveryService = params.EmbeddedDiscoveryServiceEnabled
		res.TypedSpec().Value.EtcdBackupSettings = &specs.EtcdBackupSettings{
			TickInterval: durationpb.New(params.EtcdBackupTickInterval),
			MinInterval:  durationpb.New(params.EtcdBackupMinInterval),
			MaxInterval:  durationpb.New(params.EtcdBackupMaxInterval),
		}

		res.TypedSpec().Value.AuditLogEnabled = params.AuditLogPath != ""
		res.TypedSpec().Value.ImageFactoryBaseUrl = params.ImageFactoryBaseURL
		res.TypedSpec().Value.ImageFactoryPxeBaseUrl = params.ImageFactoryPXEBaseURL.String()

		res.TypedSpec().Value.UserPilotSettings = &specs.UserPilotSettings{
			AppToken: params.UserPilotAppToken,
		}
		res.TypedSpec().Value.StripeSettings = &specs.StripeSettings{
			Enabled:   params.StripeEnabled,
			MinCommit: params.StripeMinCommit,
		}
		res.TypedSpec().Value.Account = &specs.Account{
			Id:   params.AccountID,
			Name: params.AccountName,
		}
		res.TypedSpec().Value.TalosPreReleaseVersionsEnabled = params.TalosPreReleaseVersionsEnabled

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
