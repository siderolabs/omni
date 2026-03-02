// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

const (
	identityTypeUser           = "user"
	identityTypeServiceAccount = "service_account"

	window7d  = "7d"
	window30d = "30d"

	reconcileInterval = 5 * time.Minute
)

// UserMetricsController provides metrics on users and service accounts.
//
//nolint:govet
type UserMetricsController struct {
	metricsOnce       sync.Once
	metricActiveUsers *prometheus.GaugeVec
	metricUsersTotal  *prometheus.GaugeVec
}

// Name implements controller.Controller interface.
func (ctrl *UserMetricsController) Name() string {
	return "UserMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UserMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.MetricsNamespace,
			Type:      authres.IdentityLastActiveType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      authres.IdentityType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UserMetricsController) Outputs() []controller.Output {
	return nil
}

func (ctrl *UserMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.metricActiveUsers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_active_users",
			Help: "Number of active users and service accounts by time window (7d, 30d).",
		}, []string{"type", "window"})

		ctrl.metricUsersTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_users",
			Help: "Total number of registered users and service accounts.",
		}, []string{"type"})
	})
}

// Run implements controller.Controller interface.
func (ctrl *UserMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		if err := ctrl.updateMetrics(ctx, r); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reconcileInterval):
		}
	}
}

func (ctrl *UserMetricsController) updateMetrics(ctx context.Context, r controller.Runtime) error {
	identities, err := safe.ReaderListAll[*authres.Identity](ctx, r)
	if err != nil {
		return err
	}

	serviceAccountIDs := map[string]struct{}{}
	totalUsers := 0
	totalServiceAccounts := 0

	for identity := range identities.All() {
		if _, ok := identity.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount); ok {
			serviceAccountIDs[identity.Metadata().ID()] = struct{}{}
			totalServiceAccounts++
		} else {
			totalUsers++
		}
	}

	ctrl.metricUsersTotal.WithLabelValues(identityTypeUser).Set(float64(totalUsers))
	ctrl.metricUsersTotal.WithLabelValues(identityTypeServiceAccount).Set(float64(totalServiceAccounts))

	lastActiveList, err := safe.ReaderListAll[*authres.IdentityLastActive](ctx, r)
	if err != nil {
		return err
	}

	now := time.Now()
	threshold7d := now.Add(-7 * 24 * time.Hour)
	threshold30d := now.Add(-30 * 24 * time.Hour)

	activeCounts := map[string]map[string]float64{
		window7d:  {identityTypeUser: 0, identityTypeServiceAccount: 0},
		window30d: {identityTypeUser: 0, identityTypeServiceAccount: 0},
	}

	for res := range lastActiveList.All() {
		lastActive := res.TypedSpec().Value.GetLastActive().AsTime()

		identityType := identityTypeUser
		if _, ok := serviceAccountIDs[res.Metadata().ID()]; ok {
			identityType = identityTypeServiceAccount
		}

		if lastActive.After(threshold30d) {
			activeCounts[window30d][identityType]++
		}

		if lastActive.After(threshold7d) {
			activeCounts[window7d][identityType]++
		}
	}

	for window, types := range activeCounts {
		for identityType, count := range types {
			ctrl.metricActiveUsers.WithLabelValues(identityType, window).Set(count)
		}
	}

	return nil
}

// Describe implements prometheus.Collector interface.
func (ctrl *UserMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prometheus.Collector interface.
func (ctrl *UserMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.metricActiveUsers.Collect(ch)
	ctrl.metricUsersTotal.Collect(ch)
}

var _ prometheus.Collector = &UserMetricsController{}
