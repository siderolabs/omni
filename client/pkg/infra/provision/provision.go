// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provision defines the interface for the infra provisioner.
package provision

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
)

// NewStep creates a new provision step.
func NewStep[T resource.Resource](name string, run func(context.Context, *zap.Logger, Context[T]) error) Step[T] {
	return Step[T]{name: name, run: run}
}

// Step implements a single provision step.
type Step[T resource.Resource] struct {
	run  func(context.Context, *zap.Logger, Context[T]) error
	name string
}

// Name of the step.
func (s Step[T]) Name() string {
	return s.name
}

// Run the step.
func (s Step[T]) Run(ctx context.Context, logger *zap.Logger, provisionContext Context[T]) error {
	return s.run(ctx, logger, provisionContext)
}

// Provisioner is the interface that should be implemented by an infra provider.
type Provisioner[T resource.Resource] interface {
	ProvisionSteps() []Step[T]
	Deprovision(context.Context, *zap.Logger, T, *infra.MachineRequest) error
}
