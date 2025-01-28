// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// KeyPrunerController is a controller which periodically prunes expired public keys.
type KeyPrunerController struct {
	clock clock.Clock

	interval time.Duration
}

// Option is a functional option for KeyPrunerController.
type Option func(*KeyPrunerController)

// WithClock sets the clock to use for the controller.
func WithClock(clock clock.Clock) Option {
	return func(k *KeyPrunerController) {
		k.clock = clock
	}
}

// NewKeyPrunerController initializes a new KeyPrunerController.
func NewKeyPrunerController(interval time.Duration, opts ...Option) *KeyPrunerController {
	result := &KeyPrunerController{interval: interval}

	for _, opt := range opts {
		opt(result)
	}

	if result.clock == nil {
		result.clock = clock.New()
	}

	return result
}

// Name implements controller.Controller interface.
func (*KeyPrunerController) Name() string {
	return "KeyPrunerController"
}

// Inputs implements controller.Controller interface.
func (k *KeyPrunerController) Inputs() []controller.Input {
	// no inputs
	return nil
}

// Outputs implements controller.Controller interface.
func (k *KeyPrunerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: auth.PublicKeyType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (k *KeyPrunerController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	ticker := k.clock.Ticker(k.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
			logger.Info("running key pruner")

			err := k.run(ctx, runtime, logger)
			if err != nil {
				return fmt.Errorf("error running key pruner: %w", err)
			}
		case <-ticker.C:
			err := k.run(ctx, runtime, logger)
			if err != nil {
				return fmt.Errorf("error running key pruner: %w", err)
			}
		}
	}
}

func (k *KeyPrunerController) run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	list, err := safe.ReaderListAll[*auth.PublicKey](ctx, runtime)
	if err != nil {
		return err
	}

	for v := range list.All() {
		md := v.Metadata()
		publicKeySpec := v.TypedSpec().Value

		if k.clock.Now().Before(publicKeySpec.Expiration.AsTime()) {
			continue
		}

		logger.Info("removing expired public key", zap.String("id", md.ID()), zap.Time("expiration", publicKeySpec.Expiration.AsTime()))

		err := runtime.Destroy(ctx, md)
		if state.IsOwnerConflictError(err) {
			// probably empty owner, trying to remove it again
			err = runtime.Destroy(ctx, md)
			if err != nil {
				logger.Error("error destroying key with empty owner", zap.String("id", md.ID()), zap.Error(err))
			}
		} else if err != nil {
			logger.Error("error destroying key", zap.String("id", md.ID()), zap.Error(err))
		}
	}

	return nil
}
