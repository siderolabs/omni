// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/subscriptionitem"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// DefaultDebounceDuration defines the default time to wait after the last event before processing updates.
var DefaultDebounceDuration = 10 * time.Minute

// StripeMetricsReporterControllerName is the name of the StripeMetricsReporterController.
const StripeMetricsReporterControllerName = "StripeMetricsReporterController"

// StripeMetricsReporterController reports machine metrics to Stripe.
type StripeMetricsReporterController struct {
	generic.NamedController

	stripeAPIKey             string
	stripeSubscriptionItemID string
	stripeMinCommit          uint32
	debounceDuration         time.Duration
}

// StripeMetricsReporterControllerOptions defines the options for StripeMetricsReporterController.
type StripeMetricsReporterControllerOptions struct {
	DebounceDuration time.Duration
}

// StripeMetricsReporterControllerOption defines the option type for StripeMetricsReporterController.
type StripeMetricsReporterControllerOption func(*StripeMetricsReporterControllerOptions)

// WithDebounceDuration sets the debounce duration for StripeMetricsReporterController.
func WithDebounceDuration(duration time.Duration) StripeMetricsReporterControllerOption {
	return func(opts *StripeMetricsReporterControllerOptions) {
		opts.DebounceDuration = duration
	}
}

// NewStripeMetricsReporterController initializes StripeMetricsReporterController.
func NewStripeMetricsReporterController(stripeAPIKey, stripeSubscriptionItemID string, stripeMinCommit uint32, opts ...StripeMetricsReporterControllerOption) *StripeMetricsReporterController {
	options := &StripeMetricsReporterControllerOptions{
		DebounceDuration: DefaultDebounceDuration,
	}

	for _, opt := range opts {
		opt(options)
	}

	return &StripeMetricsReporterController{
		NamedController: generic.NamedController{
			ControllerName: StripeMetricsReporterControllerName,
		},
		stripeAPIKey:             stripeAPIKey,
		stripeSubscriptionItemID: stripeSubscriptionItemID,
		stripeMinCommit:          stripeMinCommit,
		debounceDuration:         options.DebounceDuration,
	}
}

// Inputs implements controller.QController interface.
func (ctrl *StripeMetricsReporterController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.EphemeralNamespace,
			Type:      omni.MachineStatusMetricsType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StripeMetricsReporterController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *StripeMetricsReporterController) Run(ctx context.Context, r controller.Runtime, log *zap.Logger) error {
	stripe.Key = ctrl.stripeAPIKey

	var (
		pendingCount optional.Optional[uint32]
		timerCh      <-chan time.Time
	)

	processPending := func() {
		if !pendingCount.IsPresent() {
			return
		}

		count := pendingCount.ValueOr(0)
		pendingCount = optional.Optional[uint32]{} // Reset to empty

		if count < ctrl.stripeMinCommit {
			log.Info("Committed machine count below minimum commit, sending minimum to stripe instead ", zap.Uint32("count", count), zap.Uint32("minimum_commit", ctrl.stripeMinCommit))
			count = ctrl.stripeMinCommit
		}

		err := updateStripeSubscriptionItemQuantity(ctx, ctrl.stripeSubscriptionItemID, count, log)
		if err != nil {
			log.Error("Failed to update subscription item", zap.String("subscription_item_id", ctrl.stripeSubscriptionItemID), zap.Uint32("count", count), zap.Error(err))
		}
	}

	for {
		select {
		case <-ctx.Done():
			processPending()

			return nil
		case <-r.EventCh():
		case <-timerCh:
			processPending()

			timerCh = nil
		}

		metrics, err := safe.ReaderGetByID[*omni.MachineStatusMetrics](ctx, r, omni.MachineStatusMetricsID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		pendingCount = optional.Some(metrics.TypedSpec().Value.RegisteredMachinesCount)

		timerCh = time.After(ctrl.debounceDuration)

		r.ResetRestartBackoff()
	}
}

// updateStripeSubscriptionItemQuantity adjusts the quantity of a subscription.
// Implements retry logic with exponential backoff for transient failures.
func updateStripeSubscriptionItemQuantity(ctx context.Context, subscriptionItemID string, count uint32, log *zap.Logger) error {
	operation := func() (bool, error) {
		newQuantity := count

		log.Info("Updating subscription item quantity", zap.String("subscription_item_id", subscriptionItemID), zap.Uint32("new_quantity", newQuantity))

		updateParams := &stripe.SubscriptionItemParams{
			Quantity: stripe.Int64(int64(newQuantity)),
		}

		_, err := subscriptionitem.Update(subscriptionItemID, updateParams)
		if err != nil {
			return false, fmt.Errorf("failed to update subscription item %s quantity: %w", subscriptionItemID, err)
		}

		return true, nil
	}

	backoffConfig := backoff.NewExponentialBackOff()

	_, err := backoff.Retry(ctx, operation, backoff.WithMaxElapsedTime(2*time.Minute), backoff.WithBackOff(backoffConfig))
	if err != nil {
		return fmt.Errorf("failed to update subscription item after retries: %w", err)
	}

	return nil
}
