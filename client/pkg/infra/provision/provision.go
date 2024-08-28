// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provision defines the interface for the infra provisioner.
package provision

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// Result is returned from the provision function.
type Result struct {
	// UUID is the machine UUID.
	UUID string
	// MachineID is the infra provider specific machine id.
	MachineID string
}

// Provisioner is the interface that should be implemented by an infra provider.
type Provisioner[T resource.Resource] interface {
	Provision(context.Context, *zap.Logger, T, *cloud.MachineRequest, *siderolink.ConnectionParams) (Result, error)
	Deprovision(context.Context, *zap.Logger, T, *cloud.MachineRequest) error
}
